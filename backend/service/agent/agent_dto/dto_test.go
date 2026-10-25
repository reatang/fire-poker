package agent_dto

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSendMessageInputJSONTags(t *testing.T) {
	payload, err := json.Marshal(SendMessageInput{
		SessionID:        7,
		Content:          "hello",
		ProviderID:       3,
		ModelName:        "gpt-test",
		EnabledUserTools: []string{"web_fetch", "code_exec"},
	})
	if err != nil {
		t.Fatalf("marshal send message input: %v", err)
	}

	got := string(payload)
	expectedFragments := []string{
		`"session_id":7`,
		`"content":"hello"`,
		`"provider_id":3`,
		`"model_name":"gpt-test"`,
		`"enabled_user_tools":["web_fetch","code_exec"]`,
	}

	for _, fragment := range expectedFragments {
		if !containsJSONFragment(got, fragment) {
			t.Fatalf("expected JSON payload to contain %s, got %s", fragment, got)
		}
	}
}

// TestModelSelectionInputsCarryNoCredentials guards against credentials leaking back into frontend-facing DTOs.
func TestModelSelectionInputsCarryNoCredentials(t *testing.T) {
	inputs := map[string]any{
		"SendMessageInput":      SendMessageInput{ProviderID: 1, ModelName: "m"},
		"GenerateTitleInput":    GenerateTitleInput{ProviderID: 1, ModelName: "m"},
		"SpawnTaskSessionInput": SpawnTaskSessionInput{ProviderID: 1, ModelName: "m"},
	}
	for name, input := range inputs {
		payload, err := json.Marshal(input)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		for _, forbidden := range []string{`"api_key"`, `"base_url"`, `"provider_type"`} {
			if strings.Contains(string(payload), forbidden) {
				t.Fatalf("%s must not carry %s, got %s", name, forbidden, payload)
			}
		}
	}
}

func TestListSessionsOutputJSONTags(t *testing.T) {
	payload, err := json.Marshal(ListSessionsOutput{
		Sessions: []SessionItem{
			{
				ID:      1,
				Title:   "Tea Chat",
				Starred: true,
				Status:  "idle",
				Created: "2026-05-15T00:00:00Z",
				Updated: "2026-05-15T00:00:01Z",
			},
		},
		NextCursor: 1,
		HasMore:    true,
	})
	if err != nil {
		t.Fatalf("marshal list sessions output: %v", err)
	}

	got := string(payload)
	expectedFragments := []string{
		`"sessions":[`,
		`"id":1`,
		`"title":"Tea Chat"`,
		`"starred":true`,
		`"status":"idle"`,
		`"created":"2026-05-15T00:00:00Z"`,
		`"updated":"2026-05-15T00:00:01Z"`,
		`"next_cursor":1`,
		`"has_more":true`,
	}

	for _, fragment := range expectedFragments {
		if !containsJSONFragment(got, fragment) {
			t.Fatalf("expected JSON payload to contain %s, got %s", fragment, got)
		}
	}
}

func containsJSONFragment(payload string, fragment string) bool {
	return len(payload) >= len(fragment) && func() bool {
		for i := 0; i+len(fragment) <= len(payload); i++ {
			if payload[i:i+len(fragment)] == fragment {
				return true
			}
		}
		return false
	}()
}
