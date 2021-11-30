package pkg

//go:generate zbusc -module provision -version 0.0.1 -name provision -package stubs github.com/threefoldtech/test/pkg+Provision stubs/provision_stub.go
//go:generate zbusc -module provision -version 0.0.1 -name statistics -package stubs github.com/threefoldtech/test/pkg+Statistics stubs/statistics_stub.go

import (
	"context"

	"github.com/threefoldtech/test/pkg/gridtypes"
)


// Provision interface
type Provision interface {
	DecommissionCached(id string, reason string) error
}

type Statistics interface {
	Reserved(ctx context.Context) <-chan gridtypes.Capacity
}
