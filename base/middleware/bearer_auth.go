package middleware

import (
	"net/http"

	"github.com/alert666/api-server/base/conf"
	"github.com/alert666/api-server/base/constant"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// bearerAuth returns a gin.HandlerFunc that validates a Bearer token.
// tokenFunc provides the expected token value.
// configKey is used in log messages when the token is empty.
// If the token is empty, the middleware is a no-op (with a startup warning).
func (m *Middleware) bearerAuth(tokenFunc func() string, configKey string) gin.HandlerFunc {
	token := tokenFunc()
	if token == "" {
		zap.L().Warn("token is empty, endpoint is unprotected", zap.String("config_key", configKey))
	}
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}
		authHeader := c.GetHeader("Authorization")
		if authHeader != "Bearer "+token {
			m.Abort(c, http.StatusUnauthorized, constant.ErrAuthFailed)
			return
		}
		c.Next()
	}
}

// AlertReceiveAuth validates the alert receive token for Alertmanager webhook callbacks.
func (m *Middleware) AlertReceiveAuth() gin.HandlerFunc {
	return m.bearerAuth(conf.GetAlertReceiveToken, "alert.receiveToken")
}

// InternalAuth validates the shared internal token for cross-replica forwarding.
func (m *Middleware) InternalAuth() gin.HandlerFunc {
	return m.bearerAuth(conf.GetInternalToken, "internal.token")
}
