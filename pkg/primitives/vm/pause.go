package vm

import (
	"context"

	"github.com/threefoldtech/test/pkg/gridtypes"
	"github.com/threefoldtech/test/pkg/provision"
	"github.com/threefoldtech/test/pkg/stubs"
)

func (m *Manager) Pause(ctx context.Context, wl *gridtypes.WorkloadWithID) error {
	vm := stubs.NewVMModuleStub(m.zbus)

	if err := vm.Lock(ctx, wl.ID.String(), true); err != nil {
		return provision.UnChanged(err)
	}

	return provision.Paused()
}

func (m *Manager) Resume(ctx context.Context, wl *gridtypes.WorkloadWithID) error {
	vm := stubs.NewVMModuleStub(m.zbus)

	if err := vm.Lock(ctx, wl.ID.String(), false); err != nil {
		return provision.UnChanged(err)
	}

	return provision.Ok()
}
