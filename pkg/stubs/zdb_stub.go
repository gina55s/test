package stubs

import (
	"context"
	zbus "github.com/threefoldtech/zbus"
	pkg "github.com/threefoldtech/test/pkg"
	gridtypes "github.com/threefoldtech/test/pkg/gridtypes"
	test "github.com/threefoldtech/test/pkg/gridtypes/test"
)

type ZDBAllocaterStub struct {
	client zbus.Client
	module string
	object zbus.ObjectID
}

func NewZDBAllocaterStub(client zbus.Client) *ZDBAllocaterStub {
	return &ZDBAllocaterStub{
		client: client,
		module: "storage",
		object: zbus.ObjectID{
			Name:    "storage",
			Version: "0.0.1",
		},
	}
}

func (s *ZDBAllocaterStub) Allocate(ctx context.Context, arg0 string, arg1 test.DeviceType, arg2 gridtypes.Unit, arg3 test.ZDBMode) (ret0 pkg.Allocation, ret1 error) {
	args := []interface{}{arg0, arg1, arg2, arg3}
	result, err := s.client.RequestContext(ctx, s.module, s.object, "Allocate", args...)
	if err != nil {
		panic(err)
	}
	if err := result.Unmarshal(0, &ret0); err != nil {
		panic(err)
	}
	ret1 = new(zbus.RemoteError)
	if err := result.Unmarshal(1, &ret1); err != nil {
		panic(err)
	}
	return
}

func (s *ZDBAllocaterStub) Find(ctx context.Context, arg0 string) (ret0 pkg.Allocation, ret1 error) {
	args := []interface{}{arg0}
	result, err := s.client.RequestContext(ctx, s.module, s.object, "Find", args...)
	if err != nil {
		panic(err)
	}
	if err := result.Unmarshal(0, &ret0); err != nil {
		panic(err)
	}
	ret1 = new(zbus.RemoteError)
	if err := result.Unmarshal(1, &ret1); err != nil {
		panic(err)
	}
	return
}
