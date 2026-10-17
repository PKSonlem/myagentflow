package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PKSonlem/myagentflow/backend/app/internal/repository/db"
)

// Event это результат адаптера claudecode: то, что нужно сохранить.
type Event struct {
	SessionID     string
	HookEventName string
	Cwd           string
	ToolName      *string
	ToolUseID     *string
	Prompt        *string // только UserPromptSubmit
	Payload       []byte  // исходное тело хука без изменений
	ReceivedAt    time.Time
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Ingest сохраняет событие в одной транзакции:
// UpsertRun (до всего: берёт блокировку Run) → SetRunTaskText / FinishRun → InsertEvent.
func (s *Service) Ingest(ctx context.Context, e Event) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ingestTx(ctx, db.New(tx), e); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func ingestTx(ctx context.Context, q *db.Queries, e Event) error {
	runID, err := q.UpsertRun(ctx, db.UpsertRunParams{
		ID:        newUUID(),
		SessionID: e.SessionID,
		Cwd:       e.Cwd,
		StartedAt: pgtype.Timestamptz{Time: e.ReceivedAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("upsert run: %w", err)
	}

	switch e.HookEventName {
	case "UserPromptSubmit":
		if e.Prompt != nil {
			err = q.SetRunTaskText(ctx, db.SetRunTaskTextParams{
				ID:       runID,
				TaskText: pgtype.Text{String: *e.Prompt, Valid: true},
			})
		}
	case "SessionEnd":
		err = q.FinishRun(ctx, db.FinishRunParams{
			ID:         runID,
			FinishedAt: pgtype.Timestamptz{Time: e.ReceivedAt, Valid: true},
		})
	}
	if err != nil {
		return fmt.Errorf("update run: %w", err)
	}

	_, err = q.InsertEvent(ctx, db.InsertEventParams{
		ID:            newUUID(),
		RunID:         runID,
		HookEventName: e.HookEventName,
		ToolName:      text(e.ToolName),
		ToolUseID:     text(e.ToolUseID),
		Ts:            pgtype.Timestamptz{Time: e.ReceivedAt, Valid: true},
		Payload:       e.Payload,
	})
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func newUUID() pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.Must(uuid.NewV7()), Valid: true}
}

func text(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}
