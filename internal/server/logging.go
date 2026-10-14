package server

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// LogCalls returns middleware that logs one line per incoming MCP request:
// the method, and for tool calls the tool name, arguments, and result.
//
// Tool failures come back to the client as a successful response with
// isError set (so the model can see them and retry), not as HTTP or
// JSON-RPC errors, so this is the only place they show up in the logs.
func LogCalls(logger *slog.Logger) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			start := time.Now()
			res, err := next(ctx, method, req)

			level := slog.LevelInfo
			attrs := []slog.Attr{
				slog.String("method", method),
				slog.Int64("durationMs", time.Since(start).Milliseconds()),
			}
			if r, ok := req.(*mcp.CallToolRequest); ok {
				attrs = append(attrs,
					slog.String("tool", r.Params.Name),
					slog.Any("args", r.Params.Arguments))
			}
			// On protocol errors res is a nil *CallToolResult, not a nil interface.
			if r, ok := res.(*mcp.CallToolResult); ok && r != nil && r.IsError {
				level = slog.LevelWarn
				attrs = append(attrs, slog.Bool("isError", true), slog.String("error", resultText(r)))
			}
			if err != nil {
				level = slog.LevelError
				attrs = append(attrs, slog.String("error", err.Error()))
			}
			logger.LogAttrs(ctx, level, "mcp request", attrs...)
			return res, err
		}
	}
}

// resultText joins the text content of a result, which for tool errors is
// the error message.
func resultText(r *mcp.CallToolResult) string {
	var parts []string
	for _, c := range r.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, t.Text)
		}
	}
	return strings.Join(parts, "; ")
}

// NewCloudLogger returns a JSON logger whose output Cloud Logging parses into
// structured entries: it expects "severity" and "message" rather than slog's
// "level" and "msg".
func NewCloudLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) > 0 {
				return a
			}
			switch a.Key {
			case slog.LevelKey:
				a.Key = "severity"
				if a.Value.Any().(slog.Level) == slog.LevelWarn {
					a.Value = slog.StringValue("WARNING")
				}
			case slog.MessageKey:
				a.Key = "message"
			}
			return a
		},
	}))
}
