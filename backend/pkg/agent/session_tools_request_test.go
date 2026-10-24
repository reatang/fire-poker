package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/reatang/fire-poker/backend/pkg/agent/toolkit"
	pkgProvider "github.com/reatang/fire-poker/backend/pkg/provider"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// Verify the final OpenAI request and runner isolation using concrete session selections.
func TestSessionSelectionControlsLLMRequestTools(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	requests := make(chan []string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		names := make([]string, 0, len(req.Tools))
		for _, current := range req.Tools {
			names = append(names, current.Function.Name)
		}
		requests <- names
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"test\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"}}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	manager := NewManager(nil)
	for _, ids := range [][]string{{"web_search"}, {}, {"code_exec"}, {"datetime", "web_search"}, {"file_read"}, {"shell"}, {toolkit.WindowsShellTool, toolkit.MacOSShellTool, toolkit.LinuxShellTool}} {
		t.Run(fmt.Sprintf("selection_%v", ids), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			r, err := manager.GetOrCreateRunner(server.URL+"/v1", "test-key", "test", pkgProvider.OpenAiCompatibility, ids, "user", 1)
			if err != nil {
				t.Fatal(err)
			}
			events, err := r.Run(ctx, "local", fmt.Sprintf("selection_%v", ids), model.NewUserMessage("hello"))
			if err != nil {
				t.Fatal(err)
			}
			for ev := range events {
				if ev.Response != nil && ev.Response.Error != nil {
					t.Fatal(ev.Response.Error)
				}
			}
			select {
			case got := <-requests:
				want := ids
				if len(ids) > 0 && (ids[0] == "shell" || toolkit.IsShellTool(ids[0])) {
					want = []string{toolkit.PlatformShellToolID()}
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("request tools = %v, selected = %v", got, ids)
				}
			case <-ctx.Done():
				t.Fatal("model request timed out")
			}
		})
	}
}
