package registry

import (
	"context"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/bsdlp/apiutils"
	"github.com/pkg/errors"
)

// ListBuildings returns list of all buildings
func (dr *DynamoRegistrar) ListBuildings(ctx context.Context) (buildings []*Building, err error) {
	out, err := dr.DB.ScanWithContext(ctx, &dynamodb.ScanInput{
		TableName: aws.String(dr.Config.BuildingTableName),
	})
	if err != nil {
		err = errors.WithStack(err)
		return
	}

	buildings = make([]*Building, aws.Int64Value(out.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(out.Items, &buildings)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	return
}

// GetBuildingByID implements registrar
func (dr *DynamoRegistrar) GetBuildingByID(ctx context.Context, buildingID string) (building *Building, err error) {
	out, err := dr.DB.GetItemWithContext(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(dr.Config.BuildingTableName),
		Key: map[string]*dynamodb.AttributeValue{
			buildingIDAttributeName: {S: aws.String(buildingID)},
		},
	})
	if err != nil {
		err = errors.WithStack(err)
		return
	}

	if out.Item == nil {
		return
	}

	building = new(Building)
	err = dynamodbattribute.UnmarshalMap(out.Item, building)
	if err != nil {
		building = nil
		err = errors.WithStack(err)
		return
	}
	return
}

// RegisterBuilding implements Registrar
func (dr *DynamoRegistrar) RegisterBuilding(ctx context.Context, in *Building) (building *Building, err error) {
	if in == nil || in.Name == "" {
		err = apiutils.NewError(http.StatusBadRequest, "building name is required")
		return
	}

	building = new(Building)
	*building = *in
	building.ID = getULID().String()

	item, err := dynamodbattribute.MarshalMap(building)
	if err != nil {
		building = nil
		err = errors.WithStack(err)
		return
	}

	_, err = dr.DB.PutItemWithContext(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(dr.Config.BuildingTableName),
		Item:      item,
	})
	if err != nil {
		building = nil
		err = errors.WithStack(err)
		return
	}
	return
}

// DeregisterBuilding implements Registrar
func (dr *DynamoRegistrar) DeregisterBuilding(ctx context.Context, buildingID string) (err error) {
	_, err = dr.DB.DeleteItemWithContext(ctx, &dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			buildingIDAttributeName: {S: aws.String(buildingID)},
		},
		TableName: aws.String(dr.Config.BuildingTableName),
	})
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	return
}
