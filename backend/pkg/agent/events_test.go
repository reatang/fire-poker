package agent

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// chatEventUse matches a reference to a registered chat event constant.
var chatEventUse = regexp.MustCompile(`event_id\.Agent(SessionStatus|SessionSpawned|StreamChunk|StreamToolCall|StreamToolResult|StreamConfirmRequest|StreamDone|StreamError)\b`)

// TestChatEventsAreEmittedOnlyThroughTypedHelpers guards the registered payload types: Wails cancels
// an event whose payload type differs, so an untyped Emit elsewhere would silently drop it.
func TestChatEventsAreEmittedOnlyThroughTypedHelpers(t *testing.T) {
	backendRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	allowed := filepath.Join(backendRoot, "pkg", "agent", "events.go")
	var offenders []string
	err = filepath.WalkDir(backendRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || path == allowed {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) == "event_id" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(content), "\n") {
			if chatEventUse.MatchString(line) {
				rel, _ := filepath.Rel(backendRoot, path)
				offenders = append(offenders, rel+":"+itoa(i+1)+": "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("chat events must be emitted via the typed helpers in pkg/agent/events.go:\n%s", strings.Join(offenders, "\n"))
	}
}

// TestChatEventPayloadsKeepWireFormat pins the JSON keys the frontend already parses,
// so moving from map payloads to typed structs changes nothing on the wire.
func TestChatEventPayloadsKeepWireFormat(t *testing.T) {
	cases := []struct {
		name    string
		payload any
		want    string
	}{
		{"status", SessionStatusEvent{SessionID: 1, Status: "loading"}, `{"sessionId":1,"status":"loading"}`},
		{"spawned", SessionSpawnedEvent{SessionID: 1, Title: "t", Kind: "task", Tags: []string{"task"}, UserMessage: "u"}, `{"sessionId":1,"title":"t","kind":"task","tags":["task"],"userMessage":"u"}`},
		{"chunk", StreamChunkEvent{SessionID: 1, Seq: 2, Delta: "d", Content: "c", ContentType: "text"}, `{"sessionId":1,"seq":2,"delta":"d","content":"c","contentType":"text"}`},
		{"tool call", StreamToolCallEvent{SessionID: 1, ToolName: "n", Args: "{}", Purpose: "p"}, `{"sessionId":1,"toolName":"n","args":"{}","purpose":"p"}`},
		{"tool result", StreamToolResultEvent{SessionID: 1, ToolName: "n", Result: "r"}, `{"sessionId":1,"toolName":"n","result":"r"}`},
		{"confirm", StreamConfirmRequestEvent{SessionID: 1, RequestID: "q", ToolName: "n", Args: "{}", Purpose: "p"}, `{"sessionId":1,"requestId":"q","toolName":"n","args":"{}","purpose":"p"}`},
		{"done", StreamDoneEvent{SessionID: 1, Interrupted: true, Usage: TokenUsage{Input: 3, Output: 4}}, `{"sessionId":1,"interrupted":true,"usage":{"input":3,"output":4}}`},
		{"error", StreamErrorEvent{SessionID: 1, Error: "e"}, `{"sessionId":1,"error":"e"}`},
	}
	for _, tc := range cases {
		got, err := json.Marshal(tc.payload)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s payload = %s, want %s", tc.name, got, tc.want)
		}
	}
}

// itoa formats a line number without pulling strconv into the assertion helper.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
