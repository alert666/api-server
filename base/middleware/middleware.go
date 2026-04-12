package middleware

import (
	apitypes "github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/pkg/casbin"
	"github.com/alert666/api-server/pkg/jwt"
	"github.com/alert666/api-server/store"
	"github.com/gin-gonic/gin"
)

type MiddlewareInterface interface {
	Auth() gin.HandlerFunc
	AuthZ() gin.HandlerFunc
	Session() gin.HandlerFunc
	TenantMiddleware() gin.HandlerFunc
}

type Middleware struct {
	jwtImpl   jwt.JwtInterface
	authZImpl casbin.AuthChecker
	cacheImpl store.CacheStorer
}

func NewMiddleware(jwtImpl jwt.JwtInterface, authZImpl casbin.AuthChecker, cacheImpl store.CacheStorer) *Middleware {
	return &Middleware{
		jwtImpl:   jwtImpl,
		authZImpl: authZImpl,
		cacheImpl: cacheImpl,
	}
}

func (m *Middleware) Abort(c *gin.Context, code int, err error) {
	c.JSON(code, apitypes.NewResponseWithOpts(code, apitypes.WithError(err.Error())))
	c.Error(err)
	c.Abort()
}
