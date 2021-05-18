package capacity

import (
	"context"
	"math"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/threefoldtech/test/pkg/gridtypes"
	"github.com/threefoldtech/test/pkg/gridtypes/test"
)

func (r *ResourceOracle) cru() (uint64, error) {
	n, err := cpu.Counts(true)
	return uint64(n), err
}

func (r *ResourceOracle) mru() (gridtypes.Unit, error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}

	// we round the value to nearest Gigabyte
	total := math.Round(float64(vm.Total)/float64(gridtypes.Gigabyte)) * float64(gridtypes.Gigabyte)
	return gridtypes.Unit(total), nil
}

func (r *ResourceOracle) sru() (gridtypes.Unit, error) {
	total, err := r.storage.Total(context.TODO(), test.SSDDevice)
	if err != nil {
		return 0, err
	}

	return gridtypes.Unit(total), nil
}

func (r *ResourceOracle) hru() (gridtypes.Unit, error) {
	total, err := r.storage.Total(context.TODO(), test.HDDDevice)
	if err != nil {
		return 0, err
	}

	return gridtypes.Unit(total), nil
}
