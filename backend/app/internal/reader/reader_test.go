package reader_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/PKSonlem/myagentflow/backend/app/internal/httpapi"
	"github.com/PKSonlem/myagentflow/backend/app/internal/ingest"
	"github.com/PKSonlem/myagentflow/backend/app/internal/reader"
)

const (
	migrationsDir = "../../migrations"
	specPath      = "../../../api/openapi.yaml"
	token         = "secret"
)

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

func clean(t *testing.T) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), "TRUNCATE events, runs"); err != nil {
		t.Fatal(err)
	}
}

// send кладёт событие через Ingest: данные те же, что в бою.
func send(t *testing.T, session, name, toolUseID string, at time.Time) {
	t.Helper()
	e := ingest.HookEvent{
		SessionID:     session,
		HookEventName: name,
		Cwd:           "/work",
		Payload:       []byte(`{"session_id":"` + session + `","hook_event_name":"` + name + `","n":1}`),
		ReceivedAt:    at,
	}
	if toolUseID != "" {
		e.ToolName = ptr("Bash")
		e.ToolUseID = &toolUseID
	}
	if name == "UserPromptSubmit" {
		e.Prompt = ptr("сделай задачу")
	}
	if err := ingest.NewService(testPool).Ingest(context.Background(), e); err != nil {
		t.Fatal(err)
	}
}

func ptr[T any](v T) *T { return &v }

func listAll(t *testing.T, svc *reader.Service, limit int) (ids []uuid.UUID, pages int) {
	t.Helper()
	cursor := ""
	for {
		runs, next, err := svc.ListRuns(context.Background(), cursor, limit)
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, r := range runs {
			ids = append(ids, r.ID)
		}
		if next == nil {
			return ids, pages
		}
		cursor = *next
	}
}

func TestListRuns_NewestFirstAndPaginates(t *testing.T) {
	clean(t)
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		send(t, string(rune('a'+i)), "Stop", "", base.Add(time.Duration(i)*time.Minute))
	}
	svc := reader.NewService(testPool)

	ids, pages := listAll(t, svc, 2)
	if len(ids) != 5 || pages != 3 {
		t.Fatalf("got %d runs on %d pages, want 5 on 3", len(ids), pages)
	}
	all, _, err := svc.ListRuns(context.Background(), "", 100)
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range all {
		if r.ID != ids[i] {
			t.Fatalf("pages order differs from single page at %d", i)
		}
		if i > 0 && !all[i-1].StartedAt.After(r.StartedAt) {
			t.Fatalf("not newest first at %d", i)
		}
	}
}

func TestListRuns_ExactPageBoundaryHasNoNextCursor(t *testing.T) {
	clean(t)
	base := time.Now().UTC()
	send(t, "a", "Stop", "", base)
	send(t, "b", "Stop", "", base.Add(time.Second))

	runs, next, err := reader.NewService(testPool).ListRuns(context.Background(), "", 2)
	if err != nil || len(runs) != 2 || next != nil {
		t.Fatalf("runs=%d next=%v err=%v, want 2 runs and empty cursor", len(runs), next, err)
	}
}

func TestListRuns_EmptyAndSameStartedAt(t *testing.T) {
	clean(t)
	svc := reader.NewService(testPool)
	if runs, next, err := svc.ListRuns(context.Background(), "", 20); err != nil || len(runs) != 0 || next != nil {
		t.Fatalf("empty: runs=%d next=%v err=%v", len(runs), next, err)
	}

	// одинаковый started_at: порядок решает id, ничего не теряется и не дублируется
	at := time.Now().UTC()
	for _, s := range []string{"a", "b", "c", "d"} {
		send(t, s, "Stop", "", at)
	}
	ids, _ := listAll(t, svc, 1)
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	if len(ids) != 4 || len(seen) != 4 {
		t.Fatalf("ids=%d unique=%d, want 4/4", len(ids), len(seen))
	}
}

