package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/reatang/fire-poker/backend/models/data_models"
	pkgAgent "github.com/reatang/fire-poker/backend/pkg/agent"
	"github.com/reatang/fire-poker/backend/pkg/i18n"
	"github.com/reatang/fire-poker/backend/pkg/id/prompt_id"
	"github.com/reatang/fire-poker/backend/pkg/ierror"
	"github.com/reatang/fire-poker/backend/pkg/prompt"
	"github.com/reatang/fire-poker/backend/service/agent/agent_dto"
	"github.com/reatang/fire-poker/backend/storage"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestStorage(t *testing.T) *storage.Storage {
	t.Helper()

	// Each helper call owns its database; background tasks from another test
	// must not share tables or locks with this one.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite connection: %v", err)
	}
	// A private in-memory database belongs to one connection.
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	stor, err := storage.NewStorageFromDB(db)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	return stor
}

func TestCreateSessionAndListSessions(t *testing.T) {
	svc := NewAgent(newTestStorage(t))

	created, err := svc.CreateSession(context.Background(), agent_dto.CreateSessionInput{})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if created.SessionID == 0 {
		t.Fatal("expected session id to be set")
	}
	if created.Title != "New Chat" {
		t.Fatalf("expected default title, got %q", created.Title)
	}

	listed, err := svc.ListSessions(context.Background(), agent_dto.ListSessionsInput{Limit: 10})
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(listed.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(listed.Sessions))
	}
	if listed.Sessions[0].ID != created.SessionID {
		t.Fatalf("expected listed session id %d, got %d", created.SessionID, listed.Sessions[0].ID)
	}
}

