package ingest_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/PKSonlem/myagentflow/backend/app/internal/ingest"
)

const migrationsDir = "../../migrations"

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, "postgres:17",
		tcpostgres.WithDatabase("agentflow"),
		tcpostgres.WithUsername("agentflow"),
		tcpostgres.WithPassword("agentflow"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	if err != nil {
		panic(err)
	}
	defer func() { _ = ctr.Terminate(ctx) }()

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic(err)
	}

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		panic(err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		panic(err)
	}
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		panic(err)
	}
	_ = sqlDB.Close()

	testPool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		panic(err)
	}
	code := m.Run()
	testPool.Close()
	_ = ctr.Terminate(ctx)
	os.Exit(code)
}

// newService возвращает Service на чистой БД.
func newService(t *testing.T) *ingest.Service {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "TRUNCATE events, runs")
	if err != nil {
		t.Fatal(err)
	}
	return ingest.NewService(testPool)
}

// ev собирает событие хука; tool_use_id пустая строка значит «нет».
func ev(session, name, toolUseID string) ingest.Event {
	e := ingest.Event{
		SessionID:     session,
		HookEventName: name,
		Cwd:           "/work",
		Payload:       []byte(`{"hook_event_name":"` + name + `"}`),
		ReceivedAt:    time.Now().UTC(),
	}
	if toolUseID != "" {
		e.ToolUseID = &toolUseID
	}
	return e
}

func mustIngest(t *testing.T, svc *ingest.Service, e ingest.Event) {
	t.Helper()
	if err := svc.Ingest(context.Background(), e); err != nil {
		t.Fatalf("ingest %s: %v", e.HookEventName, err)
	}
}

func count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := testPool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// 20 запросов одновременно в один Run: seq должны быть ровно 1..20, без дублей и дыр.
func TestIngest_ParallelRequestsGetDistinctSeq(t *testing.T) {
	svc := newService(t)
	const n = 20

	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// у каждого свой tool_use_id, чтобы идемпотентность не склеила события
			errs <- svc.Ingest(context.Background(), ev("s1", "PreToolUse", fmt.Sprintf("toolu_%d", i)))
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	if got := count(t, "SELECT count(*) FROM events"); got != n {
		t.Fatalf("events = %d, want %d", got, n)
	}
	// ровно n разных seq, минимум 1 и максимум n => это в точности 1..n
	if got := count(t, "SELECT count(DISTINCT seq) FROM events WHERE seq BETWEEN 1 AND $1", n); got != n {
		t.Fatalf("distinct seq in 1..%d = %d, want %d", n, got, n)
	}
	if got := count(t, "SELECT count(*) FROM runs"); got != 1 {
		t.Fatalf("runs = %d, want 1", got)
	}
}

func TestIngest_DuplicateToolEventIsIgnored(t *testing.T) {
	svc := newService(t)
	for range 2 {
		mustIngest(t, svc, ev("s1", "PreToolUse", "toolu_1"))
		mustIngest(t, svc, ev("s1", "PostToolUse", "toolu_1"))
	}
	if got := count(t, "SELECT count(*) FROM events WHERE hook_event_name='PreToolUse'"); got != 1 {
		t.Fatalf("PreToolUse = %d, want 1", got)
	}
	if got := count(t, "SELECT count(*) FROM events WHERE hook_event_name='PostToolUse'"); got != 1 {
		t.Fatalf("PostToolUse = %d, want 1", got)
	}
}

func TestIngest_TwoStopsGiveTwoEvents(t *testing.T) {
	svc := newService(t)
	mustIngest(t, svc, ev("s1", "Stop", ""))
	mustIngest(t, svc, ev("s1", "Stop", ""))
	if got := count(t, "SELECT count(*) FROM events WHERE hook_event_name='Stop'"); got != 2 {
		t.Fatalf("Stop = %d, want 2", got)
	}
}

func TestIngest_SessionEndFinishesAndAnyEventReopens(t *testing.T) {
	svc := newService(t)
	mustIngest(t, svc, ev("s1", "SessionStart", ""))
	mustIngest(t, svc, ev("s1", "SessionEnd", ""))
	if got := count(t, "SELECT count(*) FROM runs WHERE status='finished' AND finished_at IS NOT NULL"); got != 1 {
		t.Fatalf("finished runs = %d, want 1", got)
	}

	// resume не шлёт SessionStart по http: Run открывает любое следующее событие
	mustIngest(t, svc, ev("s1", "UserPromptSubmit", ""))
	if got := count(t, "SELECT count(*) FROM runs WHERE status='running' AND finished_at IS NULL"); got != 1 {
		t.Fatalf("running runs = %d, want 1", got)
	}
	if got := count(t, "SELECT count(*) FROM runs"); got != 1 {
		t.Fatalf("runs = %d, want 1", got)
	}
}

func TestIngest_FirstEventCreatesRun(t *testing.T) {
	svc := newService(t)
	mustIngest(t, svc, ev("s1", "PreToolUse", "toolu_1"))
	if got := count(t, "SELECT count(*) FROM runs WHERE session_id='s1' AND status='running'"); got != 1 {
		t.Fatalf("runs = %d, want 1 running", got)
	}
}

func TestIngest_TaskTextFromFirstPromptOnly(t *testing.T) {
	svc := newService(t)
	for _, text := range []string{"первый", "второй"} {
		e := ev("s1", "UserPromptSubmit", "")
		e.Prompt = &text
		mustIngest(t, svc, e)
	}
	if got := count(t, "SELECT count(*) FROM runs WHERE task_text='первый'"); got != 1 {
		t.Fatalf("task_text is not the first prompt")
	}
}
