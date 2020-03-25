package app

import (
	"github.com/pkg/errors"
	"github.com/threefoldtech/test/pkg/environment"
	"github.com/threefoldtech/test/pkg/identity"
	"github.com/threefoldtech/test/tools/client"
)

const seedPath = "/var/cache/modules/identityd/seed.txt"

// ExplorerClient return the client to the explorer based
// on the environment configured in the kernel arguments
func ExplorerClient() (*client.Client, error) {
	env, err := environment.Get()
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse node environment")
	}

	kp, err := identity.LoadKeyPair(seedPath)
	if err != nil {
		return nil, err
	}

	cl, err := client.NewClient(env.BcdbURL, kp)
	if err != nil {
		return nil, err
	}

	return cl, nil
}