func TestSpawnTaskSessionCreatesTaskSession(t *testing.T) {
	svc := NewAgent(newTestStorage(t))

	out, err := svc.SpawnTaskSession(context.Background(), agent_dto.SpawnTaskSessionInput{
		Title:       "bg test",
		UserMessage: "hi",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.SessionID == 0 {
		t.Fatal("expected non-zero session id")
	}

	session, err := svc.manager.Storage().GetSession(out.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if session.Kind != "task" {
		t.Fatalf("expected task kind, got %q", session.Kind)
	}
}

func TestLoadSessionMessagesMapsStorageMessages(t *testing.T) {
	stor := newTestStorage(t)
	session, err := stor.CreateSession(data_models.Session{Title: "Test", Status: "idle"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err = stor.CreateMessage(data_models.Message{
		SessionID:   session.ID,
		Role:        "assistant",
		ContentType: "text",
		Content:     "hello",
		ModelName:   "gpt-test",
		TokensIn:    10,
		TokensOut:   20,
		Extra:       "extra",
	})
	if err != nil {
		t.Fatalf("create message: %v", err)
	}

	svc := NewAgent(stor)
	result, err := svc.LoadSessionMessages(context.Background(), agent_dto.LoadSessionMessagesInput{
		SessionID: session.ID,
		Offset:    0,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("load session messages: %v", err)
	}

	if result.Total != 1 {
		t.Fatalf("expected total 1, got %d", result.Total)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
	if result.Messages[0].Content != "hello" {
		t.Fatalf("expected content hello, got %q", result.Messages[0].Content)
	}
}

func TestMarkSessionReadPersistsIdleStatus(t *testing.T) {
	stor := newTestStorage(t)
	session, err := stor.CreateSession(data_models.Session{Title: "Test", Status: "done-unread"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	svc := NewAgent(stor)
	if _, err := svc.MarkSessionRead(context.Background(), agent_dto.MarkSessionReadInput{SessionID: session.ID}); err != nil {
		t.Fatalf("mark session read: %v", err)
	}

	updated, err := stor.GetSession(session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if updated.Status != "idle" {
		t.Fatalf("expected idle status, got %q", updated.Status)
	}
}

func TestHelpersConvertModelsToDTOs(t *testing.T) {
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	session := data_models.Session{
		OrmModel: data_models.OrmModel{
			ID:        7,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Minute),
		},
		Title:   "Tea",
		Starred: true,
		Status:  "idle",
	}
	msg := data_models.Message{
		OrmModel: data_models.OrmModel{
			ID:        9,
			CreatedAt: now,
		},
		SessionID:   7,
		Role:        "assistant",
		ContentType: "text",
		Content:     "reply",
		ModelName:   "gpt-test",
		AgentName:   "main",
		TokensIn:    1,
		TokensOut:   2,
		Extra:       "meta",
	}

	sessionItem := toSessionItem(session)
	if sessionItem.ID != 7 || sessionItem.Title != "Tea" || !sessionItem.Starred {
		t.Fatalf("unexpected session item: %+v", sessionItem)
	}

	messageItem := toMessageItem(msg)
	if messageItem.ID != 9 || messageItem.Content != "reply" || messageItem.ModelName != "gpt-test" {
		t.Fatalf("unexpected message item: %+v", messageItem)
	}
}

func TestBuildTitlePromptUsesRegisteredPromptID(t *testing.T) {
	prompt.Register(prompt_id.GenChatTitle, "CUSTOM TITLE PROMPT")

	got := buildTitlePrompt("user: hello")
	want := "CUSTOM TITLE PROMPT\n\nuser: hello"

	if got != want {
		t.Fatalf("unexpected title prompt: got %q want %q", got, want)
	}
}

// TestGenerateTitleUsesStoredCredentialsWithoutTools proves titles come from one tool-free call
// authenticated with the provider's stored key rather than anything supplied by the caller.
func TestGenerateTitleUsesStoredCredentialsWithoutTools(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"  秋日散文  "}}]}`))
	}))
	defer server.Close()

	stor := newTestStorage(t)
	svc := NewAgent(stor)
	provider, err := stor.CreateProvider(data_models.Provider{
		ProviderName: "test", ProviderType: "openai_compatibility", BaseUrl: server.URL, ApiKey: "stored-key", Enable: true,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	session, err := stor.CreateSession(data_models.Session{Title: "New Chat"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := stor.CreateMessage(data_models.Message{SessionID: session.ID, Role: "user", ContentType: "text", Content: "写一篇秋天的散文"}); err != nil {
		t.Fatalf("create message: %v", err)
	}

	out, err := svc.GenerateTitle(context.Background(), agent_dto.GenerateTitleInput{
		SessionID: session.ID, ProviderID: provider.ID, ModelName: "gpt-test",
	})
	if err != nil {
		t.Fatalf("generate title: %v", err)
	}
	if out.Title != "秋日散文" {
		t.Fatalf("expected trimmed model title, got %q", out.Title)
	}
	if gotAuth != "Bearer stored-key" {
		t.Fatalf("expected stored API key, got %q", gotAuth)
	}
	if _, ok := gotBody["tools"]; ok {
		t.Fatalf("title generation must not bind tools, got %v", gotBody["tools"])
	}
	saved, err := stor.GetSession(session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if saved.Title != "秋日散文" {
		t.Fatalf("expected persisted title, got %q", saved.Title)
	}
}

// TestGenerateTitleForImageOnlyQuestion covers a first turn whose question is just an image and
// whose reply text sits behind thinking and tool rows: the preview must still reach the model.
func TestGenerateTitleForImageOnlyQuestion(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"猫咪照片"}}]}`))
	}))
	defer server.Close()

	stor := newTestStorage(t)
	svc := NewAgent(stor)
	provider, err := stor.CreateProvider(data_models.Provider{
		ProviderName: "test", ProviderType: "openai_compatibility", BaseUrl: server.URL, ApiKey: "stored-key", Enable: true,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	session, err := stor.CreateSession(data_models.Session{Title: "New Chat"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	attachments, err := pkgAgent.MarshalAttachments([]pkgAgent.Attachment{{Name: "cat.png", Path: "/tmp/cat.png", Mime: "image/png", Kind: "image"}})
	if err != nil {
		t.Fatalf("marshal attachments: %v", err)
	}
	rows := []data_models.Message{
		{SessionID: session.ID, Role: "user", ContentType: "text", Attachments: attachments},
		{SessionID: session.ID, Role: "assistant", ContentType: "thinking", Content: "看看图片"},
		{SessionID: session.ID, Role: "assistant", ContentType: "tool_call", Content: "{}"},
		{SessionID: session.ID, Role: "tool", ContentType: "tool_result", Content: "ok"},
		{SessionID: session.ID, Role: "assistant", ContentType: "text", Content: "这是一只橘猫。"},
	}
	for _, row := range rows {
		if _, err := stor.CreateMessage(row); err != nil {
			t.Fatalf("create message: %v", err)
		}
	}

	out, err := svc.GenerateTitle(context.Background(), agent_dto.GenerateTitleInput{
		SessionID: session.ID, ProviderID: provider.ID, ModelName: "gpt-test",
	})
	if err != nil {
		t.Fatalf("generate title: %v", err)
	}
	if out.Title != "猫咪照片" {
		t.Fatalf("expected model title, got %q", out.Title)
	}
	body, _ := json.Marshal(gotBody)
	for _, want := range []string{"cat.png", "这是一只橘猫"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("title prompt should mention %q, got %s", want, body)
		}
	}
}

// TestGenerateTitleFallsBackForUnknownProvider keeps the previous non-fatal behavior on bad input.
func TestGenerateTitleFallsBackForUnknownProvider(t *testing.T) {
	stor := newTestStorage(t)
	svc := NewAgent(stor)
	session, err := stor.CreateSession(data_models.Session{Title: "New Chat"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := stor.CreateMessage(data_models.Message{SessionID: session.ID, Role: "user", ContentType: "text", Content: "hello"}); err != nil {
		t.Fatalf("create message: %v", err)
	}

	out, err := svc.GenerateTitle(context.Background(), agent_dto.GenerateTitleInput{
		SessionID: session.ID, ProviderID: 404, ModelName: "gpt-test",
	})
	if err != nil {
		t.Fatalf("generate title: %v", err)
	}
	if out.Title != "New Chat" {
		t.Fatalf("expected fallback title, got %q", out.Title)
	}
}

func TestConvertAttachmentsRejectsTooMany(t *testing.T) {
	in := make([]agent_dto.AttachmentInput, maxAttachmentCount+1)
	for i := range in {
		in[i] = agent_dto.AttachmentInput{Path: "/tmp/x"}
	}
	_, err := convertAttachments(in)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), ierror.ErrAgentTooManyAttachments.Msg()) {
		t.Fatalf("error = %v", err)
	}
}

func TestConvertAttachmentsRejectsMissingFile(t *testing.T) {
	_, err := convertAttachments([]agent_dto.AttachmentInput{
		{Path: "/definitely/not/here.png"},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), ierror.ErrAgentAttachmentNotFound.Msg()) {
		t.Fatalf("error = %v", err)
	}
}

func TestConvertAttachmentsRejectsOversize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := f.Truncate(maxAttachmentBytes + 1); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	_ = f.Close()

	_, err = convertAttachments([]agent_dto.AttachmentInput{{Path: path}})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), ierror.ErrAgentAttachmentSize.Msg()) {
		t.Fatalf("error = %v", err)
	}
}

