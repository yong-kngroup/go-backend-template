package database

import (
	"context"
	"time"

	modelAudit "github.com/freeDog-wy/go-backend-template/internal/model/audit"
	modelAuth "github.com/freeDog-wy/go-backend-template/internal/model/auth"
	modelAuthorization "github.com/freeDog-wy/go-backend-template/internal/model/authorization"
	modelConsumption "github.com/freeDog-wy/go-backend-template/internal/model/consumption"
	modelIdentity "github.com/freeDog-wy/go-backend-template/internal/model/identity"
	modelOutbox "github.com/freeDog-wy/go-backend-template/internal/model/outbox"
	modelVerification "github.com/freeDog-wy/go-backend-template/internal/model/verification"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewPostgresDB(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database: " + err.Error())
	}
	if err := db.Use(newTracingPlugin()); err != nil {
		panic("failed to init gorm tracing: " + err.Error())
	}
	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to get database instance: " + err.Error())
	}
	sqlDB.SetConnMaxIdleTime(time.Duration(5) * time.Minute)
	sqlDB.SetConnMaxLifetime(time.Duration(30) * time.Minute)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	return db
}

func NewTxManager(db *gorm.DB) *TxManager {
	return &TxManager{db: db}
}

// RunAutoMigrate 仅在非生产环境执行 GORM AutoMigrate。
// 生产环境应使用 migration 工具管理表结构。
func RunAutoMigrate(db *gorm.DB, mode string) {
	if mode == "production" {
		return // 生产环境禁止自动建表
	}
	if err := db.AutoMigrate(
		&modelAudit.Log{},
		&modelIdentity.User{},
		&modelAuth.UserCredential{},
		&modelAuthorization.Role{},
		&modelAuthorization.Permission{},
		&modelAuthorization.UserRole{},
		&modelAuthorization.RolePermission{},
		&modelConsumption.Record{},
		&modelOutbox.Event{},
		&modelVerification.EmailVerificationToken{},
		&modelVerification.PasswordResetToken{},
	); err != nil {
		panic("auto migrate failed: " + err.Error())
	}
}

type TxManager struct {
	db *gorm.DB
}

type txKey struct{}

func (m *TxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

func DB(ctx context.Context, defaultDB *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return defaultDB
}
