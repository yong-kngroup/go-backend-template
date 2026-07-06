package main

import (
	"context"
	"encoding/json"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	domainUser "github.com/freeDog-wy/go-backend-template/internal/domain/user"
	"github.com/freeDog-wy/go-backend-template/internal/infra/cache"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	"github.com/freeDog-wy/go-backend-template/internal/infra/logging"
	"github.com/freeDog-wy/go-backend-template/internal/infra/mq"
	RepoUser "github.com/freeDog-wy/go-backend-template/internal/repository/user"
	SvcUser "github.com/freeDog-wy/go-backend-template/internal/service/user"
	"github.com/freeDog-wy/go-backend-template/pkg/email"
)

// Worker 事件消费者进程。
type Worker struct {
	consumer *mq.RedisConsumer
}

// Run 启动消费者，阻塞直到 ctx 取消。
func (w *Worker) Run(ctx context.Context) error {
	return w.consumer.Run(ctx)
}

func initWorker(cfg *config.Config) *Worker {
	// —————————— 基础设施 ——————————
	appLogger := logging.Init(cfg.App.Mode)

	rdb, err := cache.NewRedis(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		panic("failed to init redis: " + err.Error())
	}

	db := database.NewPostgresDB(cfg.Database.DSN)
	database.RunAutoMigrate(db, cfg.App.Mode)
	userRepo := RepoUser.New(db)

	emailSender := email.New(email.Config{
		SmtpHost:     cfg.Email.SmtpHost,
		SmtpPort:     cfg.Email.SmtpPort,
		SmtpUser:     cfg.Email.SmtpUser,
		SmtpPassword: cfg.Email.SmtpPassword,
		FromAddress:  cfg.Email.FromAddress,
	})

	// worker 不需要 tx/captcha/eventBus/pwdHasher（仅消费事件）
	userSvc := SvcUser.New(nil, userRepo, nil, nil, emailSender, appLogger, nil)

	// —————————— 事件消费 ——————————
	consumer := mq.NewRedisConsumer(rdb, "domain.events", "user-worker", "worker-1", appLogger)

	consumer.Handle("user.registered", func(ctx context.Context, data []byte) error {
		var evt domainUser.Registered
		if err := json.Unmarshal(data, &evt); err != nil {
			return err
		}
		return userSvc.OnUserRegistered(ctx, evt)
	})

	return &Worker{consumer: consumer}
}
