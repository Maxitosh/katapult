package grpc

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// JWTValidator validates Kubernetes ServiceAccount JWT tokens.
type JWTValidator struct {
	// expectedServiceAccount is the ServiceAccount name agents must present.
	expectedServiceAccount string
	// keyFunc resolves the signing key for JWT verification.
	keyFunc jwt.Keyfunc
}

// NewJWTValidator creates a validator for agent JWT tokens.
func NewJWTValidator(expectedSA string, keyFunc jwt.Keyfunc) *JWTValidator {
	return &JWTValidator{
		expectedServiceAccount: expectedSA,
		keyFunc:                keyFunc,
	}
}

// Claims represents the JWT claims extracted from a Kubernetes ServiceAccount token.
type Claims struct {
	jwt.RegisteredClaims
	Kubernetes *KubernetesClaims `json:"kubernetes,omitempty"`
}

// KubernetesClaims holds the Kubernetes-specific JWT fields.
type KubernetesClaims struct {
	Namespace      string            `json:"namespace"`
	ServiceAccount *ServiceAccountID `json:"serviceaccount,omitempty"`
}

// ServiceAccountID identifies a Kubernetes ServiceAccount.
type ServiceAccountID struct {
	Name string `json:"name"`
	UID  string `json:"uid"`
}

// ValidateToken parses and validates a JWT token string.
// Returns the parsed claims or an error.
// @cpt-algo:cpt-katapult-algo-agent-system-validate-registration:p1
// @cpt-dod:cpt-katapult-dod-agent-system-auth:p1
func (v *JWTValidator) ValidateToken(tokenStr string) (*Claims, error) {
	// @cpt-begin:cpt-katapult-algo-agent-system-validate-registration:p1:inst-verify-jwt
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, v.keyFunc)
	// @cpt-end:cpt-katapult-algo-agent-system-validate-registration:p1:inst-verify-jwt
	// @cpt-begin:cpt-katapult-algo-agent-system-validate-registration:p1:inst-reject-jwt
	if err != nil {
		return nil, fmt.Errorf("invalid agent identity token: %w", err)
	}
	// @cpt-end:cpt-katapult-algo-agent-system-validate-registration:p1:inst-reject-jwt

	// @cpt-begin:cpt-katapult-algo-agent-system-validate-registration:p1:inst-extract-claims
	if v.expectedServiceAccount != "" && claims.Kubernetes != nil && claims.Kubernetes.ServiceAccount != nil {
		// @cpt-begin:cpt-katapult-algo-agent-system-validate-registration:p1:inst-reject-sa
		if claims.Kubernetes.ServiceAccount.Name != v.expectedServiceAccount {
			return nil, fmt.Errorf("unauthorized ServiceAccount: expected %q, got %q",
				v.expectedServiceAccount, claims.Kubernetes.ServiceAccount.Name)
		}
		// @cpt-end:cpt-katapult-algo-agent-system-validate-registration:p1:inst-reject-sa
	}
	// @cpt-end:cpt-katapult-algo-agent-system-validate-registration:p1:inst-extract-claims

	return claims, nil
}

type claimsKey struct{}

// ClaimsFromContext extracts JWT claims from a context (set by the auth interceptor).
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsKey{}).(*Claims)
	return c, ok
}

// UnaryAuthInterceptor returns a gRPC unary interceptor that validates JWT tokens
// from the "authorization" metadata header.
// @cpt-dod:cpt-katapult-dod-agent-system-auth:p1
func UnaryAuthInterceptor(validator *JWTValidator) ggrpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *ggrpc.UnaryServerInfo, handler ggrpc.UnaryHandler) (any, error) {
		token, err := extractToken(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}

		claims, err := validator.ValidateToken(token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}

		ctx = context.WithValue(ctx, claimsKey{}, claims)
		return handler(ctx, req)
	}
}

func extractToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", fmt.Errorf("missing metadata")
	}

	vals := md.Get("authorization")
	if len(vals) == 0 {
		return "", fmt.Errorf("missing authorization header")
	}

	token := vals[0]
	if t, ok := strings.CutPrefix(token, "Bearer "); ok {
		token = t
	}

	return token, nil
}
