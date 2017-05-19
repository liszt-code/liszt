package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntegrationBuildings(t *testing.T) {
	t.Run("get nonexistent building", func(t *testing.T) {
		assert := assert.New(t)
		building, err := testRegistrar.GetBuildingByID(context.Background(), "nonexistent")
		assert.NoError(err)
		assert.Nil(building)
	})

	var registeredBuilding *Building
	t.Run("register building", func(t *testing.T) {
		assert := assert.New(t)
		building := &Building{
			ID:   "something",
			Name: getULID().String(),
		}
		var err error
		registeredBuilding, err = testRegistrar.RegisterBuilding(context.Background(), building)
		assert.NoError(err)
		assert.NotEmpty(registeredBuilding.ID)
		assert.NotEqual(building.ID, registeredBuilding.ID, "registerbuliding should generate its own id")

	})

	t.Run("get existing building", func(t *testing.T) {
		assert := assert.New(t)
		building, err := testRegistrar.GetBuildingByID(context.Background(), registeredBuilding.ID)
		assert.NoError(err)
		assert.Equal(registeredBuilding, building)
	})

	t.Run("deregister building", func(t *testing.T) {
		assert := assert.New(t)
		err := testRegistrar.DeregisterBuilding(context.Background(), registeredBuilding.ID)
		assert.NoError(err)
	})

	t.Run("get deregistered building", func(t *testing.T) {
		assert := assert.New(t)
		building, err := testRegistrar.GetBuildingByID(context.Background(), registeredBuilding.ID)
		assert.NoError(err)
		assert.Nil(building)
	})

	t.Run("deregister nonexistent building should not error", func(t *testing.T) {
		assert := assert.New(t)
		err := testRegistrar.DeregisterBuilding(context.Background(), "nonexistent")
		assert.NoError(err)
	})
}
