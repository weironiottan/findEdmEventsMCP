// Package server builds the MCP server and registers its tools.
package server

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weironiottan/findEdmEventsMCP/internal/events"
)

const (
	name    = "find-edm-events"
	version = "0.1.0"
)

// New returns an MCP server exposing the event tools backed by store.
func New(store events.Store) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: name, Version: version}, nil)
	h := &handlers{store: store}

	mcp.AddTool(s, &mcp.Tool{
		Name:         "search_events",
		OutputSchema: outputSchema[SearchEventsOutput](),
		Description: "Search upcoming EDM events by club, artist, and date range. Results are sorted by date. " +
			"If truncated is true there are more matches: raise limit or narrow the date range. " +
			"Call list_clubs first if unsure of a club's name.",
	}, h.searchEvents)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_event",
		Description: "Get a single EDM event by its ID.",
	}, h.getEvent)

	mcp.AddTool(s, &mcp.Tool{
		Name:         "list_clubs",
		OutputSchema: outputSchema[ListClubsOutput](),
		Description:  "List clubs that have upcoming events, with event counts. Use the names as the club filter in search_events.",
	}, h.listClubs)

	return s
}

type handlers struct {
	store events.Store
}

type SearchEventsInput struct {
	Club   string `json:"club,omitempty" jsonschema:"club name or partial name, e.g. hakkasan or marquee"`
	Artist string `json:"artist,omitempty" jsonschema:"artist name or partial name"`
	From   string `json:"from,omitempty" jsonschema:"earliest date, YYYY-MM-DD (default today)"`
	To     string `json:"to,omitempty" jsonschema:"latest date, YYYY-MM-DD"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of results (default 20)"`
}

type SearchEventsOutput struct {
	Events    []events.Event `json:"events"`
	Truncated bool           `json:"truncated" jsonschema:"true if more events matched than limit allowed"`
}

func (h *handlers) searchEvents(ctx context.Context, _ *mcp.CallToolRequest, in SearchEventsInput) (*mcp.CallToolResult, SearchEventsOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	// Fetch one extra so we can tell whether results were cut off.
	q := events.Query{Club: in.Club, Artist: in.Artist, Limit: limit + 1}
	var err error
	if q.From, err = parseDate(in.From); err != nil {
		return nil, SearchEventsOutput{}, fmt.Errorf("from: %w", err)
	}
	if q.From.IsZero() {
		q.From = today()
	}
	if q.To, err = parseDate(in.To); err != nil {
		return nil, SearchEventsOutput{}, fmt.Errorf("to: %w", err)
	}
	if !q.To.IsZero() {
		q.To = q.To.Add(24*time.Hour - time.Nanosecond) // include the whole end day
	}

	found, err := h.store.Search(ctx, q)
	if err != nil {
		return nil, SearchEventsOutput{}, err
	}
	out := SearchEventsOutput{Events: found}
	if len(found) > limit {
		out.Events, out.Truncated = found[:limit], true
	}
	if out.Events == nil {
		out.Events = []events.Event{}
	}
	return nil, out, nil
}

type ListClubsOutput struct {
	Clubs []events.Club `json:"clubs" jsonschema:"clubs with their number of upcoming events"`
}

func (h *handlers) listClubs(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ListClubsOutput, error) {
	clubs, err := h.store.Clubs(ctx, today())
	if err != nil {
		return nil, ListClubsOutput{}, err
	}
	return nil, ListClubsOutput{Clubs: clubs}, nil
}

// outputSchema infers T's schema like AddTool would, but declares top-level
// slice fields as plain "array" instead of ["null","array"]. The union form
// is rejected by single-type schema dialects (e.g. Gemini). Handlers must
// return empty slices rather than nil; the SDK validates output against
// this schema, so a nil would surface as an error.
func outputSchema[T any]() *jsonschema.Schema {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		panic(fmt.Sprintf("output schema for %T: %v", *new(T), err))
	}
	for _, p := range s.Properties {
		if slices.Equal(p.Types, []string{"null", "array"}) {
			p.Types, p.Type = nil, "array"
		}
	}
	return s
}

// today returns the local calendar date at midnight UTC, matching how
// EventDate is stored.
func today() time.Time {
	y, m, d := time.Now().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

type GetEventInput struct {
	ID string `json:"id" jsonschema:"event ID"`
}

type GetEventOutput struct {
	Event events.Event `json:"event"`
}

func (h *handlers) getEvent(ctx context.Context, _ *mcp.CallToolRequest, in GetEventInput) (*mcp.CallToolResult, GetEventOutput, error) {
	e, ok, err := h.store.Get(ctx, in.ID)
	if err != nil {
		return nil, GetEventOutput{}, err
	}
	if !ok {
		return nil, GetEventOutput{}, fmt.Errorf("event %q not found", in.ID)
	}
	return nil, GetEventOutput{Event: e}, nil
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.DateOnly, s)
}
