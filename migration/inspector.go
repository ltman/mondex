package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"bitbucket.org/ltman/mondex/db"
	"bitbucket.org/ltman/mondex/schema"
)

func InspectCurrentSchema(
	ctx context.Context,
	logger *slog.Logger,
	mongoURI, databaseName string,
	schemaFilePath string,
	dryRun bool,
) error {
	logger.Debug("Connecting to MongoDB")
	client, err := db.ConnectToMongoDB(ctx, mongoURI)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			logger.Error("Failed to disconnect from MongoDB", "error", err)
		}
	}()

	logger.Debug("Reading current schema from MongoDB")
	current, err := db.ReadCurrentSchema(ctx, client.Database(databaseName))
	if err != nil {
		return fmt.Errorf("failed to read current schema: %w", err)
	}

	logger.Debug("Writing current schema to file", "path", schemaFilePath)
	if dryRun {
		logger.Info("Dry run: showing schema without writing file")
		if err := prettyPrintSchemas(current); err != nil {
			return fmt.Errorf("failed to print current schema: %w", err)
		}
	} else {
		if err := writeSchemas(current, schemaFilePath); err != nil {
			return fmt.Errorf("failed to write current schema: %w", err)
		}
		logger.Info("Successfully wrote current schema to file", "path", schemaFilePath)
	}

	return nil
}

func prettyPrintSchemas(schemas []schema.Schema) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(schemas)
}

func writeSchemas(schemas []schema.Schema, path string) error {
	b, err := json.MarshalIndent(schemas, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, filePermissions)
}
