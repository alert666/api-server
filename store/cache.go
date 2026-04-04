package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/qinquanliuxiang666/alertmanager/base/conf"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	NeverExpires time.Duration = 0
)

type CacheType string

const (
	RoleType  CacheType = "role"
	AlertType CacheType = "alert"
	LockType  CacheType = "lock"
)

type CacheStorer interface {
	DelKey(ctx context.Context, cacheType CacheType, cacheKey any) error
	CacheSeter
	CacheStringer
	CacheSuber
	CacheLocker
}

type CacheStore struct {
	client     *redis.Client
	expireTime time.Duration
	keyPrefix  string
}

func NewCacheStore(redisClient *redis.Client) (*CacheStore, func(), error) {
	expireTime, err := conf.GetRedisExpireTime()
	if err != nil {
		return nil, nil, err
	}
	closeup := func() {
		_ = redisClient.Close()
	}
	prefix, err := conf.GetRedisKeyPrefix()
	if err != nil {
		return nil, nil, err
	}
	return &CacheStore{
		client:     redisClient,
		expireTime: expireTime,
		keyPrefix:  prefix,
	}, closeup, nil
}

type CacheLocker interface {
	SetNX(ctx context.Context, cacheType CacheType, cacheKey any, value any, expiration time.Duration) (bool, error)
}

// SetNX 封装 Redis 的 SET IF NOT EXISTS 逻辑，用于分布式锁
// 返回值 bool: true 表示获取锁成功，false 表示锁已被占用
func (c *CacheStore) SetNX(ctx context.Context, cacheType CacheType, cacheKey any, value any, expiration time.Duration) (bool, error) {
	key, err := c.NormalizeCacheKey(cacheKey)
	if err != nil {
		return false, err
	}
	saveKey := c.buildCacheKey(cacheType, key)

	// 使用 Redis 的 SetNX 命令
	return c.client.SetNX(ctx, saveKey, value, expiration).Result()
}

type CacheSeter interface {
	GetSet(ctx context.Context, cacheType CacheType, cacheKey any) ([]string, error)
	SetSet(ctx context.Context, cacheType CacheType, cacheKey any, cacheValue []any, expireTime *time.Duration) error
}

func (c *CacheStore) GetSet(ctx context.Context, cacheType CacheType, cacheKey any) ([]string, error) {
	key, err := c.NormalizeCacheKey(cacheKey)
	if err != nil {
		return nil, err
	}

	saveKey := c.buildCacheKey(cacheType, key)
	result, err := c.client.SMembers(ctx, saveKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("get set error: %w", err)
	}
	return result, nil
}

func (c *CacheStore) SetSet(ctx context.Context, cacheType CacheType, cacheKey any, cacheValue []any, expireTime *time.Duration) error {
	if cacheValue == nil {
		return fmt.Errorf("cacheValue cannot be nil")
	}

	key, err := c.NormalizeCacheKey(cacheKey)
	if err != nil {
		return err
	}

	saveKey := c.buildCacheKey(cacheType, key)
	if expireTime != nil {
		// 使用事务确保SADD和EXPIRE的原子性
		pipe := c.client.TxPipeline()
		pipe.SAdd(ctx, saveKey, cacheValue...)
		pipe.Expire(ctx, saveKey, *expireTime)

		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("redis setSet error: %w", err)
		}
		return nil
	}

	if err := c.client.SAdd(ctx, saveKey, cacheValue...).Err(); err != nil {
		return fmt.Errorf("redis setSet error: %w", err)
	}
	return nil
}

type CacheStringer interface {
	SetObject(ctx context.Context, cacheType CacheType, cacheKey any, value any, expiration time.Duration) error
	GetObject(ctx context.Context, cacheType CacheType, cacheKey any, target any) (bool, error)
}

// SetObject 序列化对象并存入 Redis
func (c *CacheStore) SetObject(ctx context.Context, cacheType CacheType, cacheKey any, value any, expiration time.Duration) error {
	key, err := c.NormalizeCacheKey(cacheKey)
	if err != nil {
		return err
	}
	saveKey := c.buildCacheKey(cacheType, key)

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("setObject marshal object error: %w", err)
	}

	return c.client.Set(ctx, saveKey, data, expiration).Err()
}

// GetObject 获取并反序列化对象
func (c *CacheStore) GetObject(ctx context.Context, cacheType CacheType, cacheKey any, target any) (bool, error) {
	key, err := c.NormalizeCacheKey(cacheKey)
	if err != nil {
		return false, err
	}
	saveKey := c.buildCacheKey(cacheType, key)

	data, err := c.client.Get(ctx, saveKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, fmt.Errorf("get object error: %w", err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return false, fmt.Errorf("unmarshal object error: %w", err)
	}

	return true, nil
}

type CacheSuber interface {
	Publish(ctx context.Context, channel string, msg string) error
	Subscribe(ctx context.Context, channel string, handler func(msg string))
}

func (c *CacheStore) Publish(ctx context.Context, channel string, msg string) error {
	return c.client.Publish(ctx, channel, msg).Err()
}

func (c *CacheStore) Subscribe(ctx context.Context, channel string, handler func(msg string)) {
	pubsub := c.client.Subscribe(ctx, channel)
	// 1. 等待订阅成功（很重要）
	_, err := pubsub.Receive(ctx)
	if err != nil {
		zap.L().Error(fmt.Sprintf("订阅 %s topic 失败", channel), zap.Error(err))
	}

	// 2. 获取消息通道
	ch := pubsub.Channel()

	// 3. 启动消费协程
	go func(ch <-chan *redis.Message) {
		for {
			select {
			case msg := <-ch:
				if msg == nil {
					return
				}
				handler(msg.Payload)

			case <-ctx.Done():
				_ = pubsub.Close()
				return
			}
		}
	}(ch)
}

// NormalizeCacheKey 将常用类型的 cacheKey 转换为 string
// cacheKey 的值有可能是 int 等
func (c *CacheStore) NormalizeCacheKey(cacheKey any) (string, error) {
	switch v := cacheKey.(type) {
	case string:
		return v, nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	default:
		return "", fmt.Errorf("unsupported cacheKey type: %v", cacheKey)
	}
}

func (c *CacheStore) DelKey(ctx context.Context, cacheType CacheType, cacheKey any) error {
	key, err := c.NormalizeCacheKey(cacheKey)
	if err != nil {
		return err
	}
	delKey := c.buildCacheKey(cacheType, key)
	if err := c.client.Del(ctx, delKey).Err(); err != nil {
		return fmt.Errorf("redis delKey error: %w", err)
	}
	return nil
}

// 新增辅助方法用于构建缓存key，提高可读性和可测试性
func (c *CacheStore) buildCacheKey(cacheType CacheType, key string) string {
	var sb strings.Builder
	sb.Grow(len(c.keyPrefix) + 1 + len(cacheType) + 1 + len(key))
	sb.WriteString(c.keyPrefix)
	sb.WriteByte(':')
	sb.WriteString(string(cacheType))
	sb.WriteByte(':')
	sb.WriteString(key)
	return sb.String()
}
