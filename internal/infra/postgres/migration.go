package postgres

import (
	"database/sql"
	"net/url"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// NewMigrator 以 migrationDir 中的 SQL 文件创建 PostgreSQL 迁移器。
func NewMigrator(dsn, migrationDir string) (*migrate.Migrate, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	m, err := NewMigratorWithDB(db, migrationDir)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return m, nil
}

// NewMigratorWithDB 基于已配置的 PostgreSQL 连接创建迁移器，适用于隔离 schema 测试。
func NewMigratorWithDB(db *sql.DB, migrationDir string) (*migrate.Migrate, error) {
	driver, err := pgmigrate.WithInstance(db, &pgmigrate.Config{})
	if err != nil {
		return nil, err
	}

	absoluteDir, err := filepath.Abs(migrationDir)
	if err != nil {
		return nil, err
	}
	sourceURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(absoluteDir)}).String()
	return migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
}
