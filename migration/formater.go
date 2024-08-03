package migration

import (
	"cmp"
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

	schemas, err := json.MarshalIndent(prepareSchemas(declared), "", "  ")

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

func prepareSchemas(schemas []schema.Schema) []schema.Schema {
	schemas = slices.DeleteFunc(slices.Clone(schemas), func(s schema.Schema) bool {
		return slices.Contains(collectionsToIgnore, s.Collection)
	})
	slices.SortFunc(schemas, func(a, b schema.Schema) int {
		return cmp.Compare(a.Collection, b.Collection)
	})
	for i, sc := range schemas {
		sc.Indexes = slices.DeleteFunc(sc.Indexes, func(i schema.Index) bool {
			return slices.Contains(indexesToIgnore, i.Name)
		})
		slices.SortFunc(sc.Indexes, func(a, b schema.Index) int {
			return cmp.Compare(a.Name, b.Name)
		})
		schemas[i] = sc
	}
	return schemas
}
