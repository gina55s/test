package identity

import (
	"fmt"

	"github.com/threefoldtech/testv2/modules/kernel"
)

// GetFarmID reads the farmer id from the kernel parameters
// return en error if the farmer id is not set
func GetFarmID() (Identifier, error) {
	params := kernel.GetParams()

	farmerID, found := params.Get("farmer_id")
	if !found {
		return nil, fmt.Errorf("farmer id not found in kernel parameters")
	}

	return StrIdentifier(farmerID[0]), nil
}
