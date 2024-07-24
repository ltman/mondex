package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

func FormatSchemaFile(
	_ context.Context,
	logger *slog.Logger,
	schemaFilePath string,
	dryRun bool,
) error {
	declared, err := readDeclaredSchema(schemaFilePath)
	if err != nil {
		return fmt.Errorf("reading declared schema: %w", err)
	}

	schemas, err := json.MarshalIndent(declared, "", "  ")

	if dryRun {
		logger.Info("Dry-run: showing schema without writing file")

		fmt.Printf("Schema that would be written to %s:\n", schemaFilePath)
		if _, err := os.Stdout.Write(schemas); err != nil {
			return fmt.Errorf("writing declared schema: %w", err)
		}

		return nil
	}

	logger.Info("Writing current schema to file", "path", schemaFilePath)
	if err := os.WriteFile(schemaFilePath, schemas, 0644); err != nil {
		return fmt.Errorf("writing declared schema: %w", err)
	}

	return nil
}
