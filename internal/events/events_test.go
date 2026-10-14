package events

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestMemoryStoreClubs(t *testing.T) {
	s := NewMemoryStore([]Event{
		{ClubName: "omnia nightclub", EventDate: "2026-12-04T00:00:00Z"},
		{ClubName: "omnia nightclub", EventDate: "2026-12-05T00:00:00Z"},
		{ClubName: "hakkasan nightclub", EventDate: "2026-12-05T00:00:00Z"},
		{ClubName: "marquee dayclub", EventDate: "2026-11-01T00:00:00Z"}, // before from
	})
	got, err := s.Clubs(context.Background(), time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	want := []Club{{"hakkasan nightclub", 1}, {"omnia nightclub", 2}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
