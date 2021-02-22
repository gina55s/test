package main

import (
	"fmt"

	"github.com/threefoldtech/test/pkg/gridtypes/test"
	"github.com/threefoldtech/test/pkg/storage"
)

func main() {
	s, err := storage.New()
	if err != nil {
		panic(fmt.Sprintf("%v", err))
	}

	kind := test.SSDDevice
	total, err := s.Total(kind)
	if err != nil {
		panic(fmt.Sprintf("%v", err))
	}

	fmt.Printf("SSD: %v\n", total)

	kind = test.HDDDevice
	total, err = s.Total(kind)
	if err != nil {
		panic(fmt.Sprintf("%v", err))
	}

	fmt.Printf("HDD: %v\n", total)
}
