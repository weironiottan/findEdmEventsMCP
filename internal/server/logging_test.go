package server

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weironiottan/findEdmEventsMCP/internal/events"
)

// logCall makes a single tool call against a server with LogCalls installed
// and returns the log entry for it.
func logCall(t *testing.T, tool string, args map[string]any) map[string]any {
	t.Helper()
	ctx := context.Background()
	var buf bytes.Buffer
	srv := New(events.NewMemoryStore(events.SampleEvents()))
	srv.AddReceivingMiddleware(LogCalls(NewCloudLogger(&buf)))

	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Protocol errors are expected for some cases; the log entry is what's checked.
	cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})

	var entry map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("log line is not JSON: %q", line)
		}
		if e["method"] == "tools/call" {
			entry = e
		}
	}
	if entry == nil {
		t.Fatalf("no tools/call entry in logs:\n%s", buf.String())
	}
	return entry
}

func TestLogCallsToolError(t *testing.T) {
	entry := logCall(t, "get_event", map[string]any{"id": "nope"})
	if entry["severity"] != "WARNING" || entry["message"] != "mcp request" || entry["tool"] != "get_event" || entry["isError"] != true {
		t.Errorf("unexpected entry: %v", entry)
	}
	if args, _ := entry["args"].(map[string]any); args["id"] != "nope" {
		t.Errorf("args = %v, want id=nope", entry["args"])
	}
	if msg, _ := entry["error"].(string); !strings.Contains(msg, "not found") {
		t.Errorf("error = %q, want it to mention not found", msg)
	}
}

func TestLogCallsProtocolError(t *testing.T) {
	entry := logCall(t, "no_such_tool", nil)
	if entry["severity"] != "ERROR" || entry["tool"] != "no_such_tool" {
		t.Errorf("unexpected entry: %v", entry)
	}
	if msg, _ := entry["error"].(string); !strings.Contains(msg, "unknown tool") {
		t.Errorf("error = %q, want it to mention unknown tool", msg)
	}
}
