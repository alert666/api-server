package constant

import apitypes "github.com/qinquanliuxiang666/alertmanager/base/types"

type userContextKey struct{}

type providerContextKey struct{}

type requestIDContextKey struct{}

var UserContextKey = userContextKey{}
var ProviderContextKey = providerContextKey{}
var RequestIDContextKey = requestIDContextKey{}

var ApiData apitypes.ServerApiData

const (
	FlagConfigPath          = "config-path"
	EmptyRoleSentinel       = "__empty__"
	OAuth2ProviderList      = "oauth2:provider:list"
	AlertStatusResolved     = "resolved"
	AlertStatusFiring       = "firing"
	AlertChannelTopicUpdate = "alert:channel:update"
	AlertChannelTopicDelete = "alert:channel:delete"
	AlertDBTenantKey        = "cluster"
	AlertCleanCacheLockKey  = "clean_duplicate_alerts"
)
