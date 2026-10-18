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

// Значения по умолчанию и границы страниц: зеркало parameters в openapi.yaml.
// Сгенерированный код границы не проверяет, поэтому их проверяют хендлеры.
const (
	defaultRunsLimit   = 20
	maxRunsLimit       = 100
	defaultEventsLimit = 200
	maxEventsLimit     = 1000
)

type Ingester interface {
	Ingest(ctx context.Context, e ingest.HookEvent) error
}

// Reader отдаёт данные для Read API; ErrNotFound и ErrInvalidCursor пакета reader превращаются в 404 и 400.
type Reader interface {
	ListRuns(ctx context.Context, cursor string, limit int) ([]reader.Run, *string, error)
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
	limit := defaultRunsLimit
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	if limit < 1 || limit > maxRunsLimit {
		return api.ListRuns400JSONResponse{BadRequestJSONResponse: invalidRequest(
			fmt.Sprintf("поле limit должно быть от 1 до %d", maxRunsLimit))}, nil
	}
	var cursor string
	if req.Params.Cursor != nil {
		cursor = *req.Params.Cursor
	}

	runs, nextCursor, err := s.reader.ListRuns(ctx, cursor, limit)
	if errors.Is(err, reader.ErrInvalidCursor) {
		return api.ListRuns400JSONResponse{BadRequestJSONResponse: invalidRequest("поле cursor недействительно")}, nil
	}
	if err != nil {
		return nil, err
	}

	out := api.RunList{Items: make([]api.Run, len(runs))}
	for i, r := range runs {
		out.Items[i] = toAPIRun(r)
	}
	out.NextCursor = nextCursor
	return api.ListRuns200JSONResponse(out), nil
}

func (s *Server) GetRun(ctx context.Context, req api.GetRunRequestObject) (api.GetRunResponseObject, error) {
	r, err := s.reader.GetRun(ctx, req.Id)
	if errors.Is(err, reader.ErrNotFound) {
		return api.GetRun404JSONResponse{NotFoundJSONResponse: runNotFound()}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.GetRun200JSONResponse(toAPIRun(r)), nil
}

func (s *Server) ListRunEvents(ctx context.Context, req api.ListRunEventsRequestObject) (api.ListRunEventsResponseObject, error) {
	limit := defaultEventsLimit
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	var afterSeq int64
	if req.Params.AfterSeq != nil {
		afterSeq = *req.Params.AfterSeq
	}
	if limit < 1 || limit > maxEventsLimit {
		return api.ListRunEvents400JSONResponse{BadRequestJSONResponse: invalidRequest(
			fmt.Sprintf("поле limit должно быть от 1 до %d", maxEventsLimit))}, nil
	}
	if afterSeq < 0 {
		return api.ListRunEvents400JSONResponse{BadRequestJSONResponse: invalidRequest(
			"поле after_seq не может быть отрицательным")}, nil
	}

	events, nextAfterSeq, err := s.reader.ListRunEvents(ctx, req.Id, afterSeq, limit)
	if errors.Is(err, reader.ErrNotFound) {
		return api.ListRunEvents404JSONResponse{NotFoundJSONResponse: runNotFound()}, nil
	}
	if err != nil {
		return nil, err
	}

	out := api.EventList{Items: make([]api.Event, len(events)), NextAfterSeq: nextAfterSeq}
	for i, e := range events {
		if out.Items[i], err = toAPIEvent(e); err != nil {
			return nil, err
		}
	}
	return api.ListRunEvents200JSONResponse(out), nil
}

func invalidRequest(msg string) api.BadRequestJSONResponse {
	return api.BadRequestJSONResponse(errorResponse(api.ErrorCodeInvalidRequest, msg))
}

func runNotFound() api.NotFoundJSONResponse {
	return api.NotFoundJSONResponse(errorResponse(api.ErrorCodeNotFound, "запуск не найден"))
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

func toAPIEvent(e reader.Event) (api.Event, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(e.Payload, &payload); err != nil {
		return api.Event{}, fmt.Errorf("event seq %d: bad payload: %w", e.Seq, err)
	}
	return api.Event{
		Seq:           e.Seq,
		HookEventName: e.HookEventName,
		ToolName:      e.ToolName,
		Ts:            e.Ts,
		Payload:       payload,
	}, nil
}