func TestConvertAttachmentsNormalizesAndPasses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.go")
	if err := os.WriteFile(path, []byte("package x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := convertAttachments([]agent_dto.AttachmentInput{{Path: path}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "hello.go" || got[0].Kind != "file" {
		t.Fatalf("normalize failed: %+v", got)
	}
}

// iErrorMsg extracts the localized message the frontend displays.
func iErrorMsg(t *testing.T, err error) string {
	t.Helper()
	var iErr *ierror.IError
	if !errors.As(err, &iErr) {
		t.Fatalf("expected *ierror.IError, got %T: %v", err, err)
	}
	return iErr.Msg
}

// TestAgentErrorPrefersSpecificMessages proves known causes surface their own localized message.
func TestAgentErrorPrefersSpecificMessages(t *testing.T) {
	cases := []struct {
		err  error
		want ierror.ErrorCode
	}{
		{fmt.Errorf("resolve model: %w", pkgAgent.ErrProviderDisabled), ierror.ErrAgentProviderUnavail},
		{pkgAgent.ErrSessionBusy, ierror.ErrAgentSessionBusy},
		{storage.ErrSystemPromptLocked, ierror.ErrAgentPromptLocked},
		{storage.ErrSessionToolsLocked, ierror.ErrAgentToolsLocked},
		{errors.New("disk full"), ierror.ErrAgentSendMessage},
	}
	for _, tc := range cases {
		if got := iErrorMsg(t, agentError(ierror.ErrAgentSendMessage, tc.err)); got != tc.want.Msg() {
			t.Errorf("%v: msg = %q, want %q", tc.err, got, tc.want.Msg())
		}
	}
}

