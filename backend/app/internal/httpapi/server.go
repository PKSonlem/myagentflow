// Package httpapi реализует сгенерированный api.StrictServerInterface.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/PKSonlem/myagentflow/backend/app/internal/api"
	"github.com/PKSonlem/myagentflow/backend/app/internal/claudecode"
	"github.com/PKSonlem/myagentflow/backend/app/internal/ingest"
)

const baseURL = "/api/v1" // servers.url из openapi.yaml

type Ingester interface {
	Ingest(ctx context.Context, e ingest.Event) error
}

type Server struct {
	ingester Ingester
	now      func() time.Time
}

var _ api.StrictServerInterface = (*Server)(nil)

func NewServer(i Ingester) *Server {
	return &Server{ingester: i, now: time.Now}
}

// Handler собирает роутер: токен → лимит тела → сгенерированные обработчики.
func Handler(s *Server, token string, log *slog.Logger) http.Handler {
	strict := api.NewStrictHandlerWithOptions(s, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  badRequest,
		ResponseErrorHandlerFunc: internalError(log),
	})
	h := api.HandlerWithOptions(strict, api.StdHTTPServerOptions{
		BaseURL:          baseURL,
		ErrorHandlerFunc: badRequest,
	})
	return auth(token, limitBody(h))
}

func (s *Server) IngestClaudeHook(ctx context.Context, _ api.IngestClaudeHookRequestObject) (api.IngestClaudeHookResponseObject, error) {
	e, err := claudecode.Parse(rawBody(ctx), s.now())
	if err != nil {
		if errors.Is(err, claudecode.ErrInvalidPayload) {
			return api.IngestClaudeHook400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(
				errorResponse(api.ErrorCodeInvalidRequest, err.Error()))}, nil
		}
		return nil, err
	}
	if err := s.ingester.Ingest(ctx, e); err != nil {
		return nil, err // ResponseErrorHandlerFunc → 500
	}
	return api.IngestClaudeHook200Response{}, nil
}

// Чтение данных: T3.
func (s *Server) ListRuns(context.Context, api.ListRunsRequestObject) (api.ListRunsResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) GetRun(context.Context, api.GetRunRequestObject) (api.GetRunResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) ListRunEvents(context.Context, api.ListRunEventsRequestObject) (api.ListRunEventsResponseObject, error) {
	return nil, errNotImplemented
}
