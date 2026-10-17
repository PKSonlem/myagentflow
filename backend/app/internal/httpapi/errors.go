package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/PKSonlem/myagentflow/backend/app/internal/api"
)

func errorResponse(code api.ErrorCode, msg string) api.ErrorResponse {
	var r api.ErrorResponse
	r.Error.Code = code
	r.Error.Message = msg
	return r
}

func writeError(w http.ResponseWriter, status int, code api.ErrorCode, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse(code, msg))
}

// badRequest: ошибки разбора тела и параметров от сгенерированного кода приходят сюда,
// чтобы ответ был в формате ErrorResponse, а не text/plain.
func badRequest(w http.ResponseWriter, _ *http.Request, err error) {
	writeError(w, http.StatusBadRequest, api.ErrorCodeInvalidRequest, err.Error())
}

func internalError(log *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		writeError(w, http.StatusInternalServerError, api.ErrorCodeInternalError, "internal error")
	}
}

var errNotImplemented = errors.New("not implemented")
