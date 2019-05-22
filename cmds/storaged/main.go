package main

import (
	"github.com/threefoldtech/testv2/modules/storage/pkg/disks"

	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.TraceLevel)
	if err := disks.EnsureCache(); err != nil {
		panic(err)
	}
}
