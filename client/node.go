package client

import (
	"context"
	"net"

	"github.com/threefoldtech/tfgrid-sdk-go/rmb-sdk-go"
	"github.com/threefoldtech/test/pkg"
	"github.com/threefoldtech/test/pkg/capacity/dmi"
	"github.com/threefoldtech/test/pkg/diagnostics"
	"github.com/threefoldtech/test/pkg/gridtypes"
)

// NodeClient struct
type NodeClient struct {
	nodeTwin uint32
	bus      rmb.Client
}

type Version struct {
	ZOS   string `json:"test"`
	ZInit string `json:"zinit"`
}

type Interface struct {
	IPs []string `json:"ips"`
	Mac string   `json:"mac"`
}

type ExitDevice struct {
	// IsSingle is set to true if br-pub
	// is connected to test bridge
	IsSingle bool `json:"is_single"`
	// IsDual is set to true if br-pub is
	// connected to a physical nic
	IsDual bool `json:"is_dual"`
	// AsDualInterface is set to the physical
	// interface name if IsDual is true
	AsDualInterface string `json:"dual_interface"`
}

type args map[string]interface{}

// NewNodeClient creates a new node RMB client. This client then can be used to
// communicate with the node over RMB.
func NewNodeClient(nodeTwin uint32, bus rmb.Client) *NodeClient {
	return &NodeClient{nodeTwin, bus}
}

// DeploymentDeploy sends the deployment to the node for processing.
func (n *NodeClient) DeploymentDeploy(ctx context.Context, dl gridtypes.Deployment) error {
	const cmd = "test.deployment.deploy"
	return n.bus.Call(ctx, n.nodeTwin, cmd, dl, nil)
}

// DeploymentUpdate update the given deployment. deployment must be a valid update for
// a deployment that has been already created via DeploymentDeploy
func (n *NodeClient) DeploymentUpdate(ctx context.Context, dl gridtypes.Deployment) error {
	const cmd = "test.deployment.update"
	return n.bus.Call(ctx, n.nodeTwin, cmd, dl, nil)
}

// DeploymentGet gets a deployment via contract ID
func (n *NodeClient) DeploymentGet(ctx context.Context, contractID uint64) (dl gridtypes.Deployment, err error) {
	const cmd = "test.deployment.get"
	in := args{
		"contract_id": contractID,
	}

	if err = n.bus.Call(ctx, n.nodeTwin, cmd, in, &dl); err != nil {
		return dl, err
	}

	return dl, nil
}

// DeploymentList gets all deployments for a twin
func (n *NodeClient) DeploymentList(ctx context.Context) (dls []gridtypes.Deployment, err error) {
	const cmd = "test.deployment.list"

	err = n.bus.Call(ctx, n.nodeTwin, cmd, nil, &dls)
	return
}

// DeploymentChanges gets changes to deployment via contract ID
func (n *NodeClient) DeploymentChanges(ctx context.Context, contractID uint64) (changes []gridtypes.Workload, err error) {
	const cmd = "test.deployment.changes"
	in := args{
		"contract_id": contractID,
	}

	if err = n.bus.Call(ctx, n.nodeTwin, cmd, in, &changes); err != nil {
		return changes, err
	}

	return changes, nil
}

// DeploymentDelete deletes a deployment, the node will make sure to decomission all deployments
// and set all workloads to deleted. A call to Get after delete is valid
func (n *NodeClient) DeploymentDelete(ctx context.Context, contractID uint64) error {
	const cmd = "test.deployment.delete"
	in := args{
		"contract_id": contractID,
	}

	return n.bus.Call(ctx, n.nodeTwin, cmd, in, nil)
}

// Counters (statistics) of the node
type Counters struct {
	// Total system capacity
	Total gridtypes.Capacity `json:"total"`
	// Used capacity this include user + system resources
	Used gridtypes.Capacity `json:"used"`
	// System resource reserved by test
	System gridtypes.Capacity `json:"system"`
	// Users statistics by test
	Users UsersCounters `json:"users"`
}

// UsersCounters the expected counters for deployments and workloads
type UsersCounters struct {
	// Total deployments count
	Deployments int `json:"deployments"`
	// Total workloads count
	Workloads int `json:"workloads"`
}

// GPU information
type GPU struct {
	ID       string `json:"id"`
	Vendor   string `json:"vendor"`
	Device   string `json:"device"`
	Contract uint64 `json:"contract"`
}

// Counters returns some node statistics. Including total and available cpu, memory, storage, etc...
func (n *NodeClient) Counters(ctx context.Context) (counters Counters, err error) {
	const cmd = "test.statistics.get"
	err = n.bus.Call(ctx, n.nodeTwin, cmd, nil, &counters)
	return
}

