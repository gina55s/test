package primitives

import (
	"github.com/threefoldtech/zbus"
	"github.com/threefoldtech/test/pkg/gridtypes"
	"github.com/threefoldtech/test/pkg/gridtypes/test"
	"github.com/threefoldtech/test/pkg/primitives/gateway"
	"github.com/threefoldtech/test/pkg/primitives/network"
	"github.com/threefoldtech/test/pkg/primitives/pubip"
	"github.com/threefoldtech/test/pkg/primitives/qsfs"
	"github.com/threefoldtech/test/pkg/primitives/vm"
	"github.com/threefoldtech/test/pkg/primitives/zdb"
	"github.com/threefoldtech/test/pkg/primitives/zlogs"
	"github.com/threefoldtech/test/pkg/primitives/zmount"
	"github.com/threefoldtech/test/pkg/provision"
)

// NewPrimitivesProvisioner creates a new 0-OS provisioner
func NewPrimitivesProvisioner(zbus zbus.Client) provision.Provisioner {
	managers := map[gridtypes.WorkloadType]provision.Manager{
		test.ZMountType:           zmount.NewManager(zbus),
		test.ZLogsType:            zlogs.NewManager(zbus),
		test.QuantumSafeFSType:    qsfs.NewManager(zbus),
		test.ZDBType:              zdb.NewManager(zbus),
		test.NetworkType:          network.NewManager(zbus),
		test.PublicIPType:         pubip.NewManager(zbus),
		test.PublicIPv4Type:       pubip.NewManager(zbus), // backward compatibility
		test.ZMachineType:         vm.NewManager(zbus),
		test.GatewayNameProxyType: gateway.NewNameManager(zbus),
		test.GatewayFQDNProxyType: gateway.NewFQDNManager(zbus),
	}

	return provision.NewMapProvisioner(managers)
}
