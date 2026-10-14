// Package events defines the EDM event model and the store used to query it.
package events

import (
	"context"
	"sort"
	"strings"
	"time"
)

// Event is a single EDM event. Firestore tags match the stored field names.
type Event struct {
	ID             string `json:"id" firestore:"Id"`
	ClubName       string `json:"clubName" firestore:"ClubName"`
	ArtistName     string `json:"artistName" firestore:"ArtistName"`
	EventDate      string `json:"eventDate" firestore:"EventDate"` // RFC3339, e.g. 2026-10-22T00:00:00Z
	TicketURL      string `json:"ticketUrl,omitempty" firestore:"TicketUrl"`
	ArtistImageURL string `json:"artistImageUrl,omitempty" firestore:"ArtistImageUrl"`
}

// Query filters events. Zero-valued fields are ignored.
type Query struct {
	Club   string // case-insensitive substring of ClubName
	Artist string // case-insensitive substring of ArtistName
	From   time.Time
	To     time.Time
	Limit  int
}

// Club is a venue and how many events it has in the queried range.
type Club struct {
	Name   string `json:"name"`
	Events int    `json:"events"`
}

// Store is the source of event data.
type Store interface {
	Search(ctx context.Context, q Query) ([]Event, error)
	Get(ctx context.Context, id string) (Event, bool, error)
	// Clubs lists distinct clubs with events on or after from, sorted by name.
	Clubs(ctx context.Context, from time.Time) ([]Club, error)
}

// clubCounter tallies events per club.
type clubCounter map[string]int

func (c clubCounter) sorted() []Club {
	out := make([]Club, 0, len(c))
	for name, n := range c {
		out = append(out, Club{Name: name, Events: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// dateKey formats t the way EventDate is stored, so string comparison
// orders events by date.
func dateKey(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// matchesNames reports whether e satisfies the club and artist filters.
func matchesNames(e Event, q Query) bool {
	return containsFold(e.ClubName, q.Club) && containsFold(e.ArtistName, q.Artist)
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// MemoryStore is an in-memory Store, useful for development and tests.
type MemoryStore struct {
	events []Event
}

// NewMemoryStore returns a MemoryStore holding the given events.
func NewMemoryStore(events []Event) *MemoryStore {
	return &MemoryStore{events: events}
}

func (s *MemoryStore) Search(_ context.Context, q Query) ([]Event, error) {
	var out []Event
	for _, e := range s.events {
		if !q.From.IsZero() && e.EventDate < dateKey(q.From) {
			continue
		}
		if !q.To.IsZero() && e.EventDate > dateKey(q.To) {
			continue
		}
		if !matchesNames(e, q) {
			continue
		}
		out = append(out, e)
		if q.Limit > 0 && len(out) == q.Limit {
			break
		}
	}
	return out, nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (Event, bool, error) {
	for _, e := range s.events {
		if e.ID == id {
			return e, true, nil
		}
	}
	return Event{}, false, nil
}

func (s *MemoryStore) Clubs(_ context.Context, from time.Time) ([]Club, error) {
	counts := clubCounter{}
	for _, e := range s.events {
		if e.EventDate >= dateKey(from) {
			counts[e.ClubName]++
		}
	}
	return counts.sorted(), nil
}

// SampleEvents returns placeholder data shaped like the real collection.
func SampleEvents() []Event {
	return []Event{
		{
			ID:         "sample-1",
			ClubName:   "hakkasan nightclub",
			ArtistName: "dj vice",
			EventDate:  "2026-10-22T00:00:00Z",
			TicketURL:  "https://taogroup.com/event/10-22-2026-hakkasan-thursday-hakkasan-nightclub/",
		},
		{
			ID:         "sample-2",
			ClubName:   "marquee nightclub",
			ArtistName: "marquee nightclub saturday",
			EventDate:  "2027-01-02T00:00:00Z",
			TicketURL:  "https://taogroup.com/event/1-2-2027-marquee-nightclub-saturday-marquee-nightclub/",
		},
	}
}