// Pools returns statistics of separate pools
func (n *NodeClient) Pools(ctx context.Context) (pools []pkg.PoolMetrics, err error) {
	const cmd = "test.storage.pools"
	err = n.bus.Call(ctx, n.nodeTwin, cmd, nil, &pools)
	return
}

func (n *NodeClient) GPUs(ctx context.Context) (gpus []GPU, err error) {
	const cmd = "test.gpu.list"
	err = n.bus.Call(ctx, n.nodeTwin, cmd, nil, &gpus)
	return
}

// NetworkListWGPorts return a list of all "taken" ports on the node. A new deployment
// should be careful to use a free port for its network setup.
func (n *NodeClient) NetworkListWGPorts(ctx context.Context) ([]uint16, error) {
	const cmd = "test.network.list_wg_ports"
	var result []uint16

	if err := n.bus.Call(ctx, n.nodeTwin, cmd, nil, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (n *NodeClient) HasPublicIPv6(ctx context.Context) (bool, error) {
	const cmd = "test.network.has_ipv6"
	var result bool

	if err := n.bus.Call(ctx, n.nodeTwin, cmd, nil, &result); err != nil {
		return false, err
	}

	return result, nil
}

func (n *NodeClient) NetworkListInterfaces(ctx context.Context) (result map[string][]net.IP, err error) {
	const cmd = "test.network.interfaces"

	err = n.bus.Call(ctx, n.nodeTwin, cmd, nil, &result)

	return
}

// NetworkListAllInterfaces return all physical devices on a node
func (n *NodeClient) NetworkListAllInterfaces(ctx context.Context) (result map[string]Interface, err error) {
	const cmd = "test.network.admin.interfaces"

	err = n.bus.Call(ctx, n.nodeTwin, cmd, nil, &result)

	return
}

// NetworkSetPublicExitDevice select which physical interface to use as an exit device
// setting `iface` to `test` will then make node run in a single nic setup.
func (n *NodeClient) NetworkSetPublicExitDevice(ctx context.Context, iface string) error {
	const cmd = "test.network.admin.set_public_nic"

	return n.bus.Call(ctx, n.nodeTwin, cmd, iface, nil)
}

// NetworkGetPublicExitDevice gets the current dual nic setup of the node.
func (n *NodeClient) NetworkGetPublicExitDevice(ctx context.Context) (exit ExitDevice, err error) {
	const cmd = "test.network.admin.get_public_nic"

	err = n.bus.Call(ctx, n.nodeTwin, cmd, nil, &exit)
	return
}

// NetworkListPublicIPs list taken public IPs on the node
func (n *NodeClient) NetworkListPublicIPs(ctx context.Context) ([]string, error) {
	const cmd = "test.network.list_public_ips"
	var result []string

	if err := n.bus.Call(ctx, n.nodeTwin, cmd, nil, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// NetworkListPrivateIPs list private ips reserved for a network
func (n *NodeClient) NetworkListPrivateIPs(ctx context.Context, networkName string) ([]string, error) {
	const cmd = "test.network.list_private_ips"
	var result []string
	in := args{
		"network_name": networkName,
	}

	if err := n.bus.Call(ctx, n.nodeTwin, cmd, in, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// NetworkGetPublicConfig returns the current public node network configuration. A node with a
// public config can be used as an access node for wireguard.
func (n *NodeClient) NetworkGetPublicConfig(ctx context.Context) (cfg pkg.PublicConfig, err error) {
	const cmd = "test.network.public_config_get"

	err = n.bus.Call(ctx, n.nodeTwin, cmd, nil, &cfg)
	return
}

func (n *NodeClient) SystemGetNodeFeatures(ctx context.Context) (feat []pkg.NodeFeature, err error) {
	const cmd = "test.system.node_features_get"

	err = n.bus.Call(ctx, n.nodeTwin, cmd, nil, &feat)
	return
}

func (n *NodeClient) SystemVersion(ctx context.Context) (ver Version, err error) {
	const cmd = "test.system.version"

	if err = n.bus.Call(ctx, n.nodeTwin, cmd, nil, &ver); err != nil {
		return
	}

	return
}

func (n *NodeClient) SystemDMI(ctx context.Context) (result dmi.DMI, err error) {
	const cmd = "test.system.dmi"

	if err = n.bus.Call(ctx, n.nodeTwin, cmd, nil, &result); err != nil {
		return
	}

	return
}

func (n *NodeClient) SystemHypervisor(ctx context.Context) (result string, err error) {
	const cmd = "test.system.hypervisor"

	if err = n.bus.Call(ctx, n.nodeTwin, cmd, nil, &result); err != nil {
		return
	}

	return
}

func (n *NodeClient) SystemDiagnostics(ctx context.Context) (result diagnostics.Diagnostics, err error) {
	const cmd = "test.system.diagnostics"
	err = n.bus.Call(ctx, n.nodeTwin, cmd, nil, &result)
	return
}
