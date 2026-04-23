package runtimecore

import (
	"context"
	"testing"

	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
)

func TestOperatorBearerIdentityResolverResolvesConfiguredTokenToRequestIdentityContext(t *testing.T) {
	t.Setenv("BOT_PLATFORM_OPERATOR_TOKEN", "opaque-operator-token")
	registry := NewSecretRegistry(EnvSecretProvider{}, NewInMemoryAuditLog())
	resolver, err := NewOperatorBearerIdentityResolver(context.Background(), registry, &OperatorAuthConfig{Tokens: []OperatorAuthTokenConfig{{
		ID:       "console-main",
		ActorID:  "admin-user",
		TokenRef: "BOT_PLATFORM_OPERATOR_TOKEN",
	}}})
	if err != nil {
		t.Fatalf("build resolver: %v", err)
	}

	identity, ok := resolver.ResolveAuthorizationHeader("Bearer opaque-operator-token")
	if !ok {
		t.Fatal("expected authorization header to resolve")
	}
	if identity.ActorID != "admin-user" || identity.TokenID != "console-main" || identity.AuthMethod != RequestIdentityAuthMethodBearer || identity.SessionID != "session-operator-bearer-admin-user" {
		t.Fatalf("unexpected request identity %+v", identity)
	}
}

func TestOperatorBearerIdentityResolverFailsClosedWhenSecretIsMissing(t *testing.T) {
	t.Parallel()

	registry := NewSecretRegistry(EnvSecretProvider{}, NewInMemoryAuditLog())
	_, err := NewOperatorBearerIdentityResolver(context.Background(), registry, &OperatorAuthConfig{Tokens: []OperatorAuthTokenConfig{{
		ID:       "console-main",
		ActorID:  "admin-user",
		TokenRef: "BOT_PLATFORM_OPERATOR_TOKEN_MISSING",
	}}})
	if err == nil || err.Error() != `resolve operator_auth.tokens[0].token_ref for token "console-main": secret "BOT_PLATFORM_OPERATOR_TOKEN_MISSING" not found in environment` {
		t.Fatalf("expected missing secret failure, got %v", err)
	}
}

func TestOperatorBearerIdentityResolverRejectsDuplicateResolvedOpaqueTokens(t *testing.T) {
	t.Setenv("BOT_PLATFORM_OPERATOR_TOKEN_A", "same-token")
	t.Setenv("BOT_PLATFORM_OPERATOR_TOKEN_B", "same-token")
	registry := NewSecretRegistry(EnvSecretProvider{}, NewInMemoryAuditLog())
	_, err := NewOperatorBearerIdentityResolver(context.Background(), registry, &OperatorAuthConfig{Tokens: []OperatorAuthTokenConfig{
		{ID: "console-a", ActorID: "admin-user", TokenRef: "BOT_PLATFORM_OPERATOR_TOKEN_A"},
		{ID: "console-b", ActorID: "admin-user", TokenRef: "BOT_PLATFORM_OPERATOR_TOKEN_B"},
	}})
	if err == nil || err.Error() != `operator_auth.tokens[1] bearer token duplicates configured token "console-a"` {
		t.Fatalf("expected duplicate opaque token rejection, got %v", err)
	}
}

func TestBearerTokenFromAuthorizationHeaderRejectsWhitespacePayloads(t *testing.T) {
	t.Parallel()

	for _, header := range []string{"", "Basic abc", "Bearer ", "Bearer has whitespace", "Bearer token\nnext"} {
		if token, ok := BearerTokenFromAuthorizationHeader(header); ok || token != "" {
			t.Fatalf("expected header %q to be rejected, got token=%q ok=%v", header, token, ok)
		}
	}
}

func TestRequestIdentityContextRoundTripsThroughContext(t *testing.T) {
	t.Parallel()

	ctx := WithRequestIdentityContext(context.Background(), RequestIdentityContext{
		ActorID:    " admin-user ",
		TokenID:    " console-main ",
		AuthMethod: " Bearer ",
		SessionID:  " session-operator-bearer-admin-user ",
	})
	identity := RequestIdentityContextFromContext(ctx)
	if identity.ActorID != "admin-user" || identity.TokenID != "console-main" || identity.AuthMethod != "bearer" || identity.SessionID != "session-operator-bearer-admin-user" {
		t.Fatalf("unexpected context identity %+v", identity)
	}
	if empty := RequestIdentityContextFromContext(context.Background()); empty != (RequestIdentityContext{}) {
		t.Fatalf("expected empty context identity from bare context, got %+v", empty)
	}
}

func TestBindRequestIdentitySessionPersistsBearerIdentityIntoGenericSessionStore(t *testing.T) {
	t.Parallel()

	store := openTempSQLiteStore(t)
	defer func() { _ = store.Close() }()

	ctx := WithRequestIdentityContext(context.Background(), RequestIdentityContext{
		ActorID:    "admin-user",
		TokenID:    "console-main",
		AuthMethod: RequestIdentityAuthMethodBearer,
		SessionID:  OperatorBearerSessionID("admin-user"),
	})
	if err := BindRequestIdentitySession(ctx, store); err != nil {
		t.Fatalf("bind request identity session: %v", err)
	}

	stored, err := store.LoadSession(context.Background(), OperatorBearerSessionID("admin-user"))
	if err != nil {
		t.Fatalf("load bound session: %v", err)
	}
	if stored.SessionID != OperatorBearerSessionID("admin-user") || stored.PluginID != OperatorAuthSessionPluginID {
		t.Fatalf("unexpected bound session identity %+v", stored)
	}
	if stored.State["actor_id"] != "admin-user" || stored.State["token_id"] != "console-main" || stored.State["auth_method"] != RequestIdentityAuthMethodBearer {
		t.Fatalf("unexpected bound session state %+v", stored.State)
	}

	listed, err := store.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(listed) != 1 || listed[0].SessionID != stored.SessionID {
		t.Fatalf("expected one bound session in list, got %+v", listed)
	}
}

func TestApplyAuditRequestIdentityCopiesSessionIDAndMissingActor(t *testing.T) {
	t.Parallel()

	ctx := WithRequestIdentityContext(context.Background(), RequestIdentityContext{
		ActorID:    "admin-user",
		TokenID:    "console-main",
		AuthMethod: RequestIdentityAuthMethodBearer,
		SessionID:  OperatorBearerSessionID("admin-user"),
	})
	entry := pluginsdk.AuditEntry{Action: "console.read", Target: "console", Allowed: false, OccurredAt: "2026-04-23T10:00:00Z"}
	ApplyAuditRequestIdentity(&entry, ctx)
	if entry.Actor != "admin-user" || entry.SessionID != OperatorBearerSessionID("admin-user") {
		t.Fatalf("expected request identity to populate audit entry, got %+v", entry)
	}

	preserved := pluginsdk.AuditEntry{Actor: "viewer-user", SessionID: "session-custom", Action: "console.read", Target: "console", Allowed: false, OccurredAt: "2026-04-23T10:00:00Z"}
	ApplyAuditRequestIdentity(&preserved, ctx)
	if preserved.Actor != "viewer-user" || preserved.SessionID != "session-custom" {
		t.Fatalf("expected explicit audit identity fields to be preserved, got %+v", preserved)
	}
}
