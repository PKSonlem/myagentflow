// Package httpapi реализует сгенерированный api.StrictServerInterface.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/PKSonlem/myagentflow/backend/app/internal/api"
	"github.com/PKSonlem/myagentflow/backend/app/internal/claudecode"
	"github.com/PKSonlem/myagentflow/backend/app/internal/ingest"
	"github.com/PKSonlem/myagentflow/backend/app/internal/reader"
)

const baseURL = "/api/v1" // servers.url из openapi.yaml

type Ingester interface {
	Ingest(ctx context.Context, e ingest.Event) error
}

type Reader interface {
	ListRuns(ctx context.Context, cursor string, limit int) ([]reader.Run, string, error)
	GetRun(ctx context.Context, id uuid.UUID) (reader.Run, error)
	ListRunEvents(ctx context.Context, runID uuid.UUID, afterSeq int64, limit int) ([]reader.Event, *int64, error)
}

type Server struct {
	ingester Ingester
	reader   Reader
	now      func() time.Time
}

var _ api.StrictServerInterface = (*Server)(nil)

func NewServer(i Ingester, r Reader) *Server {
	return &Server{ingester: i, reader: r, now: time.Now}
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

func (s *Server) ListRuns(ctx context.Context, req api.ListRunsRequestObject) (api.ListRunsResponseObject, error) {
	limit := 20
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	if limit < 1 || limit > 100 {
		return api.ListRuns400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(
			errorResponse(api.ErrorCodeInvalidRequest, "поле limit должно быть от 1 до 100"))}, nil
	}
	var cursor string
	if req.Params.Cursor != nil {
		cursor = *req.Params.Cursor
	}

	runs, next, err := s.reader.ListRuns(ctx, cursor, limit)
	if errors.Is(err, reader.ErrInvalidCursor) {
		return api.ListRuns400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(
			errorResponse(api.ErrorCodeInvalidRequest, "поле cursor неверное"))}, nil
	}
	if err != nil {
		return nil, err
	}

	out := api.RunList{Items: make([]api.Run, len(runs))}
	for i, r := range runs {
		out.Items[i] = toAPIRun(r)
	}
	if next != "" {
		out.NextCursor = &next
	}
	return api.ListRuns200JSONResponse(out), nil
}

func (s *Server) GetRun(ctx context.Context, req api.GetRunRequestObject) (api.GetRunResponseObject, error) {
	r, err := s.reader.GetRun(ctx, req.Id)
	if errors.Is(err, reader.ErrNotFound) {
		return api.GetRun404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(
			errorResponse(api.ErrorCodeNotFound, "запуск не найден"))}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.GetRun200JSONResponse(toAPIRun(r)), nil
}

func (s *Server) ListRunEvents(ctx context.Context, req api.ListRunEventsRequestObject) (api.ListRunEventsResponseObject, error) {
	limit := 200
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	var afterSeq int64
	if req.Params.AfterSeq != nil {
		afterSeq = *req.Params.AfterSeq
	}
	if limit < 1 || limit > 1000 {
		return api.ListRunEvents400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(
			errorResponse(api.ErrorCodeInvalidRequest, "поле limit должно быть от 1 до 1000"))}, nil
	}
	if afterSeq < 0 {
		return api.ListRunEvents400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(
			errorResponse(api.ErrorCodeInvalidRequest, "поле after_seq не может быть отрицательным"))}, nil
	}

	events, next, err := s.reader.ListRunEvents(ctx, req.Id, afterSeq, limit)
	if errors.Is(err, reader.ErrNotFound) {
		return api.ListRunEvents404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(
			errorResponse(api.ErrorCodeNotFound, "запуск не найден"))}, nil
	}
	if err != nil {
		return nil, err
	}

	out := api.EventList{Items: make([]api.Event, len(events)), NextAfterSeq: next}
	for i, e := range events {
		var payload map[string]interface{}
		if err := json.Unmarshal(e.Payload, &payload); err != nil {
			return nil, fmt.Errorf("event seq %d: bad payload: %w", e.Seq, err)
		}
		out.Items[i] = api.Event{
			Seq:           e.Seq,
			HookEventName: e.HookEventName,
			ToolName:      e.ToolName,
			Ts:            e.Ts,
			Payload:       payload,
		}
	}
	return api.ListRunEvents200JSONResponse(out), nil
}

func toAPIRun(r reader.Run) api.Run {
	return api.Run{
		Id:         r.ID,
		SessionId:  r.SessionID,
		Status:     api.RunStatus(r.Status),
		Cwd:        r.Cwd,
		TaskText:   r.TaskText,
		StartedAt:  r.StartedAt,
		FinishedAt: r.FinishedAt,
		DurationMs: r.DurationMs,
	}
}
