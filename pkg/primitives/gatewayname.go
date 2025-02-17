package primitives

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/threefoldtech/test/pkg/gridtypes"
	"github.com/threefoldtech/test/pkg/gridtypes/test"
	"github.com/threefoldtech/test/pkg/provision"
	"github.com/threefoldtech/test/pkg/stubs"
)

func (p *Primitives) gwProvision(ctx context.Context, wl *gridtypes.WorkloadWithID) (interface{}, error) {

	result := test.GatewayProxyResult{}
	var proxy test.GatewayNameProxy
	if err := json.Unmarshal(wl.Data, &proxy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal gateway proxy from reservation: %w", err)
	}
	backends := make([]string, len(proxy.Backends))
	for idx, backend := range proxy.Backends {
		backends[idx] = string(backend)
	}
	twinID, _ := provision.GetDeploymentID(ctx)
	gateway := stubs.NewGatewayStub(p.zbus)
	fqdn, err := gateway.SetNamedProxy(ctx, wl.ID.String(), proxy.Name, backends, proxy.TLSPassthrough, twinID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to setup name proxy")
	}
	result.FQDN = fqdn
	log.Debug().Str("domain", fqdn).Msg("domain reserved")
	return result, nil
}

func (p *Primitives) gwDecommission(ctx context.Context, wl *gridtypes.WorkloadWithID) error {
	gateway := stubs.NewGatewayStub(p.zbus)
	if err := gateway.DeleteNamedProxy(ctx, wl.ID.String()); err != nil {
		return errors.Wrap(err, "failed to delete name proxy")
	}
	return nil
}
