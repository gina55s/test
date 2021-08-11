package primitives

import (
	"github.com/threefoldtech/zbus"
	"github.com/threefoldtech/test/pkg/gridtypes"
	"github.com/threefoldtech/test/pkg/gridtypes/test"
	"github.com/threefoldtech/test/pkg/provision"
)

// Primitives hold all the logic responsible to provision and decomission
// the different primitives workloads defined by this package
type Primitives struct {
	provision.Provisioner
	zbus zbus.Client
}

var _ provision.Provisioner = (*Primitives)(nil)

// NewPrimitivesProvisioner creates a new 0-OS provisioner
func NewPrimitivesProvisioner(zbus zbus.Client) *Primitives {
	p := &Primitives{
		zbus: zbus,
	}

	provisioners := map[gridtypes.WorkloadType]provision.DeployFunction{
		test.ZMountType:   p.zMountProvision,
		test.NetworkType:  p.networkProvision,
		test.ZDBType:      p.zdbProvision,
		test.ZMachineType: p.virtualMachineProvision,
		test.PublicIPType: p.publicIPProvision,
	}
	decommissioners := map[gridtypes.WorkloadType]provision.RemoveFunction{
		test.ZMountType:   p.zMountDecommission,
		test.NetworkType:  p.networkDecommission,
		test.ZDBType:      p.zdbDecommission,
		test.ZMachineType: p.vmDecomission,
		test.PublicIPType: p.publicIPDecomission,
	}

	// only network support update atm
	updaters := map[gridtypes.WorkloadType]provision.DeployFunction{
		test.NetworkType: p.networkProvision,
	}

	p.Provisioner = provision.NewMapProvisioner(provisioners, decommissioners, updaters)

	return p
}
