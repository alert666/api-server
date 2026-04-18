package pkg

import (
	"github.com/alert666/api-server/pkg/alertinhibit"
	"github.com/alert666/api-server/pkg/casbin"
	"github.com/alert666/api-server/pkg/feishu"
	"github.com/alert666/api-server/pkg/jwt"
	localcache "github.com/alert666/api-server/pkg/local_cache"
	"github.com/alert666/api-server/pkg/oauth"
	"github.com/google/wire"
)

var PkgProviderSet = wire.NewSet(
	wire.Bind(new(jwt.JwtInterface), new(*jwt.GenerateToken)),
	jwt.NewGenerateToken,
	feishu.NewFeiShu,
	casbin.NewEnforcer,
	casbin.NewCasbinManager,
	casbin.NewAuthChecker,
	oauth.NewOAuth2,
	localcache.NewCacher,
	alertinhibit.NewMatchers,
)
