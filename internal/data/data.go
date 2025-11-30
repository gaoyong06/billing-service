package data

import (
	"fmt"
	"time"

	"billing-service/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/v8"
	"github.com/google/wire"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewDB,
	NewRedis,
	NewData,
	NewBillingRepo,
)

// Data 数据层结构体
type Data struct {
	db  *gorm.DB
	rdb *redis.Client
}

// NewDB 创建数据库连接
func NewDB(c *conf.Bootstrap) (*gorm.DB, error) {
	if c.Data == nil || c.Data.Database == nil {
		return nil, fmt.Errorf("database config is nil")
	}
	db, err := gorm.Open(mysql.Open(c.Data.Database.Source), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// NewRedis 创建 Redis 连接
func NewRedis(c *conf.Bootstrap) (*redis.Client, error) {
	if c.Data == nil || c.Data.Redis == nil {
		return nil, fmt.Errorf("redis config is nil")
	}

	var readTimeout, writeTimeout time.Duration
	if c.Data.Redis.ReadTimeout != nil {
		readTimeout = c.Data.Redis.ReadTimeout.AsDuration()
	}
	if c.Data.Redis.WriteTimeout != nil {
		writeTimeout = c.Data.Redis.WriteTimeout.AsDuration()
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         c.Data.Redis.Addr,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	})

	// 测试连接
	if err := rdb.Ping(rdb.Context()).Err(); err != nil {
		return nil, err
	}

	return rdb, nil
}

// NewData 创建数据层实例
func NewData(c *conf.Bootstrap, logger log.Logger, db *gorm.DB, rdb *redis.Client) (*Data, func(), error) {
	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
		if err := rdb.Close(); err != nil {
			log.NewHelper(logger).Errorf("failed to close redis: %v", err)
		}
	}

	return &Data{
		db:  db,
		rdb: rdb,
	}, cleanup, nil
}
