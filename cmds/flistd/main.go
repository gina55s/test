package main

import (
	"context"
	"flag"
	"time"

	"github.com/threefoldtech/test/pkg/app"
	"github.com/threefoldtech/test/pkg/stubs"
	"github.com/threefoldtech/test/pkg/utils"

	"github.com/rs/zerolog/log"

	"github.com/threefoldtech/zbus"
	"github.com/threefoldtech/test/pkg/flist"
	"github.com/threefoldtech/test/pkg/version"
)

const (
	module = "flist"

	cacheAge     = time.Hour * 24 * 30 // 30 days
	cacheCleanup = time.Hour * 24
)

func main() {
	app.Initialize()

	var (
		moduleRoot   string
		msgBrokerCon string
		workerNr     uint
		ver          bool
	)

	flag.StringVar(&moduleRoot, "root", "/var/cache/modules/flistd", "root working directory of the module")
	flag.StringVar(&msgBrokerCon, "broker", "unix:///var/run/redis.sock", "connection string to the message broker")
	flag.UintVar(&workerNr, "workers", 1, "number of workers")
	flag.BoolVar(&ver, "v", false, "show version and exit")

	flag.Parse()
	if ver {
		version.ShowAndExit(false)
	}

	redis, err := zbus.NewRedisClient(msgBrokerCon)
	if err != nil {
		log.Fatal().Msgf("fail to connect to message broker server: %v", err)
	}
	storage := stubs.NewStorageModuleStub(redis)

	server, err := zbus.NewRedisServer(module, msgBrokerCon, workerNr)
	if err != nil {
		log.Fatal().Msgf("fail to connect to message broker server: %v\n", err)
	}

	mod := flist.New(moduleRoot, storage)
	server.Register(zbus.ObjectID{Name: module, Version: "0.0.1"}, mod)

	ctx, _ := utils.WithSignal(context.Background())

	if cleaner, ok := mod.(flist.Cleaner); ok {
		go cleaner.MountsCleaner(ctx, time.Minute)
		go cleaner.CacheCleaner(ctx, cacheCleanup, cacheAge)
	}

	log.Info().
		Str("broker", msgBrokerCon).
		Uint("worker nr", workerNr).
		Msg("starting flist module")

	utils.OnDone(ctx, func(_ error) {
		log.Info().Msg("shutting down")
	})

	if err := server.Run(ctx); err != nil && err != context.Canceled {
		log.Fatal().Err(err).Msg("unexpected error")
	}
}
