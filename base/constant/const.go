package constant

import apitypes "github.com/alert666/api-server/base/types"

type userContextKey struct{}
type providerContextKey struct{}
type requestIDContextKey struct{}
type tenantIDContextKey struct{}

var (
	UserContextKey      = userContextKey{}
	ProviderContextKey  = providerContextKey{}
	RequestIDContextKey = requestIDContextKey{}
	TenantIDContextKey  = tenantIDContextKey{}
	ApiData             apitypes.ServerApiData
)

const (
	FlagConfigPath                    = "config-path"
	EmptyRoleSentinel                 = "__empty__"
	OAuth2ProviderList                = "oauth2:provider:list"
	TenantIDHeader                    = "X-Tenant-Id"
	TenantOptionsCacheKey             = "options"
	AlertStatusResolved               = "resolved"
	AlertStatusFiring                 = "firing"
	AlertChannelTopicUpdate           = "alert:channel:update"
	AlertChannelTopicDelete           = "alert:channel:delete"
	AlertDBTenantKey                  = "cluster"
	AlertCleanDuplicateHistoryLockKey = "alert:clean:duplicate:history"
	AlertCleanExpiredSilencesLockKey  = "alert:clean:expired:silences"
)
