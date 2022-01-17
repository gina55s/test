package provisiond

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/cenkalti/backoff/v3"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/rusart/muxprom"
	"github.com/threefoldtech/substrate-client"
	"github.com/threefoldtech/test/pkg"
	"github.com/threefoldtech/test/pkg/app"
	"github.com/threefoldtech/test/pkg/capacity"
	"github.com/threefoldtech/test/pkg/environment"
	"github.com/threefoldtech/test/pkg/gridtypes"
	"github.com/threefoldtech/test/pkg/gridtypes/test"
	"github.com/threefoldtech/test/pkg/primitives"
	"github.com/threefoldtech/test/pkg/provision/mbus"
	"github.com/threefoldtech/test/pkg/provision/storage"
	fsStorage "github.com/threefoldtech/test/pkg/provision/storage.fs"
	"github.com/threefoldtech/test/pkg/rmb"
	"github.com/urfave/cli/v2"

	"github.com/threefoldtech/test/pkg/stubs"
	"github.com/threefoldtech/test/pkg/utils"

	"github.com/rs/zerolog/log"

	"github.com/threefoldtech/zbus"
	"github.com/threefoldtech/test/pkg/provision"
)

const (
	serverName       = "provision"
	provisionModule  = "provision"
	statisticsModule = "statistics"
	gib              = 1024 * 1024 * 1024

	boltStorageDB = "workloads.bolt"

	// deprecated, kept for migration
	fsStorageDB = "workloads"
)

// Module entry point
var Module cli.Command = cli.Command{
	Name:  "provisiond",
	Usage: "handles reservations streams and use other daemon to deploy them",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "root",
			Usage: "`ROOT` working directory of the module",
			Value: "/var/cache/modules/provisiond",
		},
		&cli.StringFlag{
			Name:  "broker",
			Usage: "connection string to the message `BROKER`",
			Value: "unix:///var/run/redis.sock",
		},
		&cli.StringFlag{
			Name:  "http",
			Usage: "http listen address",
			Value: ":2021",
		},
	},
	Action: action,
}

