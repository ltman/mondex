package migration

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/ltman/mondex/db"
)

func ApplyMigrations(
	ctx context.Context,
	logger *slog.Logger,
	mongoURI, databaseName string,
	migrationDir string,
) error {
	logger.Debug("Connecting to MongoDB")
	client, err := db.ConnectToMongoDB(ctx, mongoURI)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	logger.Debug("Creating MongoDB golang-migrate driver")
	driver, err := mongodb.WithInstance(client, &mongodb.Config{DatabaseName: databaseName})
	if err != nil {
		return fmt.Errorf("failed to create golang-migrate driver: %w", err)
	}

	logger.Debug("Creating MongoDB golang-migrate migrator")
	migrator, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationDir),
		"mongodb",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer func() {
		if sourceErr, dbErr := migrator.Close(); sourceErr != nil || dbErr != nil {
			logger.Error("Failed to close migration instance", "source_error", sourceErr, "database_error", dbErr)
		}
	}()

	logger.Debug("Applying MongoDB migration files")
	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}
