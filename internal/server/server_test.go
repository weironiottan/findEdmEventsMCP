package server

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weironiottan/findEdmEventsMCP/internal/events"
)

func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()

	ss, err := New(events.NewMemoryStore(events.SampleEvents())).Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func TestListTools(t *testing.T) {
	cs := connect(t)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	for _, want := range []string{"search_events", "get_event", "list_clubs"} {
		if !got[want] {
			t.Errorf("tool %q not registered", want)
		}
	}
}

// TestSchemasUseSingleTypes guards portability: clients such as Gemini
// reject schemas whose "type" is an array like ["null","array"].
func TestSchemasUseSingleTypes(t *testing.T) {
	cs := connect(t)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch v := v.(type) {
		case map[string]any:
			if _, ok := v["type"].([]any); ok {
				t.Errorf("%s: type is an array: %v", path, v["type"])
			}
			for k, child := range v {
				walk(path+"."+k, child)
			}
		case []any:
			for i, child := range v {
				walk(fmt.Sprintf("%s[%d]", path, i), child)
			}
		}
	}
	for _, tool := range res.Tools {
		b, _ := json.Marshal(tool)
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		walk(tool.Name, m)
	}
}

// call invokes a tool and decodes its structured output into out.
func call(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any, out any) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %v", res.Content)
	}
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatal(err)
	}
}

func TestSearchEvents(t *testing.T) {
	cs := connect(t)
	var out SearchEventsOutput
	call(t, cs, "search_events", map[string]any{"club": "Hakkasan", "from": "2026-01-01"}, &out)
	if len(out.Events) != 1 || out.Events[0].ID != "sample-1" {
		t.Errorf("got %+v, want only sample-1", out.Events)
	}
	if out.Truncated {
		t.Error("truncated = true, want false")
	}
}

func TestSearchEventsTruncated(t *testing.T) {
	cs := connect(t)
	var out SearchEventsOutput
	call(t, cs, "search_events", map[string]any{"from": "2026-01-01", "limit": 1}, &out)
	if len(out.Events) != 1 {
		t.Errorf("got %d events, want 1", len(out.Events))
	}
	if !out.Truncated {
		t.Error("truncated = false, want true")
	}
}

func TestListClubs(t *testing.T) {
	cs := connect(t)
	var out ListClubsOutput
	call(t, cs, "list_clubs", map[string]any{}, &out)
	// Sample events are dated Oct 2026 and Jan 2027, so only the second is
	// guaranteed to still be upcoming; just check the names are well-formed.
	for _, c := range out.Clubs {
		if c.Name == "" || c.Events < 1 {
			t.Errorf("bad club entry %+v", c)
		}
	}
}

func TestGetEventNotFound(t *testing.T) {
	cs := connect(t)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_event",
		Arguments: map[string]any{"id": "nope"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Error("expected tool error for unknown ID")
	}
}