// Новые запуски между запросами страниц не сдвигают и не дублируют уже выданные.
func TestListRuns_StableWhenNewRunsArrive(t *testing.T) {
	clean(t)
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		send(t, string(rune('a'+i)), "Stop", "", base.Add(time.Duration(i)*time.Minute))
	}
	svc := reader.NewService(testPool)
	ctx := context.Background()

	first, next, err := svc.ListRuns(ctx, "", 2)
	if err != nil || next == nil {
		t.Fatalf("first page: next=%v err=%v", next, err)
	}
	send(t, "new1", "Stop", "", base.Add(time.Hour))
	send(t, "new2", "Stop", "", base.Add(2*time.Hour))

	second, next2, err := svc.ListRuns(ctx, *next, 2)
	if err != nil || next2 != nil || len(second) != 2 {
		t.Fatalf("second page: n=%d next=%v err=%v", len(second), next2, err)
	}
	for _, r := range second {
		for _, f := range first {
			if r.ID == f.ID {
				t.Fatalf("run %s repeated on the second page", r.ID)
			}
		}
	}
	if !second[0].StartedAt.Before(first[1].StartedAt) {
		t.Fatal("second page must continue right after the first")
	}
}

func TestListRuns_InvalidCursor(t *testing.T) {
	for _, c := range []string{"!!!", "bm9jb2xvbg", "YTpi"} {
		_, _, err := reader.NewService(testPool).ListRuns(context.Background(), c, 10)
		if err != reader.ErrInvalidCursor {
			t.Errorf("cursor %q: err = %v, want ErrInvalidCursor", c, err)
		}
	}
}

