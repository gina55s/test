package nr

import (
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/threefoldtech/testv2/modules"
	"github.com/vishvananda/netlink"
)

type testIdentityManager struct {
	id   string
	farm string
}

var _ modules.IdentityManager = (*testIdentityManager)(nil)

// NodeID returns the node id (public key)
func (t *testIdentityManager) NodeID() modules.StrIdentifier {
	return modules.StrIdentifier(t.id)
}

// FarmID return the farm id this node is part of. this is usually a configuration
// that the node is booted with. An error is returned if the farmer id is not configured
func (t *testIdentityManager) FarmID() (modules.StrIdentifier, error) {
	return modules.StrIdentifier(t.farm), nil
}

// Sign signs the message with privateKey and returns a signature.
func (t *testIdentityManager) Sign(message []byte) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

// Verify reports whether sig is a valid signature of message by publicKey.
func (t *testIdentityManager) Verify(message, sig []byte) error {
	return fmt.Errorf("not implemented")
}

// Encrypt encrypts message with the public key of the node
func (t *testIdentityManager) Encrypt(message []byte) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

// Decrypt decrypts message with the private of the node
func (t *testIdentityManager) Decrypt(message []byte) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestNamespace(t *testing.T) {
	nr, err := New("networkd1", &modules.NetResource{
		NodeID: "node1",
	})
	require.NoError(t, err)

	nsName, err := nr.Namespace()
	require.NoError(t, err)

	brName, err := nr.BridgeName()
	require.NoError(t, err)

	wgName, err := nr.WGName()
	require.NoError(t, err)

	assert.Equal(t, "net-networkd1", nsName)
	assert.Equal(t, "br-networkd1", brName)
	assert.Equal(t, "wg-networkd1", wgName)
}

func TestNaming(t *testing.T) {
	nr, err := New("networkd1", &modules.NetResource{
		NodeID: "node1",
	})
	require.NoError(t, err)

	nsName, err := nr.Namespace()
	require.NoError(t, err)

	brName, err := nr.BridgeName()
	require.NoError(t, err)

	wgName, err := nr.WGName()
	require.NoError(t, err)

	assert.Equal(t, "net-networkd1", nsName)
	assert.Equal(t, "br-networkd1", brName)
	assert.Equal(t, "wg-networkd1", wgName)
}

func TestCreateBridge(t *testing.T) {
	nr, err := New("networkd1", &modules.NetResource{
		NodeID: "node1",
	})
	require.NoError(t, err)

	brName, err := nr.BridgeName()
	require.NoError(t, err)

	err = nr.createBridge()
	require.NoError(t, err)

	l, err := netlink.LinkByName(brName)
	assert.NoError(t, err)
	_, ok := l.(*netlink.Bridge)
	assert.True(t, ok)

	// cleanup
	netlink.LinkDel(l)
}

func mustParseCIDR(cidr string) *net.IPNet {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	ipnet.IP = ip
	return ipnet
}

func Test_wgIP(t *testing.T) {
	type args struct {
		subnet *net.IPNet
	}
	tests := []struct {
		name string
		args args
		want *net.IPNet
	}{
		{
			name: "default",
			args: args{
				subnet: &net.IPNet{
					IP:   net.ParseIP("10.3.1.0"),
					Mask: net.CIDRMask(16, 32),
				},
			},
			want: &net.IPNet{
				IP:   net.ParseIP("100.64.3.1"),
				Mask: net.CIDRMask(16, 32),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wgIP(tt.args.subnet); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("wgIP() = %v, want %v", got, tt.want)
			}
		})
	}
}
