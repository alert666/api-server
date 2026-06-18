package casbin

import (
	"fmt"

	"github.com/casbin/casbin/v2"
)

// AuthChecker 授权检查接口
type AuthChecker interface {
	Enforce(sub, obj, act string) (bool, error)
}

// CasbinManager 策略和角色管理接口
type CasbinManager interface {
	LoadPolicy() error
}

// casbinManager 实现结构体
type casbinManager struct {
	enforcer *casbin.Enforcer
}

// NewCasbinManager 创建 CasbinManager 实例
func NewCasbinManager(enforcer *casbin.Enforcer) CasbinManager {
	return &casbinManager{
		enforcer: enforcer,
	}
}

// NewAuthChecker 创建 AuthChecker 实例
func NewAuthChecker(enforcer *casbin.Enforcer) AuthChecker {
	return &casbinManager{ // casbinManager 结构体同时实现了 AuthChecker 和 CasbinManager 接口
		enforcer: enforcer,
	}
}

// Enforce 实现 AuthChecker 接口的授权检查方法
func (m *casbinManager) Enforce(sub, obj, act string) (bool, error) {
	ok, err := m.enforcer.Enforce(sub, obj, act)
	if err != nil {
		return false, fmt.Errorf("casbin enforce failed: %w", err)
	}
	return ok, nil
}

// LoadPolicy 加载策略
func (m *casbinManager) LoadPolicy() error {
	if err := m.enforcer.LoadPolicy(); err != nil {
		return fmt.Errorf("failed to load casbin policy: %w", err)
	}
	return nil
}
