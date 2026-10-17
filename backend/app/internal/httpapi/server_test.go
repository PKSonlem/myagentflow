package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PKSonlem/myagentflow/backend/app/internal/ingest"
)

type fakeIngester struct {
	got []ingest.Event
	err error
}

func (f *fakeIngester) Ingest(_ context.Context, e ingest.Event) error {
	f.got = append(f.got, e)
	return f.err
}

const testToken = "secret"

func post(t *testing.T, ing *fakeIngester, token, body string) *http.Response {
	t.Helper()
	return postAuth(t, ing, "Bearer "+token, token == "", body)
}

func postAuth(t *testing.T, ing *fakeIngester, authHeader string, noAuth bool, body string) *http.Response {
	t.Helper()
	h := Handler(NewServer(ing, nil), testToken, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/hooks/claude", strings.NewReader(body))
	if !noAuth {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func TestIngest_Unauthorized(t *testing.T) {
	for _, token := range []string{"", "wrong", "secret2"} {
		ing := &fakeIngester{}
		resp := post(t, ing, token, `{"session_id":"s1","hook_event_name":"Stop"}`)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("token %q: status = %d, want 401", token, resp.StatusCode)
		}
		if len(ing.got) != 0 {
			t.Errorf("token %q: event was saved", token)
		}
	}
}

func TestIngest_OKKeepsRawPayload(t *testing.T) {
	ing := &fakeIngester{}
	body := `{"session_id":"s1","hook_event_name":"PreToolUse","tool_use_id":"toolu_1","unknown":{"x":[1,2]}}`
	resp := post(t, ing, testToken, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if b, _ := io.ReadAll(resp.Body); len(b) != 0 {
		t.Fatalf("body must be empty, got %q", b)
	}
	if len(ing.got) != 1 || string(ing.got[0].Payload) != body {
		t.Fatalf("payload must be stored unchanged: %+v", ing.got)
	}
}

func TestIngest_BadRequest(t *testing.T) {
	for _, body := range []string{``, `not json`, `{}`, `{"session_id":"s1"}`, `{"hook_event_name":"Stop"}`} {
		ing := &fakeIngester{}
		resp := post(t, ing, testToken, body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%q: status = %d, want 400", body, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("%q: content-type = %q, want application/json", body, ct)
		}
		if len(ing.got) != 0 {
			t.Errorf("%q: event was saved", body)
		}
	}
}

func TestIngest_TooLarge(t *testing.T) {
	ing := &fakeIngester{}
	big := `{"session_id":"s1","hook_event_name":"Stop","pad":"` + strings.Repeat("a", maxBodyBytes) + `"}`
	resp := post(t, ing, testToken, big)
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}
	if len(ing.got) != 0 {
		t.Fatal("event was saved")
	}
}

func TestIngest_StorageErrorIs500(t *testing.T) {
	ing := &fakeIngester{err: errors.New("db down")}
	resp := post(t, ing, testToken, `{"session_id":"s1","hook_event_name":"Stop"}`)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

func TestIngest_BearerSchemeIsCaseInsensitive(t *testing.T) {
	for _, header := range []string{"bearer " + testToken, "BEARER " + testToken, "Bearer  " + testToken} {
		ing := &fakeIngester{}
		resp := postAuth(t, ing, header, false, `{"session_id":"s1","hook_event_name":"Stop"}`)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%q: status = %d, want 200", header, resp.StatusCode)
		}
	}
}

func TestIngest_NULInPayloadIs400(t *testing.T) {
	ing := &fakeIngester{}
	resp := post(t, ing, testToken, `{"session_id":"s1","hook_event_name":"UserPromptSubmit","prompt":"a\u0000b"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if len(ing.got) != 0 {
		t.Fatal("event was saved")
	}
}
