package main

import (
	"fmt"
	"os"

	"github.com/alert666/api-server/cmd"
)

// @title           api-server — 告警管理 API
// @version         1.0
// @description     告警管理后端服务 API 文档，支持告警接收、静默、抑制、通知、多租户管理、RBAC 权限控制等功能。
// @host      0.0.0.0:8080
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	if err := cmd.NewCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
