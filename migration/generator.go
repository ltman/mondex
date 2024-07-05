package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mongo-migrate/db"
	"mongo-migrate/schema"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const (
	filePermissions           = 0644
	timestampFormat           = "20060102150405"
	upMigrationFileTemplate   = "%s_%s.up.json"
	downMigrationFileTemplate = "%s_%s.up.json"
)

var (
	collectionsToIgnore = []string{"migrate_advisory_lock", "schema_migrations"}
	indexesToIgnore     = []string{"_id_"}
)

func GenerateMigrationScripts(
	ctx context.Context,
	logger *slog.Logger,
	mongoURI, databaseName string,
	schemaFilePath string,
	outputDir, migrationName string,
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

	logger.Debug("Filter current schemas by removing migration-related collections", "collections", collectionsToIgnore)
	current = filterSchemas(current)

	logger.Debug("Reading declared schema from file", "path", schemaFilePath)
	declared, err := readDeclaredSchema(schemaFilePath)
	if err != nil {
		return fmt.Errorf("failed to read declared schema: %w", err)
	}

	logger.Debug("Filter declared schemas by removing migration-related collections", "collections", collectionsToIgnore)
	declared = filterSchemas(declared)

	logger.Debug("Generating migration commands")
	upCommand, downCommand, err := generateMigrationCommands(current, declared, logger)
	if err != nil {
		return fmt.Errorf("failed to generate migration commands: %w", err)
	}

	if upCommand == nil && downCommand == nil {
		logger.Info("No changes detected, skipping migration generation")
		return nil
	}

	logger.Debug("Writing migration commands to files", "outputDir", outputDir)
	if dryRun {
		logger.Info("Dry run: showing changes without writing files")
		prettyPrintMigrationFiles(upCommand, downCommand)
	} else {
		if err := writeMigrationCommands(upCommand, downCommand, outputDir, migrationName); err != nil {
			return fmt.Errorf("failed to write migration commands: %w", err)
		}
		logger.Debug("Successfully wrote migration commands to files", "outputDir", outputDir)
	}

	return nil
}

// indexesDifference calculate index diff between i1 and i2
func indexesDifference(i1, i2 []schema.Index) []schema.Index {
	diff := make([]schema.Index, 0)
	for _, i := range i1 {
		if !slices.ContainsFunc(i2, func(si schema.Index) bool {
			return si.Name == i.Name
		}) {
			diff = append(diff, i)
		}
	}
	return diff
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

// readDeclaredSchema reads the declared schema from a file
func readDeclaredSchema(path string) ([]schema.Schema, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var schemas []schema.Schema

	if err := json.NewDecoder(f).Decode(&schemas); err != nil {
		return nil, err
	}

	if schemas == nil {
		schemas = make([]schema.Schema, 0)
	}

	return schemas, nil
}

// generateMigrationCommands generates up and down migration commands
func generateMigrationCommands(current, declared []schema.Schema, logger *slog.Logger) (upCommand, downCommand []byte, err error) {
	toCreate := make([]schema.Schema, 0)
	for _, ds := range declared {
		csIdx := slices.IndexFunc(current, func(cs schema.Schema) bool {
			return cs.Collection == ds.Collection
		})
		if csIdx < 0 {
			toCreate = append(toCreate, ds)
			logger.Debug("New collection to create", "collection", ds.Collection)
			continue
		}

		diff := indexesDifference(ds.Indexes, current[csIdx].Indexes)
		if len(diff) > 0 {
			toCreate = append(toCreate, schema.Schema{Collection: ds.Collection, Indexes: diff})
			logger.Debug("Indexes to create", "collection", ds.Collection, "indexCount", len(diff))
		}
	}

	toDrop := make([]schema.Schema, 0)
	for _, cs := range current {
		dsIdx := slices.IndexFunc(declared, func(ds schema.Schema) bool {
			return ds.Collection == cs.Collection
		})
		if dsIdx < 0 {
			toDrop = append(toDrop, cs)
			logger.Debug("Collection to drop", "collection", cs.Collection)
			continue
		}

		diff := indexesDifference(cs.Indexes, declared[dsIdx].Indexes)
		if len(diff) > 0 {
			toDrop = append(toDrop, schema.Schema{Collection: cs.Collection, Indexes: diff})
			logger.Debug("Indexes to drop", "collection", cs.Collection, "indexCount", len(diff))
		}
	}

	if len(toCreate) == 0 && len(toDrop) == 0 {
		return nil, nil, nil
	}

	upCommand, err = json.MarshalIndent(append(generateCreateIndexesCommands(toCreate), generateDestroyIndexCommands(toDrop)...), "", "  ")
	if err != nil {
		return nil, nil, err
	}

	downCommand, err = json.MarshalIndent(append(generateDestroyIndexCommands(toCreate), generateCreateIndexesCommands(toDrop)...), "", "  ")
	if err != nil {
		return nil, nil, err
	}

	return upCommand, downCommand, nil
}

// generateCreateIndexesCommands generates createIndexes MongoDB commands
func generateCreateIndexesCommands(schemas []schema.Schema) []map[string]interface{} {
	commands := make([]map[string]interface{}, 0, len(schemas))

	for _, s := range schemas {
		filtered := filterIndexes(s.Indexes)
		if len(filtered) > 0 {
			commands = append(commands, map[string]interface{}{
				"createIndexes": s.Collection,
				"indexes":       filtered,
			})
		}
	}

	return commands
}

// generateDestroyIndexCommands generates dropIndexes MongoDB commands
func generateDestroyIndexCommands(schemas []schema.Schema) []map[string]interface{} {
	commands := make([]map[string]interface{}, 0, len(schemas))

	for _, s := range schemas {
		filtered := filterIndexes(s.Indexes)
		if len(filtered) == 0 {
			continue
		}

		indexes := make([]string, 0, len(filtered))
		for _, index := range filtered {
			indexes = append(indexes, index.Name)
		}

		if len(indexes) > 0 {
			commands = append(commands, map[string]interface{}{
				"dropIndexes": s.Collection,
				"index":       indexes,
			})
		}
	}

	return commands
}

// prettyPrintMigrationFiles print migration commands to stdout
func prettyPrintMigrationFiles(upCommand, downCommand []byte) {
	fmt.Println("Up command:")
	fmt.Println(string(upCommand))
	fmt.Println("Down command:")
	fmt.Println(string(downCommand))
}

// writeMigrationCommands writes the migration commands to files
func writeMigrationCommands(upCommand, downCommand []byte, outputDir, migrationName string) error {
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	timestamp := time.Now().Format(timestampFormat)

	upCommandFilePath := filepath.Join(outputDir, fmt.Sprintf(upMigrationFileTemplate, timestamp, migrationName))
	if err := os.WriteFile(upCommandFilePath, upCommand, filePermissions); err != nil {
		return fmt.Errorf("failed to write up command: %w", err)
	}

	downCommandFilePath := filepath.Join(outputDir, fmt.Sprintf(downMigrationFileTemplate, timestamp, migrationName))
	if err := os.WriteFile(downCommandFilePath, downCommand, filePermissions); err != nil {
		return fmt.Errorf("failed to write down command: %w", err)
	}

	return nil
}