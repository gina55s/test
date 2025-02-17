package primitives

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/threefoldtech/test/pkg"
	"github.com/threefoldtech/test/pkg/gridtypes"
	"github.com/threefoldtech/test/pkg/gridtypes/test"
	"github.com/threefoldtech/test/pkg/provision"
	"github.com/threefoldtech/test/pkg/stubs"
)

func (p *Primitives) zlogsProvision(ctx context.Context, wl *gridtypes.WorkloadWithID) (interface{}, error) {

	var (
		vm      = stubs.NewVMModuleStub(p.zbus)
		network = stubs.NewNetworkerStub(p.zbus)
	)

	var cfg test.ZLogs
	if err := json.Unmarshal(wl.Data, &cfg); err != nil {
		return nil, errors.Wrap(err, "failed to decode zlogs config")
	}

	machine, err := provision.GetWorkload(ctx, cfg.ZMachine)
	if err != nil || machine.Type != test.ZMachineType {
		return nil, errors.Wrapf(err, "no zmachine with name '%s'", cfg.ZMachine)
	}

	if !machine.Result.State.IsAny(gridtypes.StateOk) {
		return nil, errors.Wrapf(err, "machine state is not ok")
	}

	var machineCfg test.ZMachine
	if err := json.Unmarshal(machine.Data, &machineCfg); err != nil {
		return nil, errors.Wrap(err, "failed to decode zlogs config")
	}

	var net gridtypes.Name

	if len(machineCfg.Network.Interfaces) > 0 {
		net = machineCfg.Network.Interfaces[0].Network
	} else {
		return nil, fmt.Errorf("invalid zmachine network configuration")
	}

	twin, _ := provision.GetDeploymentID(ctx)

	return nil, vm.StreamCreate(ctx, machine.ID.String(), pkg.Stream{
		ID:        wl.ID.String(),
		Namespace: network.Namespace(ctx, test.NetworkID(twin, net)),
		Output:    cfg.Output,
	})

}

func (p *Primitives) zlogsDecomission(ctx context.Context, wl *gridtypes.WorkloadWithID) error {
	var (
		vm = stubs.NewVMModuleStub(p.zbus)
	)

	return vm.StreamDelete(ctx, wl.ID.String())
}
