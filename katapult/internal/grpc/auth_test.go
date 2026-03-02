package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var testSigningKey = []byte("test-secret-key-for-hmac-signing")

func testKeyFunc(_ *jwt.Token) (any, error) {
	return testSigningKey, nil
}

func signTestToken(t *testing.T, claims jwt.Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString(testSigningKey)
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}
	return s
}

func validClaims(sa string) *Claims {
	return &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Kubernetes: &KubernetesClaims{
			Namespace: "katapult",
			ServiceAccount: &ServiceAccountID{
				Name: sa,
				UID:  "test-uid",
			},
		},
	}
}

func TestValidateToken(t *testing.T) {
	tests := []struct {
		name       string
		expectedSA string
		claims     *Claims
		wantErr    bool
	}{
		{
			name:       "valid token with matching SA",
			expectedSA: "katapult-agent",
			claims:     validClaims("katapult-agent"),
			wantErr:    false,
		},
		{
			name:       "empty expected SA accepts any",
			expectedSA: "",
			claims:     validClaims("any-sa"),
			wantErr:    false,
		},
		{
			name:       "wrong service account",
			expectedSA: "katapult-agent",
			claims:     validClaims("wrong-sa"),
			wantErr:    true,
		},
		{
			name:       "expired token",
			expectedSA: "",
			claims: &Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
					IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewJWTValidator(tt.expectedSA, testKeyFunc)
			tokenStr := signTestToken(t, tt.claims)
			_, err := v.ValidateToken(tokenStr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateToken() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateToken_MalformedToken(t *testing.T) {
	v := NewJWTValidator("", testKeyFunc)
	_, err := v.ValidateToken("not.a.valid.jwt.token.at.all")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func TestExtractToken(t *testing.T) {
	tests := []struct {
		name    string
		md      metadata.MD
		noMD    bool
		want    string
		wantErr bool
	}{
		{
			name: "bearer prefix",
			md:   metadata.Pairs("authorization", "Bearer my-token"),
			want: "my-token",
		},
		{
			name: "raw token without prefix",
			md:   metadata.Pairs("authorization", "raw-token-value"),
			want: "raw-token-value",
		},
		{
			name:    "missing authorization header",
			md:      metadata.Pairs("other-header", "value"),
			wantErr: true,
		},
		{
			name:    "no metadata",
			noMD:    true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx context.Context
			if tt.noMD {
				ctx = context.Background()
			} else {
				ctx = metadata.NewIncomingContext(context.Background(), tt.md)
			}

			got, err := extractToken(ctx)
			if (err != nil) != tt.wantErr {
				t.Fatalf("extractToken() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("extractToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClaimsFromContext(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		claims := validClaims("test-sa")
		ctx := context.WithValue(context.Background(), claimsKey{}, claims)
		got, ok := ClaimsFromContext(ctx)
		if !ok {
			t.Fatal("expected claims to be present")
		}
		if got.Kubernetes.ServiceAccount.Name != "test-sa" {
			t.Fatalf("got SA %q, want %q", got.Kubernetes.ServiceAccount.Name, "test-sa")
		}
	})

	t.Run("absent", func(t *testing.T) {
		_, ok := ClaimsFromContext(context.Background())
		if ok {
			t.Fatal("expected claims to be absent")
		}
	})
}

func TestUnaryAuthInterceptor(t *testing.T) {
	validator := NewJWTValidator("katapult-agent", testKeyFunc)
	interceptor := UnaryAuthInterceptor(validator)

	dummyHandler := func(ctx context.Context, req any) (any, error) {
		claims, ok := ClaimsFromContext(ctx)
		if !ok {
			t.Fatal("expected claims in context after successful auth")
		}
		return claims.Kubernetes.ServiceAccount.Name, nil
	}

	info := &ggrpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	t.Run("valid token passes through", func(t *testing.T) {
		tokenStr := signTestToken(t, validClaims("katapult-agent"))
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tokenStr))

		resp, err := interceptor(ctx, nil, info, dummyHandler)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp != "katapult-agent" {
			t.Fatalf("expected SA name in response, got %v", resp)
		}
	})

	t.Run("missing auth returns error", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs())
		_, err := interceptor(ctx, nil, info, dummyHandler)
		if err == nil {
			t.Fatal("expected error for missing auth")
		}
	})

	t.Run("invalid token returns error", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer invalid-token"))
		_, err := interceptor(ctx, nil, info, dummyHandler)
		if err == nil {
			t.Fatal("expected error for invalid token")
		}
	})
}
