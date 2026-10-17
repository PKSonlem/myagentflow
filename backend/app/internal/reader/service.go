// Package reader отдаёт запуски и их события для Read API.
package reader

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PKSonlem/myagentflow/backend/app/internal/repository/db"
)

var (
	ErrNotFound      = errors.New("run not found")
	ErrInvalidCursor = errors.New("invalid cursor")
)

type Run struct {
	ID         uuid.UUID
	SessionID  string
	Status     string
	Cwd        string
	TaskText   *string
	StartedAt  time.Time
	FinishedAt *time.Time
	DurationMs *int64 // nil, пока Run не завершён
}

type Event struct {
	Seq           int64
	HookEventName string
	ToolName      *string
	Ts            time.Time
	Payload       []byte
}

type Service struct {
	q *db.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{q: db.New(pool)}
}

// ListRuns возвращает страницу запусков, новые сверху. nextCursor == "" на последней странице.
func (s *Service) ListRuns(ctx context.Context, cursor string, limit int) (runs []Run, nextCursor string, err error) {
	// limit+1: лишняя строка говорит, что есть следующая страница.
	size := int32(limit + 1)
	var rows []db.Run
	if cursor == "" {
		rows, err = s.q.ListRunsFirstPage(ctx, size)
	} else {
		var startedAt time.Time
		var id uuid.UUID
		if startedAt, id, err = decodeCursor(cursor); err != nil {
			return nil, "", err
		}
		rows, err = s.q.ListRunsAfter(ctx, db.ListRunsAfterParams{
			CursorStartedAt: pgtype.Timestamptz{Time: startedAt, Valid: true},
			CursorID:        pgtype.UUID{Bytes: id, Valid: true},
			PageSize:        size,
		})
	}
	if err != nil {
		return nil, "", fmt.Errorf("list runs: %w", err)
	}

	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		nextCursor = encodeCursor(last.StartedAt.Time, last.ID.Bytes)
	}
	runs = make([]Run, len(rows))
	for i, r := range rows {
		runs[i] = toRun(r)
	}
	return runs, nextCursor, nil
}

func (s *Service) GetRun(ctx context.Context, id uuid.UUID) (Run, error) {
	r, err := s.q.GetRun(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	if err != nil {
		return Run{}, fmt.Errorf("get run: %w", err)
	}
	return toRun(r), nil
}

// ListRunEvents возвращает события после afterSeq. nextAfterSeq == nil на последней странице.
func (s *Service) ListRunEvents(ctx context.Context, runID uuid.UUID, afterSeq int64, limit int) (events []Event, nextAfterSeq *int64, err error) {
	if _, err := s.GetRun(ctx, runID); err != nil {
		return nil, nil, err
	}
	rows, err := s.q.ListRunEvents(ctx, db.ListRunEventsParams{
		RunID:    pgtype.UUID{Bytes: runID, Valid: true},
		AfterSeq: afterSeq,
		PageSize: int32(limit + 1),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list run events: %w", err)
	}

	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1].Seq
		nextAfterSeq = &last
	}
	events = make([]Event, len(rows))
	for i, e := range rows {
		events[i] = Event{
			Seq:           e.Seq,
			HookEventName: e.HookEventName,
			ToolName:      textPtr(e.ToolName),
			Ts:            e.Ts.Time.UTC(),
			Payload:       e.Payload,
		}
	}
	return events, nextAfterSeq, nil
}

func toRun(r db.Run) Run {
	out := Run{
		ID:        r.ID.Bytes,
		SessionID: r.SessionID,
		Status:    r.Status,
		Cwd:       r.Cwd,
		TaskText:  textPtr(r.TaskText),
		StartedAt: r.StartedAt.Time.UTC(),
	}
	if r.FinishedAt.Valid {
		f := r.FinishedAt.Time.UTC()
		out.FinishedAt = &f
		if r.Status == "finished" {
			d := f.Sub(out.StartedAt).Milliseconds()
			out.DurationMs = &d
		}
	}
	return out
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// Курсор непрозрачный для клиента: base64url("<started_at в микросекундах>:<id>").
// Микросекунды это точность timestamptz, поэтому границы страниц не теряют строки.
func encodeCursor(startedAt time.Time, id uuid.UUID) string {
	raw := strconv.FormatInt(startedAt.UnixMicro(), 10) + ":" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(c string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	micros, idStr, ok := strings.Cut(string(b), ":")
	if !ok {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	us, err := strconv.ParseInt(micros, 10, 64)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	return time.UnixMicro(us).UTC(), id, nil
}
