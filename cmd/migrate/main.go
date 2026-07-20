package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	"github.com/freeDog-wy/go-backend-template/internal/infra/postgres"
	"github.com/golang-migrate/migrate/v4"
)

func main() {
	var configPath string
	var migrationDir string
	var direction string
	var steps int
	var forceVersion int
	var allowDestructive bool
	var showVersion bool

	flag.StringVar(&configPath, "config", "", "Path to the configuration file")
	flag.StringVar(&migrationDir, "migrations", "db/migrations", "Path to SQL migration files")
	flag.StringVar(&direction, "direction", "up", "Migration direction: up or down")
	flag.IntVar(&steps, "steps", 0, "Number of migrations to apply; 0 means all")
	flag.IntVar(&forceVersion, "force-version", -1, "Mark a migration version clean without executing SQL; requires -allow-destructive")
	flag.BoolVar(&allowDestructive, "allow-destructive", false, "Required when migrating down")
	flag.BoolVar(&showVersion, "version", false, "Print the current migration version")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}
	m, err := postgres.NewMigrator(cfg.Database.DSN, migrationDir)
	if err != nil {
		log.Fatalf("open migrator: %v", err)
	}
	defer func() {
		if sourceErr, databaseErr := m.Close(); sourceErr != nil || databaseErr != nil {
			log.Printf("close migrator: source=%v database=%v", sourceErr, databaseErr)
		}
	}()

	if showVersion {
		version, dirty, err := m.Version()
		if errors.Is(err, migrate.ErrNilVersion) {
			fmt.Println("version: none")
			return
		}
		if err != nil {
			log.Fatalf("read migration version: %v", err)
		}
		fmt.Printf("version: %d, dirty: %t\n", version, dirty)
		return
	}

	if direction == "down" && !allowDestructive {
		log.Fatal("down migrations require -allow-destructive")
	}
	if forceVersion >= 0 {
		if !allowDestructive {
			log.Fatal("forcing a migration version requires -allow-destructive")
		}
		if err := m.Force(forceVersion); err != nil {
			log.Fatalf("force migration version: %v", err)
		}
		fmt.Fprintf(os.Stdout, "migration version forced: %d\n", forceVersion)
		return
	}

	switch direction {
	case "up":
		if steps > 0 {
			err = m.Steps(steps)
		} else {
			err = m.Up()
		}
	case "down":
		if steps > 0 {
			err = m.Steps(-steps)
		} else {
			err = m.Down()
		}
	default:
		log.Fatalf("unsupported migration direction %q", direction)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Fprintln(os.Stdout, "database schema is already current")
		return
	}
	if err != nil {
		log.Fatalf("apply migrations: %v", err)
	}
	fmt.Fprintf(os.Stdout, "migrations applied: direction=%s steps=%d\n", direction, steps)
}
