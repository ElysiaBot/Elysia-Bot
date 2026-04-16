package pluginsdk

import (
	"fmt"
	"strings"
)

type AuthorizationPolicy struct {
	Permissions []string `json:"permissions,omitempty" yaml:"permissions,omitempty"`
	PluginScope []string `json:"pluginScope,omitempty" yaml:"plugin_scope,omitempty"`
	EventScope  []string `json:"eventScope,omitempty" yaml:"event_scope,omitempty"`
}

type AuthorizationTargetKind string

const (
	AuthorizationTargetPlugin AuthorizationTargetKind = "plugin"
	AuthorizationTargetEvent  AuthorizationTargetKind = "event"
)

type AuthorizationDecision struct {
	Allowed    bool   `json:"allowed"`
	Permission string `json:"permission"`
	Reason     string `json:"reason,omitempty"`
}

type Authorizer struct {
	actorRoles map[string][]string
	policies   map[string]AuthorizationPolicy
}

func ValidatePermission(permission string) error {
	trimmed := strings.TrimSpace(permission)
	if trimmed == "" {
		return fmt.Errorf("permission is required")
	}
	if trimmed != permission {
		return fmt.Errorf("permission %q must not contain surrounding whitespace", permission)
	}
	resource, action, found := strings.Cut(trimmed, ":")
	if !found || strings.Contains(action, ":") {
		return fmt.Errorf("permission %q must use resource:action format", permission)
	}
	if strings.TrimSpace(resource) == "" || strings.TrimSpace(action) == "" {
		return fmt.Errorf("permission %q must use resource:action format", permission)
	}
	return nil
}

func (p AuthorizationPolicy) Validate() error {
	for _, permission := range p.Permissions {
		if err := ValidatePermission(permission); err != nil {
			return err
		}
	}
	return nil
}

func NewAuthorizer(actorRoles map[string][]string, policies map[string]AuthorizationPolicy) *Authorizer {
	bindings := make(map[string][]string, len(actorRoles))
	for actor, roles := range actorRoles {
		bindings[actor] = append([]string(nil), roles...)
	}
	policySet := make(map[string]AuthorizationPolicy, len(policies))
	for role, policy := range policies {
		policySet[role] = AuthorizationPolicy{
			Permissions: append([]string(nil), policy.Permissions...),
			PluginScope: append([]string(nil), policy.PluginScope...),
			EventScope:  append([]string(nil), policy.EventScope...),
		}
	}
	return &Authorizer{actorRoles: bindings, policies: policySet}
}

func (a *Authorizer) Authorize(actor, permission, target string) AuthorizationDecision {
	return a.AuthorizeTarget(actor, permission, AuthorizationTargetPlugin, target)
}

func (a *Authorizer) AuthorizeTarget(actor, permission string, kind AuthorizationTargetKind, target string) AuthorizationDecision {
	if permission == "" {
		return AuthorizationDecision{Allowed: false, Permission: permission, Reason: "permission is required"}
	}
	hasPermission := false
	for _, role := range a.actorRoles[actor] {
		policy := a.policies[role]
		if !policyHasPermission(policy, permission) {
			continue
		}
		hasPermission = true
		for _, allowedTarget := range policyTargets(policy, kind) {
			if allowedTarget == "*" || allowedTarget == target {
				return AuthorizationDecision{Allowed: true, Permission: permission}
			}
		}
	}
	if len(a.actorRoles[actor]) == 0 || !hasPermission {
		return AuthorizationDecision{Allowed: false, Permission: permission, Reason: "permission denied"}
	}
	return AuthorizationDecision{Allowed: false, Permission: permission, Reason: "plugin scope denied"}
}

func policyHasPermission(policy AuthorizationPolicy, permission string) bool {
	for _, candidate := range policy.Permissions {
		if candidate == permission {
			return true
		}
	}
	return false
}

func policyTargets(policy AuthorizationPolicy, kind AuthorizationTargetKind) []string {
	switch kind {
	case AuthorizationTargetEvent:
		if len(policy.EventScope) > 0 {
			return policy.EventScope
		}
		return policy.PluginScope
	default:
		return policy.PluginScope
	}
}
