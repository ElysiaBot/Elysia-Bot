package runtimecore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
)

const RequestIdentityAuthMethodBearer = "bearer"

const OperatorAuthSessionPluginID = "operator-auth"

var ErrRequestUnauthorized = errors.New("unauthorized")

type RequestIdentityContext struct {
	ActorID    string `json:"actor_id"`
	TokenID    string `json:"token_id,omitempty"`
	AuthMethod string `json:"auth_method"`
	SessionID  string `json:"session_id"`
}

type OperatorBearerIdentityResolver struct {
	identitiesByToken map[string]RequestIdentityContext
}

type requestIdentityContextKey struct{}

func OperatorAuthConfigured(cfg *OperatorAuthConfig) bool {
	return cfg != nil && len(cfg.Tokens) > 0
}

func RequestActorID(ctx context.Context, operatorAuthConfigured bool, legacyActor string) (string, error) {
	if operatorAuthConfigured {
		identity := RequestIdentityContextFromContext(ctx)
		if identity.AuthMethod != RequestIdentityAuthMethodBearer || strings.TrimSpace(identity.ActorID) == "" {
			return "", ErrRequestUnauthorized
		}
		return identity.ActorID, nil
	}
	return strings.TrimSpace(legacyActor), nil
}

func WithRequestIdentityContext(ctx context.Context, identity RequestIdentityContext) context.Context {
	identity = identity.normalized()
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, requestIdentityContextKey{}, identity)
}

func RequestIdentityContextFromContext(ctx context.Context) RequestIdentityContext {
	if ctx == nil {
		return RequestIdentityContext{}
	}
	stored, _ := ctx.Value(requestIdentityContextKey{}).(RequestIdentityContext)
	return stored.normalized()
}

func NewOperatorBearerIdentityResolver(ctx context.Context, registry *SecretRegistry, cfg *OperatorAuthConfig) (*OperatorBearerIdentityResolver, error) {
	resolver := &OperatorBearerIdentityResolver{identitiesByToken: map[string]RequestIdentityContext{}}
	if cfg == nil || len(cfg.Tokens) == 0 {
		return resolver, nil
	}
	if registry == nil {
		return nil, fmt.Errorf("secret registry is required when operator_auth.tokens is configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for index, token := range cfg.Tokens {
		secretValue, err := registry.Resolve(ctx, token.TokenRef, operatorAuthTokenSecretConsumer)
		if err != nil {
			return nil, fmt.Errorf("resolve operator_auth.tokens[%d].token_ref for token %q: %w", index, token.ID, err)
		}
		if err := validateOpaqueBearerToken(secretValue); err != nil {
			return nil, fmt.Errorf("invalid operator_auth.tokens[%d].token_ref for token %q: %w", index, token.ID, err)
		}
		if existing, exists := resolver.identitiesByToken[secretValue]; exists {
			return nil, fmt.Errorf("operator_auth.tokens[%d] bearer token duplicates configured token %q", index, existing.TokenID)
		}
		resolver.identitiesByToken[secretValue] = RequestIdentityContext{
			ActorID:    token.ActorID,
			TokenID:    token.ID,
			AuthMethod: RequestIdentityAuthMethodBearer,
			SessionID:  OperatorBearerSessionID(token.ActorID),
		}.normalized()
	}
	return resolver, nil
}

func (r *OperatorBearerIdentityResolver) ResolveAuthorizationHeader(header string) (RequestIdentityContext, bool) {
	if r == nil {
		return RequestIdentityContext{}, false
	}
	bearerToken, ok := BearerTokenFromAuthorizationHeader(header)
	if !ok {
		return RequestIdentityContext{}, false
	}
	return r.ResolveBearerToken(bearerToken)
}

func (r *OperatorBearerIdentityResolver) ResolveBearerToken(token string) (RequestIdentityContext, bool) {
	if r == nil {
		return RequestIdentityContext{}, false
	}
	identity, ok := r.identitiesByToken[token]
	if !ok {
		return RequestIdentityContext{}, false
	}
	return identity.normalized(), true
}

func BearerTokenFromAuthorizationHeader(header string) (string, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", false
	}
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, RequestIdentityAuthMethodBearer) {
		return "", false
	}
	token = strings.TrimSpace(token)
	if err := validateOpaqueBearerToken(token); err != nil {
		return "", false
	}
	return token, true
}

func OperatorBearerSessionID(actorID string) string {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return ""
	}
	return "session-operator-bearer-" + actorID
}

func validateOpaqueBearerToken(token string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("opaque bearer token is required")
	}
	if token != strings.TrimSpace(token) {
		return fmt.Errorf("opaque bearer token must not contain leading or trailing whitespace")
	}
	if strings.ContainsAny(token, " \t\r\n") {
		return fmt.Errorf("opaque bearer token must not contain whitespace")
	}
	return nil
}

func (c RequestIdentityContext) normalized() RequestIdentityContext {
	c.ActorID = strings.TrimSpace(c.ActorID)
	c.TokenID = strings.TrimSpace(c.TokenID)
	c.AuthMethod = strings.ToLower(strings.TrimSpace(c.AuthMethod))
	c.SessionID = strings.TrimSpace(c.SessionID)
	return c
}

func BindRequestIdentitySession(ctx context.Context, store SessionStateStore) error {
	if store == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	identity := RequestIdentityContextFromContext(ctx)
	if identity.AuthMethod != RequestIdentityAuthMethodBearer || identity.SessionID == "" {
		return nil
	}
	state := map[string]any{
		"actor_id":    identity.ActorID,
		"token_id":    identity.TokenID,
		"auth_method": identity.AuthMethod,
	}
	return store.SaveSession(ctx, SessionState{
		SessionID: identity.SessionID,
		PluginID:  OperatorAuthSessionPluginID,
		State:     state,
	})
}

func ApplyAuditRequestIdentity(entry *pluginsdk.AuditEntry, ctx context.Context) {
	if entry == nil {
		return
	}
	identity := RequestIdentityContextFromContext(ctx)
	if strings.TrimSpace(entry.Actor) == "" {
		entry.Actor = identity.ActorID
	}
	if strings.TrimSpace(entry.SessionID) == "" {
		entry.SessionID = identity.SessionID
	}
}
