package data

import (
	"context"
	"fmt"

	"github.com/alert666/api-server/base/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func NewRDB() (*redis.Client, error) {
	ctx := context.TODO()
	switch config.GetRedisMode() {
	case "sentinel":
		return initSentinelRedis(ctx)
	case "single":
		return initSingleRedis(ctx)
	default:
		return nil, fmt.Errorf("redis.mode is not supported: %s", config.GetRedisMode())
	}
}

func initSingleRedis(ctx context.Context) (*redis.Client, error) {
	host, err := config.GetRedisHost()
	if err != nil {
		return nil, err
	}
	password, err := config.GetRedisPassword()
	if err != nil {
		return nil, err
	}

	user := config.GetRedisUser()

	opts := &redis.Options{
		Addr:            host,
		Password:        password,
		DB:              config.GetRedisDB(),
		PoolSize:        config.GetRedisPoolSize(),
		MinIdleConns:    config.GetRedisMinIdleConns(),
		ConnMaxLifetime: config.GetRedisConnMaxLifetime(),
	}

	if user != "" {
		opts.Username = user
	}

	rdb := redis.NewClient(opts)
	err = rdb.Ping(ctx).Err()
	if err != nil {
		return nil, fmt.Errorf("redis connect failed: %w", err)
	}
	zap.L().Info("redis connect success")
	return rdb, nil
}

func initSentinelRedis(ctx context.Context) (*redis.Client, error) {
	sentinelHosts, err := config.GetRedisSentinelHosts()
	if err != nil {
		return nil, err
	}
	masterName, err := config.GetRedisMasterName()
	if err != nil {
		return nil, err
	}
	password, err := config.GetRedisPassword()
	if err != nil {
		return nil, err
	}
	sentPassword, err := config.GetRedisSentinelPassword()
	if err != nil {
		return nil, err
	}
	user := config.GetRedisUser()
	opts := &redis.FailoverOptions{
		MasterName:       masterName,
		SentinelAddrs:    sentinelHosts,
		Password:         password,
		SentinelPassword: sentPassword,
		RouteByLatency:   true,
		DB:               config.GetRedisDB(),
		PoolSize:         config.GetRedisPoolSize(),        // 最多50个连接
		MinIdleConns:     config.GetRedisMinIdleConns(),    // 最少20个空闲连接
		ConnMaxLifetime:  config.GetRedisConnMaxLifetime(), // 强制重连以避免连接老化
	}
	if user != "" {
		opts.Username = user
	}

	rdb := redis.NewFailoverClient(opts)
	err = rdb.Ping(ctx).Err()
	if err != nil {
		return nil, fmt.Errorf("redis sentinel connect failed: %w", err)
	}
	zap.L().Info("redis sentinel connect success")
	return rdb, nil
}
