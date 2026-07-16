package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// NewPostgresDB 创建带追踪和连接池限制的 PostgreSQL 连接。
func NewPostgresDB(dsn string) (*gorm.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("postgres dsn is required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Use(newTracingPlugin()); err != nil {
		return nil, fmt.Errorf("initialize postgres tracing: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get postgres database handle: %w", err)
	}
	sqlDB.SetConnMaxIdleTime(time.Duration(5) * time.Minute)
	sqlDB.SetConnMaxLifetime(time.Duration(30) * time.Minute)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	return db, nil
}

// NewTxManager 创建通过 context 向 Repository 传递事务连接的事务管理器。
func NewTxManager(db *gorm.DB) *TxManager {
	return &TxManager{db: db}
}

type TxManager struct {
	db *gorm.DB
}

type txKey struct{}

// Do 在单个 PostgreSQL 事务中执行 fn，并将事务连接写入回调的 context。
//
// 回调不得继续使用外层 context 调用 Repository，否则该写入会逃离事务边界。
func (m *TxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// DB 返回 context 中的事务连接；没有事务时返回默认连接。
// Repository 必须通过此函数访问数据库，避免同一用例内的写入破坏原子性。
func DB(ctx context.Context, defaultDB *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return defaultDB
}
