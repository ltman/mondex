# Mondex: MongoDB Index Managing Tools

Mondex is a MongoDB index managing tool built on top of [golang-migrate](https://github.com/golang-migrate/migrate). It provides a simple and efficient way to manage MongoDB indexes through migration scripts.

## Features

- **Seamless Integration**: Integrates with `golang-migrate` to apply migrations to MongoDB.
- **Easy Setup**: Quickly set up and manage your MongoDB indexes.
- **Structured Logging**: Provides detailed and structured logging for better traceability.

## Installation

```sh
go install github.com/ltman/mondex@latest
```

## Usage

### Configuration

Create a mondex.yml configuration file in your project root:

```yaml
mongo_uri: "mongodb://localhost:27017"
database_name: "your_database"
schema_file_path: "path/to/schema/file"
migration_dir: "path/to/migrations"
log_level: "info"
```

### Commands

#### Apply Migrations

Apply current migrations to the database:

```sh
mondex apply
```

#### Generate Migration Scripts

Generate migration scripts based on schema differences:

```sh
mondex diff your_migration_name
```

#### Inspect Database Schema

Inspect and output the current database schema:

```sh
mondex inspect
```

#### Help

Identify how to use `mondex`

```sh
mondex help
```