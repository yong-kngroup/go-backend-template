//go:build integration

package database

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/freeDog-wy/go-backend-template/internal/testsupport"
	"github.com/golang-migrate/migrate/v4"
)

func TestMigrationsApplyInitialSchema(t *testing.T) {
	db := testsupport.OpenPostgres(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}

	migrator, err := NewMigratorWithDB(sqlDB, migrationDir(t))
	if err != nil {
		t.Fatalf("open migrator: %v", err)
	}
	t.Cleanup(func() {
		_, _ = migrator.Close()
	})

	if err := migrator.Up(); err != nil {
		t.Fatalf("apply up migrations: %v", err)
	}

	version, dirty, err := migrator.Version()
	if err != nil {
		t.Fatalf("migration version: %v", err)
	}
	if version != 6 || dirty {
		t.Fatalf("migration version = (%d, dirty=%t), want (6, false)", version, dirty)
	}

	for _, table := range initialTables {
		if !tableExists(t, sqlDB, table) {
			t.Fatalf("table %q was not created", table)
		}
	}

	if err := migrator.Down(); err != nil {
		t.Fatalf("apply down migrations: %v", err)
	}
	for _, table := range initialTables {
		if tableExists(t, sqlDB, table) {
			t.Fatalf("table %q still exists after down migration", table)
		}
	}

	if _, _, err := migrator.Version(); !errors.Is(err, migrate.ErrNilVersion) {
		t.Fatalf("migration version after down error = %v, want ErrNilVersion", err)
	}
}

var initialTables = []string{
	"users",
	"user_credentials",
	"roles",
	"permissions",
	"user_roles",
	"role_permissions",
	"outbox_events",
	"message_consumptions",
	"email_verification_tokens",
	"password_reset_tokens",
	"logs",
	"locales",
	"categories",
	"category_translations",
	"articles",
	"article_translations",
	"article_categories",
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var relation sql.NullString
	if err := db.QueryRow(`SELECT to_regclass($1)`, table).Scan(&relation); err != nil {
		t.Fatalf("find table %q: %v", table, err)
	}
	return relation.Valid
}

func migrationDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		candidate := filepath.Join(dir, "db", "migrations")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("locate db/migrations from the working directory")
		}
		dir = parent
	}
}
