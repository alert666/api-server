package v1

import (
	"github.com/alert666/api-server/store"
)

var (
	userStore      = store.User
	roleStore      = store.Role
	apiStore       = store.Api
	casbinStore    = store.CasbinRule
	oauth2Store    = store.Oauth2User
	tenantStore    = store.Tenant
	aHistoryStore  = store.AlertHistory
	aChannelStore  = store.AlertChannel
	aTemlpateStore = store.AlertTemplate
	aSendStore     = store.AlertSendRecord
	aSilenceStore  = store.AlertSilence
)

func NewStore() {
	userStore = store.User
	roleStore = store.Role
	apiStore = store.Api
	casbinStore = store.CasbinRule
	oauth2Store = store.Oauth2User
	tenantStore = store.Tenant
	aHistoryStore = store.AlertHistory
	aChannelStore = store.AlertChannel
	aTemlpateStore = store.AlertTemplate
	aSendStore = store.AlertSendRecord
	aSilenceStore = store.AlertSilence
}
