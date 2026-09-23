package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

const bearerPrefix = "Bearer "

// requireBearer rejects requests whose "Authorization: Bearer <token>" does not match one of
// keys, comparing in constant time.
func requireBearer(keys []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r.Header.Get("Authorization"), keys) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="gomailer"`)
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "missing or invalid bearer token"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authorized(header string, keys []string) bool {
	if !strings.HasPrefix(header, bearerPrefix) {
		return false
	}

	token := []byte(strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix)))
	match := 0
	for _, key := range keys {
		match |= subtle.ConstantTimeCompare(token, []byte(key))
	}
	return match == 1
}
