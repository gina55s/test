package directory

import (
	"context"

	"github.com/pkg/errors"
	"github.com/threefoldtech/test/pkg/schema"
	"github.com/threefoldtech/test/tools/bcdb_mock/models"
	directory "github.com/threefoldtech/test/tools/bcdb_mock/pkg/directory/types"
	"go.mongodb.org/mongo-driver/mongo"
)

// FarmAPI holds farm releated handlers
type FarmAPI struct{}

// List farms
// TODO: add paging arguments
func (s *FarmAPI) List(ctx context.Context, db *mongo.Database) ([]directory.Farm, error) {
	var filter directory.FarmFilter
	//options.Find().
	cur, err := filter.Find(ctx, db, models.Page(0))
	if err != nil {
		return nil, errors.Wrap(err, "failed to list farms")
	}
	defer cur.Close(ctx)
	var out []directory.Farm
	if err := cur.All(ctx, &out); err != nil {
		return nil, errors.Wrap(err, "failed to load farm list")
	}

	return out, nil
}

// GetByName gets a farm by name
func (s *FarmAPI) GetByName(ctx context.Context, db *mongo.Database, name string) (directory.Farm, error) {
	var filter directory.FarmFilter
	filter = filter.WithName(name)

	return filter.Get(ctx, db)
}

// GetByID gets a farm by ID
func (s *FarmAPI) GetByID(ctx context.Context, db *mongo.Database, id int64) (directory.Farm, error) {
	var filter directory.FarmFilter
	filter = filter.WithID(schema.ID(id))

	return filter.Get(ctx, db)
}

// Add add farm to store
// TODO: support update farm information ?
func (s *FarmAPI) Add(ctx context.Context, db *mongo.Database, farm directory.Farm) (schema.ID, error) {
	return directory.FarmCreate(ctx, db, farm)
}
