// Package claudecode переводит нативный payload http-хука Claude Code во внутреннее событие.
package claudecode

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/PKSonlem/myagentflow/backend/app/internal/ingest"
)

var ErrInvalidPayload = errors.New("invalid hook payload")

// hook это только общие поля входа, нужные серверу. Остальное остаётся в Payload как есть.
type hook struct {
	SessionID     string  `json:"session_id"`
	HookEventName string  `json:"hook_event_name"`
	Cwd           string  `json:"cwd"`
	ToolName      *string `json:"tool_name"`
	ToolUseID     *string `json:"tool_use_id"`
	Prompt        *string `json:"prompt"`
}

// Parse разбирает тело хука. raw сохраняется без изменений в Event.Payload.
// Неизвестный hook_event_name и неизвестные поля не ошибка.
func Parse(raw []byte, receivedAt time.Time) (ingest.Event, error) {
	var h hook
	if err := json.Unmarshal(raw, &h); err != nil {
		return ingest.Event{}, errors.Join(ErrInvalidPayload, err)
	}
	if hasNUL(raw) {
		return ingest.Event{}, errors.Join(ErrInvalidPayload, errors.New("NUL character (\\u0000) is not supported"))
	}
	if h.SessionID == "" || h.HookEventName == "" {
		return ingest.Event{}, errors.Join(ErrInvalidPayload, errors.New("session_id and hook_event_name are required"))
	}
	return ingest.Event{
		SessionID:     h.SessionID,
		HookEventName: h.HookEventName,
		Cwd:           h.Cwd,
		ToolName:      emptyToNil(h.ToolName),
		ToolUseID:     emptyToNil(h.ToolUseID),
		Prompt:        emptyToNil(h.Prompt),
		Payload:       raw,
		ReceivedAt:    receivedAt.UTC(),
	}, nil
}

// emptyToNil: пустая строка это «нет значения»: иначе пустой tool_use_id попал бы в ключ
// идемпотентности, а пустой prompt занял бы task_text навсегда.
func emptyToNil(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

// hasNUL ищет U+0000 в любой строке или ключе JSON: Postgres не принимает его ни в jsonb, ни в text.
// Экранированный обратный слеш перед u0000 (`\\u0000`) это обычный текст и ошибкой не считается.
func hasNUL(raw []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err != nil {
			return false // конец входа; невалидный JSON отсеивает json.Unmarshal в Parse
		}
		if s, ok := tok.(string); ok && strings.IndexByte(s, 0) >= 0 {
			return true
		}
	}
}
