package api

import (
	"context"
	"log/slog"
	"net/http"
)

// ValidatedUser holds the identity extracted from Rancher proxy headers.
type ValidatedUser struct {
	Username string
	Groups   []string
}

type contextKey string

const userContextKey contextKey = "validated-user"

// AuthMiddleware validates that the request came through Rancher's
// authenticated K8s service proxy. The backend never uses the user's
// credentials — it operates exclusively with its own ServiceAccount.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Rancher's K8s proxy sets Impersonate-User when forwarding
		// authenticated requests through the service proxy.
		user := r.Header.Get("Impersonate-User")
		if user == "" {
			// Fallback: check X-Api-Cattle-Auth (direct Rancher proxy)
			if r.Header.Get("X-Api-Cattle-Auth") == "" {
				slog.Warn("rejected unauthenticated request",
					"remote", r.RemoteAddr,
					"path", r.URL.Path,
				)
				http.Error(w, `{"error":"unauthorized: missing identity headers"}`, http.StatusUnauthorized)
				return
			}
			user = "authenticated-via-cattle-auth"
		}

		groups := r.Header.Values("Impersonate-Group")

		slog.Info("authenticated request",
			"user", user,
			"groups", groups,
			"path", r.URL.Path,
		)

		// Strip all auth/impersonation headers so they cannot be
		// accidentally forwarded to downstream services.
		r.Header.Del("Impersonate-User")
		r.Header.Del("Impersonate-Group")
		r.Header.Del("Impersonate-Uid")
		r.Header.Del("X-Api-Cattle-Auth")
		r.Header.Del("Authorization")

		ctx := context.WithValue(r.Context(), userContextKey, &ValidatedUser{
			Username: user,
			Groups:   groups,
		})

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserFromContext retrieves the validated user from the request context.
func UserFromContext(ctx context.Context) *ValidatedUser {
	if u, ok := ctx.Value(userContextKey).(*ValidatedUser); ok {
		return u
	}
	return nil
}
