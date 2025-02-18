package db

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/ltman/mondex/schema"
)

func ConnectToMongoDB(uri string) (*mongo.Client, error) {
	return mongo.Connect(options.Client().ApplyURI(uri))
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

		for i, indexes := range collectionIndexes {
			// NOTE: The index is a fts index,
			// MongoDB doesn't return what fields are used in the key,
			// So we will do ourselves.
			if len(indexes.Weights) > 0 {
				var key bson.D
				for _, weight := range indexes.Weights {
					key = append(key, bson.E{Key: weight.Key, Value: "text"})
				}
				collectionIndexes[i].Key = key
			}
		}

		schemas = append(schemas, schema.Schema{
			Collection: collectionName,
			Indexes:    collectionIndexes,
		})
	}

	return schemas, nil
}
