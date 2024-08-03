package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"

	"github.com/ltman/mondex/schema"
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

	filterSchemas(declared)
	for i, d := range declared {
		d.Indexes = filterIndexes(d.Indexes)
		declared[i] = d
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

// filterSchemas removes ignored collections from the schema list
func filterSchemas(schemas []schema.Schema) []schema.Schema {
	filtered := slices.Clone(schemas)
	return slices.DeleteFunc(filtered, func(s schema.Schema) bool {
		return slices.Contains(collectionsToIgnore, s.Collection)
	})
}

// filterIndexes removes non-modifiable indexes
func filterIndexes(indexes []schema.Index) []schema.Index {
	filtered := slices.Clone(indexes)
	return slices.DeleteFunc(filtered, func(i schema.Index) bool {
		return slices.Contains(indexesToIgnore, i.Name)
	})
}