func TestRun_DurationOnlyWhenFinished(t *testing.T) {
	clean(t)
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	send(t, "s1", "UserPromptSubmit", "", at)
	svc := reader.NewService(testPool)

	runs, _, _ := svc.ListRuns(context.Background(), "", 10)
	r := runs[0]
	if r.Status != "running" || r.DurationMs != nil || r.FinishedAt != nil {
		t.Fatalf("running run: %+v", r)
	}
	if r.TaskText == nil || *r.TaskText != "сделай задачу" {
		t.Fatalf("task_text = %v", r.TaskText)
	}

	send(t, "s1", "SessionEnd", "", at.Add(90*time.Second+500*time.Millisecond))
	got, err := svc.GetRun(context.Background(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "finished" || got.DurationMs == nil || *got.DurationMs != 90500 {
		t.Fatalf("finished run: %+v duration=%v", got, got.DurationMs)
	}

	// resume: снова running, duration пропадает
	send(t, "s1", "PreToolUse", "toolu_1", at.Add(10*time.Minute))
	got, _ = svc.GetRun(context.Background(), r.ID)
	if got.Status != "running" || got.DurationMs != nil || got.FinishedAt != nil {
		t.Fatalf("resumed run: %+v", got)
	}
}

func TestGetRun_NotFound(t *testing.T) {
	clean(t)
	_, err := reader.NewService(testPool).GetRun(context.Background(), uuid.New())
	if err != reader.ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestListRunEvents_OrderPaginationAndNotFound(t *testing.T) {
	clean(t)
	at := time.Now().UTC()
	send(t, "s1", "SessionStart", "", at)
	send(t, "s1", "PreToolUse", "t1", at.Add(time.Second))
	send(t, "s1", "PostToolUse", "t1", at.Add(2*time.Second))
	send(t, "s1", "Stop", "", at.Add(3*time.Second))
	send(t, "other", "Stop", "", at)
	svc := reader.NewService(testPool)
	ctx := context.Background()
	runs, _, _ := svc.ListRuns(ctx, "", 10)
	var runID uuid.UUID
	for _, r := range runs {
		if r.SessionID == "s1" {
			runID = r.ID
		}
	}

	var seqs []int64
	var after int64
	for {
		evs, next, err := svc.ListRunEvents(ctx, runID, after, 3)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range evs {
			seqs = append(seqs, e.Seq)
		}
		if next == nil {
			break
		}
		after = *next
	}
	want := []int64{1, 2, 3, 4}
	if len(seqs) != len(want) {
		t.Fatalf("seqs = %v, want %v", seqs, want)
	}
	for i := range want {
		if seqs[i] != want[i] {
			t.Fatalf("seqs = %v, want %v", seqs, want)
		}
	}

	if evs, next, err := svc.ListRunEvents(ctx, runID, 4, 3); err != nil || len(evs) != 0 || next != nil {
		t.Fatalf("past the end: n=%d next=%v err=%v", len(evs), next, err)
	}
	if _, _, err := svc.ListRunEvents(ctx, uuid.New(), 0, 3); err != reader.ErrNotFound {
		t.Fatalf("unknown run: err = %v, want ErrNotFound", err)
	}
}

// --- HTTP: ответы проверяются по openapi.yaml ---

func TestSpecIsValid(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// get ходит в полный http-стек (токен, роутер, хендлеры, Postgres) и проверяет ответ по спеке.
func get(t *testing.T, path string, withToken bool) (int, []byte) {
	t.Helper()
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	router, err := legacy.NewRouter(doc)
	if err != nil {
		t.Fatal(err)
	}

	h := httpapi.Handler(httpapi.NewServer(ingest.NewService(testPool), reader.NewService(testPool)),
		token, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8980/api/v1"+path, nil)
	if withToken {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	resp := rec.Result()
	body, _ := io.ReadAll(resp.Body)

	route, pathParams, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("route %s: %v", path, err)
	}
	err = openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: pathParams,
			Route:      route,
			Options:    &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		},
		Status: resp.StatusCode,
		Header: resp.Header,
		Body:   io.NopCloser(bytes.NewReader(body)),
	})
	if err != nil {
		t.Errorf("GET %s: response does not match spec: %v\nbody: %s", path, err, body)
	}
	return resp.StatusCode, body
}

func TestHTTP_ReadAPIMatchesSpec(t *testing.T) {
	clean(t)
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	send(t, "s1", "UserPromptSubmit", "", at)
	send(t, "s1", "PreToolUse", "t1", at.Add(time.Second))
	send(t, "s1", "SessionEnd", "", at.Add(2*time.Second))
	send(t, "s2", "Stop", "", at.Add(time.Minute))

	// список: страница из одного, курсор есть; вторая страница без курсора
	code, body := get(t, "/runs?limit=1", true)
	if code != 200 {
		t.Fatalf("status = %d: %s", code, body)
	}
	var page struct {
		Items []struct {
			ID        string `json:"id"`
			SessionID string `json:"session_id"`
		}
		NextCursor *string `json:"next_cursor"`
	}
	_ = json.Unmarshal(body, &page)
	if len(page.Items) != 1 || page.NextCursor == nil || page.Items[0].SessionID != "s2" {
		t.Fatalf("page 1: %s", body)
	}
	code, body = get(t, "/runs?limit=1&cursor="+*page.NextCursor, true)
	_ = json.Unmarshal(body, &page)
	if code != 200 || len(page.Items) != 1 || page.NextCursor != nil || page.Items[0].SessionID != "s1" {
		t.Fatalf("page 2: %d %s", code, body)
	}
	runID := page.Items[0].ID

	if code, body = get(t, "/runs/"+runID, true); code != 200 {
		t.Fatalf("get run: %d %s", code, body)
	}
	code, body = get(t, "/runs/"+runID+"/events?limit=2", true)
	var evs struct {
		Items []struct {
			Seq     int64
			Payload map[string]any
		}
		NextAfterSeq *int64 `json:"next_after_seq"`
	}
	_ = json.Unmarshal(body, &evs)
	if code != 200 || len(evs.Items) != 2 || evs.NextAfterSeq == nil || *evs.NextAfterSeq != 2 || evs.Items[0].Payload["n"] != float64(1) {
		t.Fatalf("events: %d %s", code, body)
	}
	if code, body = get(t, "/runs/"+runID+"/events?after_seq=2&limit=2", true); code != 200 {
		t.Fatalf("events page 2: %d %s", code, body)
	}
}

func TestHTTP_Errors(t *testing.T) {
	clean(t)
	missing := uuid.NewString()
	for _, tc := range []struct {
		path  string
		token bool
		want  int
	}{
		{"/runs", false, 401},
		{"/runs/" + missing, false, 401},
		{"/runs/" + missing + "/events", false, 401},
		{"/runs/" + missing, true, 404},
		{"/runs/" + missing + "/events", true, 404},
		{"/runs/not-a-uuid", true, 400},
		{"/runs?limit=0", true, 400},
		{"/runs?limit=101", true, 400},
		{"/runs?limit=abc", true, 400},
		{"/runs?cursor=garbage", true, 400},
		{"/runs/" + missing + "/events?limit=1001", true, 400},
		{"/runs/" + missing + "/events?after_seq=-1", true, 400},
		{"/runs", true, 200},
	} {
		if code, body := get(t, tc.path, tc.token); code != tc.want {
			t.Errorf("GET %s token=%v: status = %d, want %d (%s)", tc.path, tc.token, code, tc.want, body)
		}
	}
}
