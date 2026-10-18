package claudecode_test

import (
	"errors"
	"testing"
	"time"

	"github.com/PKSonlem/myagentflow/backend/app/internal/claudecode"
)

func TestParse(t *testing.T) {
	now := time.Now()
	raw := []byte(`{"session_id":"s1","hook_event_name":"PreToolUse","tool_name":"Bash","tool_use_id":"toolu_1","new_field":{"a":1}}`)
	e, err := claudecode.Parse(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if e.SessionID != "s1" || e.HookEventName != "PreToolUse" || *e.ToolName != "Bash" || *e.ToolUseID != "toolu_1" {
		t.Fatalf("unexpected event: %+v", e)
	}
	if string(e.Payload) != string(raw) {
		t.Fatal("payload must be stored unchanged")
	}
}

func TestParse_UnknownEventIsAccepted(t *testing.T) {
	if _, err := claudecode.Parse([]byte(`{"session_id":"s1","hook_event_name":"BrandNew"}`), time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestParse_EmptyToolUseIDBecomesNil(t *testing.T) {
	e, err := claudecode.Parse([]byte(`{"session_id":"s1","hook_event_name":"Stop","tool_use_id":""}`), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if e.ToolUseID != nil {
		t.Fatal("empty tool_use_id must be nil")
	}
}

func TestParse_Invalid(t *testing.T) {
	for _, raw := range []string{``, `not json`, `null`, `[]`, `{}`, `{"session_id":"s1"}`, `{"hook_event_name":"Stop"}`} {
		if _, err := claudecode.Parse([]byte(raw), time.Now()); !errors.Is(err, claudecode.ErrInvalidPayload) {
			t.Errorf("%q: err = %v, want ErrInvalidPayload", raw, err)
		}
	}
}

func TestParse_EmptyPromptBecomesNil(t *testing.T) {
	e, err := claudecode.Parse([]byte(`{"session_id":"s1","hook_event_name":"UserPromptSubmit","prompt":""}`), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if e.Prompt != nil {
		t.Fatal("empty prompt must be nil")
	}
}

func TestParse_NULIsRejected(t *testing.T) {
	for _, raw := range []string{
		`{"session_id":"s1","hook_event_name":"UserPromptSubmit","prompt":"a\u0000b"}`,
		`{"session_id":"s1","hook_event_name":"PreToolUse","tool_input":{"cmd":"x\u0000"}}`,
		`{"session_id":"s1","hook_event_name":"PreToolUse","tool_input":{"k\u0000":"x"}}`,
	} {
		if _, err := claudecode.Parse([]byte(raw), time.Now()); !errors.Is(err, claudecode.ErrInvalidPayload) {
			t.Errorf("%s: err = %v, want ErrInvalidPayload", raw, err)
		}
	}
}

func TestParse_EscapedBackslashBeforeU0000IsText(t *testing.T) {
	raw := `{"session_id":"s1","hook_event_name":"UserPromptSubmit","prompt":"literal \\u0000"}`
	if _, err := claudecode.Parse([]byte(raw), time.Now()); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}
