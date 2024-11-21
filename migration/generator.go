package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ltman/mondex/db"
	"github.com/ltman/mondex/schema"
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
	migrationDir, migrationName string,
	dryRun bool,
) error {
	upCommand, downCommand, err := generateMigrationScripts(ctx, logger, mongoURI, databaseName, schemaFilePath)
	if err != nil {
		return fmt.Errorf("failed to generate migration scripts: %w", err)
	}

	if upCommand == nil && downCommand == nil {
		logger.Info("No changes detected, skipping migration generation")
		return nil
	}

	if dryRun {
		logger.Info("Dry-run: showing migrations without writing file")

		fmt.Println("Up migration:") //nolint:forbidigo
		if _, err := os.Stdout.Write(upCommand); err != nil {
			return fmt.Errorf("writing up migration to stdout: %w", err)
		}

		fmt.Println("\nDown migration:") //nolint:forbidigo
		if _, err := os.Stdout.Write(downCommand); err != nil {
			return fmt.Errorf("writing down migration to stdout: %w", err)
		}

		return nil
	}

	logger.Debug("Writing migration commands to files", "migrationDir", migrationDir)
	if err := writeMigrationCommands(upCommand, downCommand, migrationDir, migrationName); err != nil {
		return fmt.Errorf("failed to write migration commands: %w", err)
	}

	return nil
}

func generateMigrationScripts(
	ctx context.Context,
	logger *slog.Logger,
	mongoURI, databaseName string,
	schemaFilePath string,
) (upMigration, downMigration []byte, err error) {
	logger.Debug("Connecting to MongoDB")
	client, err := db.ConnectToMongoDB(ctx, mongoURI)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			logger.Error("Failed to disconnect from MongoDB", "error", err)
		}
	}()

	logger.Debug("Reading current schema from MongoDB")
	current, err := db.ReadCurrentSchema(ctx, client.Database(databaseName))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read current schema: %w", err)
	}

	logger.Debug("Filter current schemas by removing migration-related collections", "collections", collectionsToIgnore)
	current = prepareSchemas(current)

	logger.Debug("Reading declared schema from file", "path", schemaFilePath)
	declared, err := readDeclaredSchema(schemaFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read declared schema: %w", err)
	}

	logger.Debug("Filter declared schemas by removing migration-related collections", "collections", collectionsToIgnore)
	declared = prepareSchemas(declared)

	logger.Debug("Generating migration commands")
	upCommand, downCommand, err := generateMigrationCommands(current, declared, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate migration commands: %w", err)
	}

	return upCommand, downCommand, nil
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
		commands = append(commands, map[string]interface{}{
			"createIndexes": s.Collection,
			"indexes":       s.Indexes,
		})
	}

	return commands
}

// generateDestroyIndexCommands generates dropIndexes MongoDB commands
func generateDestroyIndexCommands(schemas []schema.Schema) []map[string]interface{} {
	commands := make([]map[string]interface{}, 0, len(schemas))

	for _, s := range schemas {
		indexes := make([]string, 0, len(s.Indexes))
		for _, index := range s.Indexes {
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

// writeMigrationCommands writes the migration commands to files
func writeMigrationCommands(upCommand, downCommand []byte, migrationDir, migrationName string) error {
	if err := os.MkdirAll(migrationDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	version, err := getNextVersion(migrationDir)
	if err != nil {
		return fmt.Errorf("failed to determine next version: %w", err)
	}

	upCommandFilePath := filepath.Join(migrationDir, fmt.Sprintf("%06d_%s.up.json", version, migrationName))
	if err := os.WriteFile(upCommandFilePath, upCommand, 0600); err != nil {
		return fmt.Errorf("failed to write up command: %w", err)
	}

	downCommandFilePath := filepath.Join(migrationDir, fmt.Sprintf("%06d_%s.down.json", version, migrationName))
	if err := os.WriteFile(downCommandFilePath, downCommand, 0600); err != nil {
		return fmt.Errorf("failed to write down command: %w", err)
	}

	return nil
}

// getNextVersion determines the next version number for a migration file.
func getNextVersion(migrationDir string) (uint64, error) {
	matches, err := filepath.Glob(filepath.Join(migrationDir, "*.json"))
	if err != nil {
		return 0, fmt.Errorf("failed to match migration files: %w", err)
	}

	if len(matches) == 0 {
		return 1, nil
	}

	var maxVersion uint64
	for _, match := range matches {
		filename := filepath.Base(match)
		parts := strings.SplitN(filename, "_", 2)
		if len(parts) < 2 {
			continue
		}

		version, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			log.Printf("Warning: malformed migration filename: %s", filename)
			continue
		}

		if version > maxVersion {
			maxVersion = version
		}
	}

	return maxVersion + 1, nil
}
