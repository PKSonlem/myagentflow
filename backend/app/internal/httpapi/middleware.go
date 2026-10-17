package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/PKSonlem/myagentflow/backend/app/internal/api"
)

const maxBodyBytes = 10 << 20 // 10 МиБ

type rawBodyKey struct{}

// auth проверяет статический токен из Authorization: Bearer.
// Сравниваются SHA-256 хэши: ConstantTimeCompare не зависит от длины токена.
func auth(token string, next http.Handler) http.Handler {
	want := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := bearerToken(r.Header.Get("Authorization"))
		sum := sha256.Sum256([]byte(got))
		if !ok || subtle.ConstantTimeCompare(sum[:], want[:]) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, http.StatusUnauthorized, api.ErrorCodeUnauthorized, "missing or invalid token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// limitBody читает тело целиком с лимитом 10 МиБ. Сырые байты кладёт в контекст
// (payload хранится без изменений), а в r.Body оставляет копию для сгенерированного декодера.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				writeError(w, http.StatusRequestEntityTooLarge, api.ErrorCodePayloadTooLarge, "request body is larger than 10 MiB")
				return
			}
			badRequest(w, r, err)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), rawBodyKey{}, raw)))
	})
}

func rawBody(ctx context.Context) []byte {
	b, _ := ctx.Value(rawBodyKey{}).([]byte)
	return b
}

// bearerToken: схема авторизации нечувствительна к регистру (RFC 7235).
func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	return strings.TrimSpace(token), true
}
