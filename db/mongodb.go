package db

import (
	"context"
	"time"

	"bitbucket.org/ltman/mondex/schema"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const mongoConnectTimeout = 10 * time.Second

func ConnectToMongoDB(ctx context.Context, uri string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, mongoConnectTimeout)
	defer cancel()
	return mongo.Connect(ctx, options.Client().ApplyURI(uri))
}

func ReadCurrentSchema(ctx context.Context, db *mongo.Database) ([]schema.Schema, error) {
	collections, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	schemas := make([]schema.Schema, 0)

	for _, collectionName := range collections {
		collection := db.Collection(collectionName)
		cursor, err := collection.Indexes().List(ctx)
		if err != nil {
			return nil, err
		}

		var collectionIndexes []schema.Index
		if err := cursor.All(ctx, &collectionIndexes); err != nil {
			return nil, err
		}

		schemas = append(schemas, schema.Schema{
			Collection: collectionName,
			Indexes:    collectionIndexes,
		})
	}

	return schemas, nil
}
