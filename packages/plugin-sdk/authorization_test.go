package pluginsdk

import "testing"

func TestValidatePermissionAcceptsResourceActionFormat(t *testing.T) {
	t.Parallel()

	for _, permission := range []string{"reply:send", "plugin:rollout", "schedule:manage"} {
		if err := ValidatePermission(permission); err != nil {
			t.Fatalf("expected permission %q to validate, got %v", permission, err)
		}
	}
}

func TestValidatePermissionRejectsInvalidFormat(t *testing.T) {
	t.Parallel()

	for _, permission := range []string{"", "reply-send", "reply:", ":send", "reply:send:now", " reply:send"} {
		if err := ValidatePermission(permission); err == nil {
			t.Fatalf("expected permission %q to fail validation", permission)
		}
	}
}

func TestAuthorizationPolicyValidateRejectsInvalidPermissionFormat(t *testing.T) {
	t.Parallel()

	policy := AuthorizationPolicy{Permissions: []string{"plugin-enable"}, PluginScope: []string{"*"}}
	if err := policy.Validate(); err == nil {
		t.Fatal("expected invalid policy permission format to fail validation")
	}
}

func TestAuthorizerAllowsRoleWithPermissionAndEventScope(t *testing.T) {
	t.Parallel()

	authorizer := NewAuthorizer(
		map[string][]string{"replay-user": {"replay-role"}},
		map[string]AuthorizationPolicy{"replay-role": {Permissions: []string{"plugin:replay"}, EventScope: []string{"evt-allowed"}}},
	)
	decision := authorizer.AuthorizeTarget("replay-user", "plugin:replay", AuthorizationTargetEvent, "evt-allowed")
	if !decision.Allowed || decision.Permission != "plugin:replay" || decision.Reason != "" {
		t.Fatalf("unexpected event-scope authorization decision %+v", decision)
	}
}

func TestAuthorizerFallsBackToPluginScopeForEventTargetCompatibility(t *testing.T) {
	t.Parallel()

	authorizer := NewAuthorizer(
		map[string][]string{"replay-user": {"replay-role"}},
		map[string]AuthorizationPolicy{"replay-role": {Permissions: []string{"plugin:replay"}, PluginScope: []string{"evt-allowed"}}},
	)
	decision := authorizer.AuthorizeTarget("replay-user", "plugin:replay", AuthorizationTargetEvent, "evt-allowed")
	if !decision.Allowed {
		t.Fatalf("expected event target to fall back to plugin scope for compatibility, got %+v", decision)
	}
}

func TestAuthorizerAllowsRoleWithPermissionAndScope(t *testing.T) {
	t.Parallel()

	authorizer := NewAuthorizer(
		map[string][]string{"admin-user": {"admin"}},
		map[string]AuthorizationPolicy{"admin": {Permissions: []string{"plugin:enable"}, PluginScope: []string{"*"}}},
	)
	decision := authorizer.Authorize("admin-user", "plugin:enable", "plugin-echo")
	if !decision.Allowed || decision.Permission != "plugin:enable" || decision.Reason != "" {
		t.Fatalf("unexpected authorization decision %+v", decision)
	}
}

func TestAuthorizerRejectsWhenPermissionAndScopeOnlyExistAcrossDifferentRoles(t *testing.T) {
	t.Parallel()

	authorizer := NewAuthorizer(
		map[string][]string{"split-user": {"enable-any", "scope-only"}},
		map[string]AuthorizationPolicy{
			"enable-any": {Permissions: []string{"plugin:enable"}, PluginScope: []string{}},
			"scope-only": {Permissions: []string{}, PluginScope: []string{"plugin-echo"}},
		},
	)
	decision := authorizer.Authorize("split-user", "plugin:enable", "plugin-echo")
	if decision.Allowed || decision.Reason != "plugin scope denied" {
		t.Fatalf("expected split-role authorization to be denied, got %+v", decision)
	}
}

func TestAuthorizerRejectsUnknownActor(t *testing.T) {
	t.Parallel()

	authorizer := NewAuthorizer(nil, nil)
	decision := authorizer.Authorize("unknown", "plugin:enable", "plugin-echo")
	if decision.Allowed || decision.Reason != "permission denied" {
		t.Fatalf("expected unknown actor to be denied, got %+v", decision)
	}
}

func TestAuthorizerRejectsKnownActorWithoutRequiredPermission(t *testing.T) {
	t.Parallel()

	authorizer := NewAuthorizer(
		map[string][]string{"viewer-user": {"viewer"}},
		map[string]AuthorizationPolicy{"viewer": {Permissions: []string{"plugin:disable"}, PluginScope: []string{"*"}}},
	)
	decision := authorizer.Authorize("viewer-user", "plugin:enable", "plugin-echo")
	if decision.Allowed || decision.Reason != "permission denied" {
		t.Fatalf("expected wrong-permission actor to be denied, got %+v", decision)
	}
}
