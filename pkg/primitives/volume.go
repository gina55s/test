package primitives

import (
	"context"
	"encoding/json"

	"github.com/rs/zerolog/log"

	"github.com/threefoldtech/test/pkg/gridtypes"
	"github.com/threefoldtech/test/pkg/gridtypes/test"
	"github.com/threefoldtech/test/pkg/stubs"
)

const (
	// gigabyte to byte conversion
	gigabyte uint64 = 1024 * 1024 * 1024
)

// Volume defines a mount point
type Volume = test.Volume

// VolumeResult types
type VolumeResult = test.VolumeResult

func (p *Primitives) volumeProvisionImpl(ctx context.Context, wl *gridtypes.Workload) (VolumeResult, error) {
	var config Volume
	if err := json.Unmarshal(wl.Data, &config); err != nil {
		return VolumeResult{}, err
	}

	storageClient := stubs.NewStorageModuleStub(p.zbus)

	_, err := storageClient.Path(wl.ID.String())
	if err == nil {
		log.Info().Stringer("id", wl.ID).Msg("volume already deployed")
		return VolumeResult{
			ID: wl.ID.String(),
		}, nil
	}

	_, err = storageClient.CreateFilesystem(FilesystemName(wl), config.Size*gigabyte, config.Type)

	return VolumeResult{
		ID: wl.ID.String(),
	}, err
}

// VolumeProvision is entry point to provision a volume
func (p *Primitives) volumeProvision(ctx context.Context, wl *gridtypes.Workload) (interface{}, error) {
	return p.volumeProvisionImpl(ctx, wl)
}

func (p *Primitives) volumeDecommission(ctx context.Context, wl *gridtypes.Workload) error {
	storageClient := stubs.NewStorageModuleStub(p.zbus)

	return storageClient.ReleaseFilesystem(wl.ID.String())
}
