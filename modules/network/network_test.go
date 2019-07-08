package network

import (
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/threefoldtech/testv2/modules/network/bridge"
	testip "github.com/threefoldtech/testv2/modules/network/ip"

	"github.com/threefoldtech/testv2/modules/network/namespace"

	"github.com/threefoldtech/testv2/modules"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

var peers = []*modules.Peer{
	{
		Type:   modules.ConnTypeWireguard,
		Prefix: mustParseCIDR("2a02:1802:5e:ff02::/64"),
		Connection: modules.Wireguard{
			IP:   net.ParseIP("2001:1:1:1::1"),
			Port: 1601,
			Key:  "4w4woC+AuDUAaRipT49M8SmTkzERps3xA5i0BW4hPiw=",
		},
	},
	{
		Type:   modules.ConnTypeWireguard,
		Prefix: mustParseCIDR("2a02:1802:5e:cc02::/64"),
		Connection: modules.Wireguard{
			IP:   net.ParseIP("2001:1:1:2::1"),
			Port: 1602,
			Key:  "HXnTmizQdGlAuE9PpVPw1Drg2WygUsxwGnJY+A5xgVo=",
		},
	},
	{
		Type:   modules.ConnTypeWireguard,
		Prefix: mustParseCIDR("2a02:1802:5e:aaaa::/64"),
		Connection: modules.Wireguard{
			IP:   net.ParseIP("2001:3:3:3::3"),
			Port: 1603,
			Key:  "5Adc456lkjlRtRipT49M8SmTkzERps3xA5i0BW4hPiw=",
		},
	},
}

var node1 = &modules.NetResource{
	NodeID: &modules.NodeID{
		ID:             "node1",
		ReachabilityV4: modules.ReachabilityV4Public,
		ReachabilityV6: modules.ReachabilityV6Public,
	},
	Prefix:    mustParseCIDR("2a02:1802:5e:ff02::/64"),
	LinkLocal: mustParseCIDR("fe80::ff02/64"),
	Peers:     peers,
	ExitPoint: true,
}
var node2 = &modules.NetResource{
	NodeID: &modules.NodeID{
		ID:             "node2",
		ReachabilityV4: modules.ReachabilityV4Hidden,
		ReachabilityV6: modules.ReachabilityV6ULA,
	},
	Prefix:    mustParseCIDR("2a02:1802:5e:cc02::/64"),
	LinkLocal: mustParseCIDR("fe80::cc02/64"),
	Peers:     peers,
}

var node3 = &modules.NetResource{
	NodeID: &modules.NodeID{
		ID:             "node3",
		ReachabilityV4: modules.ReachabilityV4Public,
		ReachabilityV6: modules.ReachabilityV6Public,
	},
	Prefix:    mustParseCIDR("2a02:1802:5e:aaaa::/64"),
	LinkLocal: mustParseCIDR("fe80::aaaa/64"),
	Peers:     peers,
}

var networks = []*modules.Network{
	{
		NetID: "net1",
		Resources: []*modules.NetResource{
			node1, node2, node3,
		},
		PrefixZero: mustParseCIDR("2a02:1802:5e:0000::/64"),
		Exit:       &modules.ExitPoint{},
	},
}

func TestCreateNetwork(t *testing.T) {
	var (
		network    = networks[0]
		resource   = network.Resources[0]
		nibble     = testip.NewNibble(resource.Prefix, network.AllocationNR)
		netName    = nibble.NetworkName()
		bridgeName = nibble.BridgeName()
		vethName   = nibble.VethName()
	)

	dir, err := ioutil.TempDir("", netName)
	require.NoError(t, err)

	storage := filepath.Join(dir, netName)
	networker := &networker{
		nodeID:      *node1.NodeID,
		storageDir:  storage,
		netResAlloc: nil,
	}

	for _, tc := range []struct {
		exitIface *ExitIface
	}{
		{
			exitIface: nil,
		},
		{
			exitIface: &ExitIface{
				Master: "test0",
				Type:   MacVlanIface,
				IPv6:   mustParseCIDR("2a02:1802:5e:ff02::100/64"),
				GW6:    net.ParseIP("fe80::1"),
			},
		},
	} {
		name := "withPublicNamespace"
		if tc.exitIface == nil {
			name = "NoPubNamespace"
		}
		t.Run(name, func(t *testing.T) {
			defer func() {
				err := networker.DeleteNetResource(network)
				require.NoError(t, err)
				if tc.exitIface != nil {
					pubNs, _ := namespace.GetByName(PublicNamespace)
					err = namespace.Delete(pubNs)
					require.NoError(t, err)
				}
			}()

			if tc.exitIface != nil {
				err := CreatePublicNS(tc.exitIface)
				require.NoError(t, err)
			}

			err := createNetworkResource(resource, network)
			require.NoError(t, err)

			assert.True(t, bridge.Exists(bridgeName))
			assert.True(t, namespace.Exists(netName))

			netns, err := namespace.GetByName(netName)
			require.NoError(t, err)
			defer netns.Close()
			var handler = func(_ ns.NetNS) error {
				link, err := netlink.LinkByName(vethName)
				require.NoError(t, err)

				addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
				require.NoError(t, err)
				assert.Equal(t, 1, len(addrs))
				assert.Equal(t, "10.255.2.1/24", addrs[0].IPNet.String())

				addrs, err = netlink.AddrList(link, netlink.FAMILY_V6)
				require.NoError(t, err)
				assert.Equal(t, 2, len(addrs))
				assert.Equal(t, "2a02:1802:5e:ff02::/64", addrs[0].IPNet.String())

				return nil
			}
			err = netns.Do(handler)
			assert.NoError(t, err)
		})
	}

}

func TestConfigureWG(t *testing.T) {
	var (
		network = networks[0]
	)

	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	networker := &networker{
		nodeID:      *node1.NodeID,
		storageDir:  dir,
		netResAlloc: nil,
	}

	_, err = networker.GenerateWireguarKeyPair(network.NetID)
	require.NoError(t, err)

	defer func() {
		_ = networker.DeleteNetResource(network)
		_ = os.RemoveAll(dir)
	}()

	err = networker.ApplyNetResource(network)
	require.NoError(t, err)
}

func mustParseCIDR(cidr string) *net.IPNet {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	ipnet.IP = ip
	return ipnet
}
