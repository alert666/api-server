package pkg

import (
	"github.com/google/wire"
	"github.com/alert666/api-server/pkg/casbin"
	"github.com/alert666/api-server/pkg/feishu"
	"github.com/alert666/api-server/pkg/jwt"
	localcache "github.com/alert666/api-server/pkg/local_cache"
	"github.com/alert666/api-server/pkg/oauth"
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
)
