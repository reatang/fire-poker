package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/id/prompt_id"
	"github.com/reatang/fire-poker/backend/pkg/prompt"
	"github.com/reatang/fire-poker/backend/service/agent/agent_dto"
)

func TestGenerateTitleUsesCompleteFirstQuestionAndAnswer(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	var request struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"  首轮问题与回答生成简短会话标题  "}}]}`))
	}))
	defer server.Close()

	stor := newTestStorage(t)
	svc := NewAgent(stor)
	provider, err := stor.CreateProvider(data_models.Provider{
		ProviderName: "test", ProviderType: "openai_compatibility", BaseUrl: server.URL, ApiKey: "stored-key", Enable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := stor.CreateSession(data_models.Session{Title: "New Chat"})
	if err != nil {
		t.Fatal(err)
	}
	question := strings.Repeat("问题内容", 80) + "问题结尾"
	answer := strings.Repeat("回答内容", 80) + "回答结尾"
	rows := []data_models.Message{{Role: "user", ContentType: "text", Content: question}}
	for range 30 {
		rows = append(rows,
			data_models.Message{Role: "assistant", ContentType: "tool_call", Content: "hidden tool call"},
			data_models.Message{Role: "tool", ContentType: "tool_result", Content: "hidden tool result"},
		)
	}
	rows = append(rows,
		data_models.Message{Role: "assistant", ContentType: "thinking", Content: "hidden reasoning"},
		data_models.Message{Role: "assistant", ContentType: "text", Content: answer},
		data_models.Message{Role: "user", ContentType: "text", Content: "later question"},
		data_models.Message{Role: "assistant", ContentType: "text", Content: "later answer"},
	)
	for _, row := range rows {
		row.SessionID = session.ID
		if _, err := stor.CreateMessage(row); err != nil {
			t.Fatal(err)
		}
	}

	out, err := svc.GenerateTitle(context.Background(), agent_dto.GenerateTitleInput{
		SessionID: session.ID, ProviderID: provider.ID, ModelName: "gpt-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(request.Messages) != 1 {
		t.Fatalf("expected one title prompt, got %+v", request)
	}
	content := request.Messages[0].Content
	for _, want := range []string{"user: " + question, "assistant: " + answer, "10 characters"} {
		if !strings.Contains(content, want) {
			t.Errorf("title prompt is missing %q: %s", want, content)
		}
	}
	for _, unwanted := range []string{"hidden tool call", "hidden tool result", "hidden reasoning", "later question", "later answer"} {
		if strings.Contains(content, unwanted) {
			t.Errorf("title prompt includes %q", unwanted)
		}
	}
	wantTitle := "首轮问题与回答生成简"
	if out.Title != wantTitle {
		t.Errorf("expected title limited to ten characters, got %q", out.Title)
	}
	saved, err := stor.GetSession(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Title != wantTitle {
		t.Errorf("expected persisted short title, got %q", saved.Title)
	}
}

func TestBuildTitlePreviewStopsAtNextQuestion(t *testing.T) {
	got := buildTitlePreview([]data_models.Message{
		{Role: "user", ContentType: "text", Content: "first question"},
		{Role: "assistant", ContentType: "error", Content: "failed"},
		{Role: "user", ContentType: "text", Content: "second question"},
		{Role: "assistant", ContentType: "text", Content: "second answer"},
	})
	if got != "user: first question\n" {
		t.Fatalf("expected only first question when its answer is missing, got %q", got)
	}
}

func TestBuildTitlePromptFallsBackToDefault(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	prompt.Register(prompt_id.GenChatTitle, "")
	t.Cleanup(func() { prompt.Register(prompt_id.GenChatTitle, prompt.DefaultGenChatTitlePrompt) })
	got := buildTitlePrompt("user: question\nassistant: answer")
	if !strings.Contains(got, "10 characters") || !strings.Contains(got, "first answer") {
		t.Fatalf("expected first-turn instructions and ten-character limit, got %q", got)
	}
}

func TestNormalizeGeneratedTitle(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"empty", " \n\t ", "New Chat"},
		{"quotes", "  “简短标题”  ", "简短标题"},
		{"lines", "first\n  title", "first titl"},
		{"ten characters", "一二三四五六七八九十", "一二三四五六七八九十"},
		{"unicode", "标题😀一二三四五六七八", "标题😀一二三四五六七"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeGeneratedTitle(tc.input); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
