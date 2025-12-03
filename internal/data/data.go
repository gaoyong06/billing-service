package data

import (
	"billing-service/internal/conf"
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"github.com/google/wire"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewData,
	NewRedSync,
	NewUserBalanceRepo,
	NewFreeQuotaRepo,
	NewBillingRecordRepo,
	NewRechargeOrderRepo,
	NewStatsRepo,
	NewBillingRepo,
	NewPaymentServiceClient,
)

// Data .
type Data struct {
	db  *gorm.DB
	rdb *redis.Client
}

// NewData .
func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	log := log.NewHelper(logger)

	// MySQL
	db, err := gorm.Open(mysql.Open(c.Database.Source), &gorm.Config{})
	if err != nil {
		return nil, nil, err
	}

	// 配置数据库连接池（优化高频调用场景）
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	// 根据高频调用场景优化连接池配置
	// MaxOpenConns: 最大打开连接数（建议根据实际负载调整，100 适合中等规模）
	sqlDB.SetMaxOpenConns(100)
	// MaxIdleConns: 最大空闲连接数（建议为 MaxOpenConns 的 20-30%）
	sqlDB.SetMaxIdleConns(20)
	// ConnMaxLifetime: 连接最大生存时间（防止长时间连接导致的问题）
	sqlDB.SetConnMaxLifetime(time.Hour)
	// ConnMaxIdleTime: 空闲连接最大生存时间（及时释放空闲连接）
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	// Redis（配置连接池以优化高频调用场景）
	rdb := redis.NewClient(&redis.Options{
		Addr:         c.Redis.Addr,
		ReadTimeout:  c.Redis.ReadTimeout.AsDuration(),
		WriteTimeout: c.Redis.WriteTimeout.AsDuration(),
		// 连接池配置（优化高频调用）
		PoolSize:     20,              // 连接池大小（建议根据实际负载调整，20 适合中等规模）
		MinIdleConns: 5,               // 最小空闲连接数（保持一定数量的连接以减少连接建立开销）
		MaxRetries:   3,               // 最大重试次数（网络抖动时自动重试）
		DialTimeout:  5 * time.Second, // 连接超时时间
	})

	// Ping Redis to check connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		return nil, nil, err
	}

	d := &Data{
		db:  db,
		rdb: rdb,
	}

	cleanup := func() {
		log.Info("closing the data resources")
		if sqlDB, err := d.db.DB(); err == nil {
			sqlDB.Close()
		}
		if err := d.rdb.Close(); err != nil {
			log.Error(err)
		}
	}
	return d, cleanup, nil
}

// NewRedSync 创建 Redis 分布式锁管理器
// 从 Data 中提取 Redis Client
func NewRedSync(data *Data) *redsync.Redsync {
	if data == nil || data.rdb == nil {
		return nil
	}
	pool := goredis.NewPool(data.rdb)
	return redsync.New(pool)
}