func action(cli *cli.Context) error {
	var (
		msgBrokerCon string = cli.String("broker")
		rootDir      string = cli.String("root")
	)

	ctx := context.Background()
	ctx, _ = utils.WithSignal(ctx)

	// keep checking if limited-cache flag is set
	if app.CheckFlag(app.LimitedCache) {
		log.Error().Msg("failed cache reservation! Retrying every 30 seconds...")
		for app.CheckFlag(app.LimitedCache) {
			time.Sleep(time.Second * 30)
		}
	}

	if err := os.MkdirAll(rootDir, 0770); err != nil {
		return errors.Wrap(err, "failed to create cache directory")
	}

	env, err := environment.Get()
	if err != nil {
		return errors.Wrap(err, "failed to parse node environment")
	}

	if env.Orphan {
		// disable providiond on this node
		// we don't have a valid farmer id set
		log.Info().Msg("orphan node, we won't provision anything at all")
		select {}
	}

	server, err := zbus.NewRedisServer(serverName, msgBrokerCon, 1)
	if err != nil {
		return errors.Wrap(err, "failed to connect to message broker")
	}
	cl, err := zbus.NewRedisClient(msgBrokerCon)
	if err != nil {
		return errors.Wrap(err, "fail to connect to message broker server")
	}

	identity := stubs.NewIdentityManagerStub(cl)
	sk := ed25519.PrivateKey(identity.PrivateKey(ctx))

	// block until networkd is ready to serve request from zbus
	// this is used to prevent uptime and online status to the explorer if the node is not in a fully ready
	// https://github.com/threefoldtech/test/issues/632
	// NOTE - UPDATE: this block of code should be deprecated
	// since we do the waiting in zinit now since provisiond waits for networkd
	// which has a 'test' condition in the zinit yaml file for networkd to wait
	// for zbus
	network := stubs.NewNetworkerStub(cl)
	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = 0
	backoff.RetryNotify(func() error {
		return network.Ready(cli.Context)
	}, bo, func(err error, d time.Duration) {
		log.Error().Err(err).Msg("networkd is not ready yet")
	})

	router := mux.NewRouter().StrictSlash(true)

	prom := muxprom.New(
		muxprom.Router(router),
		muxprom.Namespace("provision"),
	)
	prom.Instrument()

	mBus, err := rmb.New(msgBrokerCon)
	if err != nil {
		return errors.Wrap(err, "Failed to initialize message bus")
	}

	testRouter := mBus.Subroute("test")
	testRouter.Use(rmb.LoggerMiddleware)

	// the v1 endpoint will be used by all components to register endpoints
	// that are specific for that component
	//v1 := router.PathPrefix("/api/v1").Subrouter()
	// keep track of resource units reserved and amount of workloads provisionned

	// to store reservation locally on the node
	store, err := storage.New(filepath.Join(rootDir, boltStorageDB))
	if err != nil {
		return errors.Wrap(err, "failed to create local reservation store")
	}
	defer store.Close()
	// we check if the old fs storage still exists
	fsStoragePath := filepath.Join(rootDir, fsStorageDB)
	if _, err := os.Stat(fsStoragePath); err == nil {
		// if it does we need to migrate this storage to new bolt storage
		fs, err := fsStorage.NewFSStore(fsStoragePath)
		if err != nil {
			return err
		}

		if err := storageMigration(store, fs); err != nil {
			return errors.Wrap(err, "storage migration failed")
		}

		if err := os.RemoveAll(fsStoragePath); err != nil {
			log.Error().Err(err).Msg("failed to clean up deprecated storage")
		}
	}

	provisioners := primitives.NewPrimitivesProvisioner(cl)

	cap, err := capacity.NewResourceOracle(stubs.NewStorageModuleStub(cl)).Total()
	if err != nil {
		return errors.Wrap(err, "failed to get node capacity")
	}

	// update initial capacity with
	reserved, err := getNodeReserved(cl, cap)
	if err != nil {
		return errors.Wrap(err, "failed to get node reserved capacity")
	}
	var current gridtypes.Capacity
	if !app.IsFirstBoot(serverName) {
		// if this is the first boot of this module.
		// it means the provision engine will still
		// rerun all deployments, which means we don't need
		// to set the current consumed capacity from store
		// since the counters will get populated anyway.
		// but if not, we need to set the current counters
		// from store.
		current, err = store.Capacity()
		if err != nil {
			log.Error().Err(err).Msg("failed to compute current consumed capacity")
		}
	}

	log.Debug().Msgf("current used capacity: %+v", current)
	// statistics collects information about workload statistics
	// also does some checks on capacity
	statistics := primitives.NewStatistics(
		cap,
		current,
		reserved,
		provisioners,
	)

	if err := primitives.NewStatisticsMessageBus(testRouter, statistics); err != nil {
		return errors.Wrap(err, "failed to create statistics api")
	}

	sub, err := env.GetSubstrate()
	if err != nil {
		return errors.Wrap(err, "failed to get connection to substrate")
	}
	users, err := provision.NewSubstrateTwins(sub)
	if err != nil {
		return errors.Wrap(err, "failed to create substrate users database")
	}

	admins, err := provision.NewSubstrateAdmins(sub, uint32(env.FarmerID))
	if err != nil {
		return errors.Wrap(err, "failed to create substrate admins database")
	}

	kp, err := substrate.NewIdentityFromEd25519Key(sk)
	if err != nil {
		return errors.Wrap(err, "failed to get substrate keypair from secure key")
	}

	twin, err := sub.GetTwinByPubKey(kp.PublicKey())
	if err != nil {
		return errors.Wrap(err, "failed to get node twin id")
	}

	node, err := sub.GetNodeByTwinID(twin)
	if err != nil {
		return errors.Wrap(err, "failed to get node from twin")
	}

	queues := filepath.Join(rootDir, "queues")
	if err := os.MkdirAll(queues, 0755); err != nil {
		return errors.Wrap(err, "failed to create storage for queues")
	}

	engine, err := provision.New(
		store,
		statistics,
		queues,
		provision.WithTwins(users),
		provision.WithAdmins(admins),
		provision.WithSubstrate(node, sub),
		// set priority to some reservation types on boot
		// so we always need to make sure all volumes and networks
		// comes first.
		provision.WithStartupOrder(
			test.ZMountType,
			test.QuantumSafeFSType,
			test.NetworkType,
			test.PublicIPv4Type,
			test.PublicIPType,
		),
		// if this is a node reboot, the node needs to
		// recreate all reservations. so we set rerun = true
		provision.WithRerunAll(app.IsFirstBoot(serverName)),
	)

	if err != nil {
		return errors.Wrap(err, "failed to instantiate provision engine")
	}

	server.Register(
		zbus.ObjectID{Name: provisionModule, Version: "0.0.1"},
		pkg.Provision(engine),
	)

	server.Register(
		zbus.ObjectID{Name: statisticsModule, Version: "0.0.1"},
		pkg.Statistics(primitives.NewStatisticsStream(statistics)),
	)

	log.Info().
		Str("broker", msgBrokerCon).
		Msg("starting provision module")

	// call the runtime upgrade before running engine
	if err := provisioners.InitializeZDB(ctx); err != nil {
		log.Error().Err(err).Msg("failed to initialize zdb subsystem")
	}

	// spawn the engine
	go func() {
		if err := engine.Run(ctx); err != nil && err != context.Canceled {
			log.Fatal().Err(err).Msg("provision engine exited unexpectedely")
		}
	}()

	if err := app.MarkBooted(provisionModule); err != nil {
		log.Error().Err(err).Msg("failed to mark module as booted")
	}

	reporter, err := NewReporter(engine, node, cl, queues)
	if err != nil {
		return errors.Wrap(err, "failed to setup capacity reporter")
	}
	// also spawn the capacity reporter
	go func() {
		if err := reporter.Run(ctx); err != nil && err != context.Canceled {
			log.Fatal().Err(err).Msg("capacity reported stopped unexpectedely")
		}
		log.Info().Msg("capacity reported stopped")
	}()

	// and start the zbus server in the background
	go func() {
		if err := server.Run(ctx); err != nil && err != context.Canceled {
			log.Fatal().Err(err).Msg("zbus provision engine api exited unexpectedely")
		}
		log.Info().Msg("zbus server stopped")
	}()

	// register message bug api
	setupMessageBusses(testRouter, cl, engine)

	log.Info().Msg("running messagebus")

	for _, handler := range mBus.Handlers() {
		log.Debug().Msgf("registered handler: %s", handler)
	}

	if err := mBus.Run(ctx); err != nil && err != context.Canceled {
		return errors.Wrap(err, "message bus error")
	}

	log.Info().Msg("provision engine stopped")
	return nil
}

