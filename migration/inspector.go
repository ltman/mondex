package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"bitbucket.org/ltman/mondex/db"
)

func InspectCurrentSchema(
	ctx context.Context,
	logger *slog.Logger,
	mongoURI, databaseName string,
	schemaFilePath string,
	dryRun bool,
) error {
	schemas, err := inspectCurrentSchema(ctx, logger, mongoURI, databaseName)
	if err != nil {
		return fmt.Errorf("inspecting current schema: %w", err)
	}

	if dryRun {
		logger.Info("Dry-run: showing schema without writing file")

		fmt.Printf("Schema that would be written to %s:\n", schemaFilePath)
		if _, err := os.Stdout.Write(schemas); err != nil {
			return fmt.Errorf("writing current schema: %w", err)
		}

		return nil
	}

	logger.Info("Writing current schema to file", "path", schemaFilePath)
	if err := os.WriteFile(schemaFilePath, schemas, filePermissions); err != nil {
		return fmt.Errorf("writing current schema: %w", err)
	}

	return nil
}

func inspectCurrentSchema(
	ctx context.Context,
	logger *slog.Logger,
	mongoURI, databaseName string,
) ([]byte, error) {
	logger.Debug("Connecting to MongoDB")
	client, err := db.ConnectToMongoDB(ctx, mongoURI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			logger.Error("Failed to disconnect from MongoDB", "error", err)
		}
	}()

	logger.Debug("Reading current schema from MongoDB")
	current, err := db.ReadCurrentSchema(ctx, client.Database(databaseName))
	if err != nil {
		return nil, fmt.Errorf("failed to read current schema: %w", err)
	}

	return json.MarshalIndent(current, "", "  ")
}
