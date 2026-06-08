package api

import (
	"context"

	"github.com/distributed-file-storage/service/src/infrastructure/auth"
)

type contextKey string

const claimsKey contextKey = "claims"

func WithClaims(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

func GetClaims(ctx context.Context) *auth.Claims {
	claims, ok := ctx.Value(claimsKey).(*auth.Claims)
	if !ok {
		return nil
	}
	return claims
}

func GetUserID(ctx context.Context) string {
	claims := GetClaims(ctx)
	if claims == nil {
		return ""
	}
	return claims.UserID
}

func GetUsername(ctx context.Context) string {
	claims := GetClaims(ctx)
	if claims == nil {
		return ""
	}
	return claims.Username
}

func GetUserRole(ctx context.Context) string {
	claims := GetClaims(ctx)
	if claims == nil {
		return ""
	}
	return claims.Role
}
