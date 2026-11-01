package data

import (
	"fmt"

	"gin-kratos-gorm/internal/conf"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

const defaultRBACModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (r.sub == p.sub || g(r.sub, p.sub)) && (keyMatch2(r.obj, p.obj) || keyMatch(r.obj, p.obj)) && (r.act == p.act || p.act == "*")
`

// NewEnforcer builds a Casbin enforcer backed by the same database. It returns
// (nil, nil) when RBAC is disabled, which the HTTP layer treats as "skip
// Casbin" (JWT authentication still applies).
func NewEnforcer(cfg *conf.Bootstrap, db *gorm.DB) (*casbin.Enforcer, error) {
	if cfg == nil || cfg.Auth == nil || !cfg.Auth.CasbinEnabled {
		return nil, nil
	}

	var m model.Model
	var err error
	if cfg.Auth.CasbinModelPath != "" {
		m, err = model.NewModelFromFile(cfg.Auth.CasbinModelPath)
		if err != nil {
			// Fall back to the embedded model so a missing file cannot take the
			// service down; log via the returned error only for unexpected cases.
			m, err = model.NewModelFromString(defaultRBACModel)
			if err != nil {
				return nil, fmt.Errorf("load casbin model: %w", err)
			}
		}
	} else {
		m, err = model.NewModelFromString(defaultRBACModel)
		if err != nil {
			return nil, fmt.Errorf("load casbin model: %w", err)
		}
	}

	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, fmt.Errorf("casbin gorm adapter: %w", err)
	}
	enforcer, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("casbin enforcer: %w", err)
	}
	enforcer.EnableAutoSave(true)
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("load casbin policy: %w", err)
	}

	if cfg.Data != nil && cfg.Data.Database != nil && cfg.Data.Database.Seed {
		if err := seedPolicies(enforcer); err != nil {
			return nil, err
		}
	}
	return enforcer, nil
}

func seedPolicies(e *casbin.Enforcer) error {
	policies := [][]string{
		{"r:admin", "/api/users", "*"},
		{"r:admin", "/api/users/:id", "*"},
		{"r:admin", "/api/users/:id/role", "*"},
		{"r:admin", "/api/products", "*"},
		{"r:admin", "/api/products/:id", "*"},
		{"r:admin", "/api/products/:id/status", "*"},
		{"r:admin", "/api/products/:id/stock", "*"},
		{"r:admin", "/api/orders", "*"},
		{"r:admin", "/api/orders*", "*"},
		{"r:admin", "/api/orders/:orderSn", "*"},
		{"r:user", "/api/users/me", "GET"},
		{"r:user", "/api/products", "GET"},
		{"r:user", "/api/products/:id", "GET"},
		{"r:user", "/api/orders", "*"},
		{"r:user", "/api/orders/:orderSn", "*"},
		{"r:user", "/api/orders/:orderSn/pay", "*"},
		{"r:user", "/api/orders/:orderSn/cancel", "*"},
	}
	for _, p := range policies {
		if _, err := e.AddPolicy(p); err != nil {
			return fmt.Errorf("seed casbin policy %v: %w", p, err)
		}
	}
	// Keep role groups in sync for deployments that use the grouping API too.
	for _, g := range [][]string{{"u:1", "r:admin"}, {"u:2", "r:user"}} {
		if _, err := e.AddGroupingPolicy(g); err != nil {
			return fmt.Errorf("seed casbin grouping %v: %w", g, err)
		}
	}
	return nil
}