func getNodeReserved(cl zbus.Client, available gridtypes.Capacity) (counter primitives.Counters, err error) {
	// fill in reserved storage
	storage := stubs.NewStorageModuleStub(cl)
	fs, err := storage.Cache(context.TODO())
	if err != nil {
		return counter, err
	}

	counter.SRU.Increment(fs.Usage.Size)

	// we reserve 10% of memory to ZOS itself, with a min of 2G
	counter.MRU.Increment(
		gridtypes.Max(
			available.MRU*10/100,
			2*gridtypes.Gigabyte,
		),
	)

	return
}

func setupMessageBusses(router rmb.Router, cl zbus.Client, engine provision.Engine) error {

	_ = mbus.NewDeploymentMessageBus(router, engine)

	_ = mbus.NewNetworkMessagebus(router, engine, cl)

	return nil
}

func storageMigration(db *storage.BoltStorage, fs *fsStorage.Fs) error {
	log.Info().Msg("starting storage migration")
	twins, err := fs.Twins()
	if err != nil {
		return err
	}
	migration := db.Migration()
	errorred := false
	for _, twin := range twins {
		dls, err := fs.ByTwin(twin)
		if err != nil {
			log.Error().Err(err).Uint32("twin", twin).Msg("failed to list twin deployments")
			continue
		}

		sort.Slice(dls, func(i, j int) bool {
			return dls[i] < dls[j]
		})

		for _, dl := range dls {
			log.Info().Uint32("twin", twin).Uint64("deployment", dl).Msg("processing deployment migration")
			deployment, err := fs.Get(twin, dl)
			if err != nil {
				log.Error().Err(err).Uint32("twin", twin).Uint64("deployment", dl).Msg("failed to get deployment")
				errorred = true
				continue
			}
			if err := migration.Migrate(deployment); err != nil {
				log.Error().Err(err).Uint32("twin", twin).Uint64("deployment", dl).Msg("failed to migrate deployment")
				errorred = true
				continue
			}
			if err := fs.Delete(deployment); err != nil {
				log.Error().Err(err).Uint32("twin", twin).Uint64("deployment", dl).Msg("failed to delete migrated deployment")
				continue
			}
		}
	}

	if errorred {
		return fmt.Errorf("not all deployments where migrated")
	}

	return nil
}
