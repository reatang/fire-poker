package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/reatang/fire-poker/backend/models/data_models"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// Exercise the real runner and OpenAI adapter: cancellation must persist the
// streamed prefix, then the next HTTP request must contain that prefix and
// the session's fixed system instruction, without changing user content.
func TestInterruptedTurnResumesFromStoredHistory(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	manager := NewManager(newTestStorage(t))
	s, err := manager.Storage().CreateSession(data_models.Session{Title: "resume", SystemPrompt: "只使用中文回答"})
	if err != nil {
		t.Fatal(err)
	}
	type request struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	requests := make(chan request, 5)
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		requests <- req
		w.Header().Set("Content-Type", "text/event-stream")
		if count.Add(1) == 1 {
			fmt.Fprint(w, "data: {\"id\":\"first\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"秋天的风吹过田野，\"}}]}\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		fmt.Fprint(w, "data: {\"id\":\"next\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"稻谷染上金黄。\"}}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	p := SendMessageParams{SessionID: s.ID, Content: "写一篇秋天的散文", BaseURL: server.URL + "/v1", ModelName: "test", ProviderType: "openai_compatibility"}
	h := NewChatHandler(manager)
	if err := h.SendMessage(ctx, p); err != nil {
		t.Fatal(err)
	}
	select {
	case <-requests:
	case <-ctx.Done():
		t.Fatal("model request timed out")
	}
	// Wait until the real stream consumer receives the prefix (not merely the server writing it).
	// Its channel blocks once delivered, allowing deterministic cancellation in the unit test below;
	// here a short stream interval covers the actual HTTP reader and SDK event forwarding.
	time.Sleep(150 * time.Millisecond)
	if err := manager.Streams().StopAndWait(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	msgs, err := manager.Storage().ListMessagesForSession(s.ID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[1].Content != "秋天的风吹过田野，" || msgs[1].Extra != "interrupted" {
		t.Fatalf("partial answer not saved: %+v", msgs)
	}
	p.Content = "继续"
	if err := h.SendMessage(ctx, p); err != nil {
		t.Fatal(err)
	}
	var next request
	select {
	case next = <-requests:
	case <-ctx.Done():
		t.Fatal("continuation request timed out")
	}
	wire, _ := json.Marshal(next)
	if !strings.Contains(string(wire), "秋天的风吹过田野，") {
		t.Fatalf("missing prefix in actual request: %s", wire)
	}
	foundSystem, foundUser := false, false
	for _, m := range next.Messages {
		if m.Role == "system" && strings.Contains(string(m.Content), "只使用中文回答") {
			foundSystem = true
		}
		if m.Role == "user" && string(m.Content) == `"继续"` {
			foundUser = true
		}
	}
	if !foundSystem || !foundUser {
		t.Fatalf("incorrect instruction role or user content: %s", wire)
	}
	deadline := time.Now().Add(3 * time.Second)
	for manager.Streams().IsActive(s.ID) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if manager.Streams().IsActive(s.ID) {
		t.Fatal("continuation did not finish")
	}
}

func TestStopPersistsThinkingAndTextBeforeReturning(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	m := NewManager(newTestStorage(t))
	s, err := m.Storage().CreateSession(data_models.Session{Title: "stop"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, _, err := m.Streams().TryStart(context.Background(), s.ID)
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan *event.Event)
	go NewChatHandler(m).consumeEvents(ctx, s.ID, "test", events)
	events <- &event.Event{Response: &model.Response{Choices: []model.Choice{{Delta: model.Message{Content: "已输出的正文", ReasoningContent: "思考片段"}}}}}
	// This second send completes only after the first delta was processed.
	events <- nil
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := m.Streams().StopAndWait(stopCtx, s.ID); err != nil {
		t.Fatal(err)
	}
	msgs, err := m.Storage().ListMessagesForSession(s.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].ContentType != "thinking" || msgs[1].Content != "已输出的正文" || msgs[1].Extra != "interrupted" {
		t.Fatalf("lost partial output: %+v", msgs)
	}
	if m.Streams().IsActive(s.ID) {
		t.Fatal("stream still active after stop")
	}
	h, err := NewChatHandler(m).loadSessionHistory(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 1 || h[0].Content != "已输出的正文" {
		t.Fatalf("incorrect continuation history: %+v", h)
	}
	close(events)
}

func TestPerSessionInstructionsDoNotLeakThroughCachedRunner(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	m := NewManager(newTestStorage(t))
	r := newDelayedRunner()
	h := NewChatHandler(m)
	for _, instruction := range []string{"你是架构顾问", "你是写作教练", ""} {
		s, err := m.Storage().CreateSession(data_models.Session{Title: "isolated", SystemPrompt: instruction})
		if err != nil {
			t.Fatal(err)
		}
		injectSessionRunner(t, m, s.ID, "https://example.test", "model", "", r)
		if err := h.SendMessage(context.Background(), SendMessageParams{SessionID: s.ID, Content: "你好", BaseURL: "https://example.test", ModelName: "model"}); err != nil {
			t.Fatal(err)
		}
		expected := instruction
		if expected == "" {
			expected = DefaultSystemPrompt()
		}
		if r.lastRunOpts.Instruction != expected || r.lastMessage.Content != "你好" {
			t.Fatalf("instruction leaked or became user text: %+v", r.lastRunOpts)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		if err := m.Streams().StopAndWait(ctx, s.ID); err != nil {
			cancel()
			t.Fatal(err)
		}
		cancel()
	}
	close(r.events)
}
