package yggdrasil

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"net"

	"github.com/jbenet/go-base58"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/threefoldtech/test/pkg/network/latency"

	"github.com/yggdrasil-network/yggdrasil-go/src/address"
	"github.com/yggdrasil-network/yggdrasil-go/src/config"
)

// List of port used by yggdrasil
const (
	YggListenTCP       = 9943
	YggListenTLS       = 9944
	YggListenLinkLocal = 9945

	YggIface = "ygg0"
)

// NodeConfig wrapper around yggdrasil node config
type NodeConfig config.NodeConfig

// Address gets the address from the config
func (n *NodeConfig) Address() (net.IP, error) {
	ip := make([]byte, net.IPv6len)
	pk, err := hex.DecodeString(n.PublicKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load public key")
	}
	copy(ip, address.AddrForKey(pk)[:])

	return ip, nil
}

func (n *NodeConfig) FindPeers(ctx context.Context, filter ...Filter) error {
	pl := fetchPeerList()
	peersUp, err := pl.Ups(filter...)
	if err != nil {
		return errors.Wrap(err, "failed to get peers list")
	}

	log.Info().Int("count", len(peersUp)).Msg("found yggdrasil up peers")
	endpoints := make([]string, len(peersUp))
	for i, p := range peersUp {
		endpoints[i] = p.Endpoint
	}

	ls := latency.NewSorter(endpoints, 5)
	results := ls.Run(ctx)
	if len(results) == 0 {
		return fmt.Errorf("cannot find public yggdrasil peer to connect to")
	}

	// select the best 3 public peers
	peers := make([]string, 3)
	for i := 0; i < 3; i++ {
		if len(results) > i {
			peers[i] = results[i].Endpoint
			log.Info().Str("endpoint", results[i].Endpoint).Msg("yggdrasill public peer selected")
		}
	}

	n.Peers = peers
	return nil
}

// GenerateConfig creates a new yggdrasil configuration and generate the
// box and signing key from the ed25519 Private key of the node
// this creates a mapping between a yggdrasil identity and the TFGrid identity
func GenerateConfig(privateKey ed25519.PrivateKey) (cfg NodeConfig) {
	cfg.IfMTU = 65535
	if privateKey != nil {
		cfg.PrivateKey = hex.EncodeToString(privateKey)

		signingPublicKey := privateKey.Public().(ed25519.PublicKey)
		cfg.PublicKey = hex.EncodeToString(signingPublicKey)

		cfg.NodeInfo = map[string]interface{}{
			"name": base58.Encode(signingPublicKey)[:6],
		}
	}

	cfg.MulticastInterfaces = []config.MulticastInterfaceConfig{
		{
			Regex:  ".*",
			Listen: true,
			Beacon: true,
			Port:   0,
		},
	}

	cfg.IfName = YggIface

	cfg.Listen = []string{
		fmt.Sprintf("tcp://[::]:%d", YggListenTCP),
		fmt.Sprintf("tls://[::]:%d", YggListenTLS),
	}

	return
}
