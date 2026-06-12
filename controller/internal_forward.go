package controller

import (
	"net/http"

	"github.com/alert666/api-server/base/bind"
	"github.com/alert666/api-server/base/conf"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// InternalForwardController handles cross-replica command forwarding.
type InternalForwardController struct {
	dataTunnelSvc v1.DataTunnelServicer
}

func NewInternalForwardController(dataTunnelSvc v1.DataTunnelServicer) *InternalForwardController {
	return &InternalForwardController{dataTunnelSvc: dataTunnelSvc}
}

// HandleForward receives a forwarded command and executes it locally.
// Uses bind.ResponseWithData with BindTypeShouldBind to auto-bind and auto-respond.
func (c *InternalForwardController) HandleForward(ctx *gin.Context) {
	bind.ResponseWithData(ctx, c.dataTunnelSvc.ExecuteCommandLocally, bind.BindTypeShouldBind)
}

// InternalAuthMiddleware validates the shared internal token.
func InternalAuthMiddleware() gin.HandlerFunc {
	token := conf.GetInternalToken()
	if token == "" {
		zap.L().Warn("internal.token is empty, internal endpoints are unprotected")
	}
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}
		authHeader := c.GetHeader("Authorization")
		if authHeader != "Bearer "+token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
