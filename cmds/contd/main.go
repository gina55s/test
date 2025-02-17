package main

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"time"

	"github.com/cenkalti/backoff/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/threefoldtech/zbus"
	"github.com/threefoldtech/test/pkg/app"
	"github.com/threefoldtech/test/pkg/container"
	"github.com/threefoldtech/test/pkg/utils"
	"github.com/threefoldtech/test/pkg/version"
)

const module = "container"

func main() {
	app.Initialize()

	var (
		moduleRoot    string
		msgBrokerCon  string
		containerdCon string
		workerNr      uint
		debug         bool
		ver           bool
	)

	flag.StringVar(&moduleRoot, "root", "/var/cache/modules/contd", "root working directory of the module")
	flag.StringVar(&msgBrokerCon, "broker", "unix:///var/run/redis.sock", "connection string to the message broker")
	flag.StringVar(&containerdCon, "containerd", "/run/containerd/containerd.sock", "connection string to containerd")
	flag.UintVar(&workerNr, "workers", 1, "number of workers")
	flag.BoolVar(&debug, "debug", false, "enable debug logging")
	flag.BoolVar(&ver, "v", false, "show version and exit")

	flag.Parse()
	if ver {
		version.ShowAndExit(false)
	}

	// Default level is info, unless debug flag is present
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	// wait for shim-logs to be available before starting
	log.Info().Msg("wait for shim-logs binary to be available")
	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = 0 //forever
	_ = backoff.RetryNotify(func() error {
		_, err := exec.LookPath("shim-logs")
		return err
		// return fmt.Errorf("wait forever")
	}, bo, func(err error, d time.Duration) {
		log.Warn().Err(err).Msgf("shim-logs binary not found, retying in %s", d.String())
	})

	if err := os.MkdirAll(moduleRoot, 0750); err != nil {
		log.Fatal().Msgf("fail to create module root: %s", err)
	}

	server, err := zbus.NewRedisServer(module, msgBrokerCon, workerNr)
	if err != nil {
		log.Fatal().Msgf("fail to connect to message broker server: %v", err)
	}

	client, err := zbus.NewRedisClient(msgBrokerCon)
	if err != nil {
		log.Fatal().Msgf("fail to connect to message broker server: %v", err)
	}

	containerd := container.New(client, moduleRoot, containerdCon)

	server.Register(zbus.ObjectID{Name: module, Version: "0.0.1"}, containerd)

	log.Info().
		Str("broker", msgBrokerCon).
		Uint("worker nr", workerNr).
		Msg("starting containerd module")

	ctx, _ := utils.WithSignal(context.Background())
	utils.OnDone(ctx, func(_ error) {
		log.Info().Msg("shutting down")
	})

	// start watching for events
	go containerd.Watch(ctx)

	if err := server.Run(ctx); err != nil && err != context.Canceled {
		log.Fatal().Err(err).Msg("unexpected error")
	}
}
