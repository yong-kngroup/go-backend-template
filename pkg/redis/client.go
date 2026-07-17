package redis

import (
	"fmt"
	"strings"
	"time"

	redisv9 "github.com/redis/go-redis/v9"
)

// Open 创建带连接池限制和追踪钩子的 Redis 客户端。
func Open(addr, password string, db int) (*redisv9.Client, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("redis address is required")
	}
	if db < 0 {
		return nil, fmt.Errorf("redis database index must not be negative")
	}
	rdb := redisv9.NewClient(&redisv9.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     10,
		MinIdleConns: 3,
		MaxIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	rdb.AddHook(newTracingHook(addr, db))

	return rdb, nil
}
