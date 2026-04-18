package localcache

import (
	"fmt"
	"time"

	"github.com/alert666/api-server/base/constant"

	"github.com/alert666/api-server/pkg/oauth"
	gocache "github.com/patrickmn/go-cache"
)

type Cacher interface {
	SetCache(key string, value any, expire time.Duration)
	UpdateCache(key string, value any, expire time.Duration) error
	GetCache(key string) (any, error)
}

type Cache struct {
	cache *gocache.Cache
}

func NewCacher(oauth *oauth.OAuth2) Cacher {
	oauth2ProviderList := make([]string, 0)
	if oauth != nil {
		for key := range oauth.Providers {
			oauth2ProviderList = append(oauth2ProviderList, key)
		}
	}
	c := gocache.New(5*time.Minute, 10*time.Minute)
	cache := &Cache{
		cache: c,
	}

	if oauth != nil {
		cache.SetCache(constant.OAuth2ProviderList, oauth2ProviderList, gocache.NoExpiration)
	}
	return cache
}

// SetCache 如果 key 不存在则创建，如果存在则覆盖。
// 如果 expire 等于 -1 (gocache.NoExpiration)，则该项永不过期。
func (receive *Cache) SetCache(key string, value any, expire time.Duration) {
	receive.cache.Set(key, value, expire)
}

// UpdateCache 仅当 key 已存在时才更新。如果 key 不存在，将返回错误。
func (receive *Cache) UpdateCache(key string, value any, expire time.Duration) error {
	err := receive.cache.Replace(key, value, expire)
	if err != nil {
		return fmt.Errorf("failed to update cache: key '%s' does not exist", key)
	}
	return nil
}

func (receive *Cache) GetCache(key string) (any, error) {
	item, found := receive.cache.Get(key)
	if !found {
		return nil, fmt.Errorf("cache not found: %s", key)
	}
	return item, nil
}
