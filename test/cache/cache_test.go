package cache_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alert666/api-server/base/conf"
	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/data"
	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/store"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	cacheStore  store.CacheStorer
	redisClient *redis.Client
	closeup     func()
)

func init() {
	var (
		err error
	)
	conf.LoadConfig("../../config.yaml")
	redisClient, err = data.NewRDB()
	if err != nil {
		panic(err)
	}
	log.NewLogger()
	cacheStore, closeup, err = store.NewCacheStore(redisClient)
	if err != nil {
		panic(err)
	}
}

func TestCacheStore(t *testing.T) {
	defer closeup()
	var (
		roles []string
		err   error
	)
	roleNames := []any{"test"}
	if err := cacheStore.SetSet(context.Background(), store.RoleType, "test", roleNames, nil); err != nil {
		t.Fatal(err)
	}
	if roles, err = cacheStore.GetSet(context.Background(), store.RoleType, "test"); err != nil {
		t.Fatal(err)
	}
	zap.L().Info("roles", zap.Any("roles", roles))
}

func TestSub(t *testing.T) {
	cacheStore.Subscribe(context.Background(), constant.AlertChannelTopicDelete, func(msg string) {
		fmt.Printf("删除事件, %s\n", msg)
	})
	cacheStore.Subscribe(context.Background(), constant.AlertChannelTopicUpdate, func(msg string) {
		fmt.Printf("更新事件, %s\n", msg)
	})

	cacheStore.Publish(context.Background(), constant.AlertChannelTopicDelete, "delete Channnel test")
	cacheStore.Publish(context.Background(), constant.AlertChannelTopicUpdate, "update Channne test")

	time.Sleep(10 * time.Second)
	cacheStore.Publish(context.Background(), constant.AlertChannelTopicDelete, "delete Channnel test1")
	time.Sleep(10 * time.Second)
}
