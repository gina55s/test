package primitives

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/threefoldtech/test/pkg"
	"github.com/threefoldtech/test/pkg/gridtypes"
	"github.com/threefoldtech/test/pkg/gridtypes/test"
	"github.com/threefoldtech/test/pkg/stubs"
)

// networkProvision is entry point to provision a network
func (p *Primitives) networkProvisionImpl(ctx context.Context, wl *gridtypes.Workload) error {
	var network test.Network
	if err := json.Unmarshal(wl.Data, &network); err != nil {
		return fmt.Errorf("failed to unmarshal network from reservation: %w", err)
	}

	if err := network.Valid(); err != nil {
		return fmt.Errorf("validation of the network resource failed: %w", err)
	}

	mgr := stubs.NewNetworkerStub(p.zbus)
	log.Debug().Str("network", fmt.Sprintf("%+v", network)).Msg("provision network")

	wgKey, err := p.decryptSecret(ctx, wl.User, network.WGPrivateKeyEncrypted, wl.Version)
	if err != nil {
		return errors.Wrap(err, "failed to decrypt wireguard private key")
	}
	_, err = mgr.CreateNR(pkg.Network{
		Network:           network,
		NetID:             test.NetworkID(wl.User.String(), network.Name),
		WGPrivateKeyPlain: wgKey,
	})

	if err != nil {
		return errors.Wrapf(err, "failed to create network resource for network %s", wl.ID)
	}

	return nil
}

func (p *Primitives) networkProvision(ctx context.Context, wl *gridtypes.Workload) (interface{}, error) {
	return nil, p.networkProvisionImpl(ctx, wl)
}

func (p *Primitives) networkDecommission(ctx context.Context, wl *gridtypes.Workload) error {
	mgr := stubs.NewNetworkerStub(p.zbus)

	var network test.Network
	if err := json.Unmarshal(wl.Data, &network); err != nil {
		return fmt.Errorf("failed to unmarshal network from reservation: %w", err)
	}

	if err := mgr.DeleteNR(pkg.Network{
		Network: network,
		NetID:   test.NetworkID(wl.User.String(), network.Name),
	}); err != nil {
		return fmt.Errorf("failed to delete network resource: %w", err)
	}

	return nil
}