// TestSendMessageReportsUnavailableProvider replaces the generic "send failed" with the actual cause.
func TestSendMessageReportsUnavailableProvider(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	svc := NewAgent(newTestStorage(t))
	created, err := svc.CreateSession(context.Background(), agent_dto.CreateSessionInput{})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err = svc.SendMessage(context.Background(), agent_dto.SendMessageInput{SessionID: created.SessionID, Content: "hi", ProviderID: 404, ModelName: "m"})
	if got := iErrorMsg(t, err); got != ierror.ErrAgentProviderUnavail.Msg() {
		t.Fatalf("msg = %q, want provider-unavailable message", got)
	}
}

// TestPromptAndForkErrorsAreLocalized covers service methods that previously returned bare Chinese errors.
func TestPromptAndForkErrorsAreLocalized(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	svc := NewAgent(newTestStorage(t))
	created, err := svc.CreateSession(context.Background(), agent_dto.CreateSessionInput{})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err = svc.UpdateSessionPrompt(context.Background(), agent_dto.UpdateSessionPromptInput{SessionID: created.SessionID, System: "  "})
	if got := iErrorMsg(t, err); got != ierror.ErrAgentPromptEmpty.Msg() {
		t.Fatalf("empty prompt msg = %q", got)
	}
	_, err = svc.UpdateSessionPrompt(context.Background(), agent_dto.UpdateSessionPromptInput{SessionID: created.SessionID, System: "role", Name: strings.Repeat("名", maxPromptNameRunes+1)})
	if got := iErrorMsg(t, err); got != ierror.ErrAgentPromptNameTooLong.Msg() {
		t.Fatalf("long name msg = %q", got)
	}
	_, err = svc.ForkSession(context.Background(), agent_dto.ForkSessionInput{SessionID: created.SessionID})
	if got := iErrorMsg(t, err); got != ierror.ErrAgentForkTarget.Msg() {
		t.Fatalf("fork target msg = %q", got)
	}
}

// TestGetSessionPromptNamesFollowLocale keeps fallback prompt labels in the active language.
func TestGetSessionPromptNamesFollowLocale(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	i18n.SetCurrentLocale(i18n.LocaleEnUS)
	t.Cleanup(func() { i18n.SetCurrentLocale(i18n.DefaultLocale) })
	svc := NewAgent(newTestStorage(t))
	created, err := svc.CreateSession(context.Background(), agent_dto.CreateSessionInput{})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	out, err := svc.GetSessionPrompt(context.Background(), agent_dto.SessionPromptInput{SessionID: created.SessionID})
	if err != nil {
		t.Fatalf("get prompt: %v", err)
	}
	if out.Name != "Default system prompt" {
		t.Fatalf("expected English default label, got %q", out.Name)
	}
}
