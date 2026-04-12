package middleware

import (
	"context"

	"github.com/alert666/api-server/base/constant"
	"github.com/gin-gonic/gin"
)

func (m *Middleware) TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader(constant.TenantIDHeader)
		if tenantID != "" {
			ctx := context.WithValue(c.Request.Context(), constant.TenantIDContextKey, tenantID)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}
}
