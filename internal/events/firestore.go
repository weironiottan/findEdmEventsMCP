package events

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

// FirestoreConfig configures the Firestore-backed store.
type FirestoreConfig struct {
	ProjectID  string
	DatabaseID string
	Collection string
}

// FirestoreStore is a Store backed by Cloud Firestore.
//
// Credentials come from Application Default Credentials: run
// `gcloud auth application-default login` locally, or set
// GOOGLE_APPLICATION_CREDENTIALS to a service account key. Setting
// FIRESTORE_EMULATOR_HOST points the client at the local emulator instead.
type FirestoreStore struct {
	client     *firestore.Client
	collection string
}

// NewFirestoreStore connects to Firestore. Call Close when done.
func NewFirestoreStore(ctx context.Context, cfg FirestoreConfig) (*FirestoreStore, error) {
	if cfg.ProjectID == "" || cfg.DatabaseID == "" || cfg.Collection == "" {
		return nil, errors.New("firestore: project, database, and collection are required")
	}
	client, err := firestore.NewClientWithDatabase(ctx, cfg.ProjectID, cfg.DatabaseID)
	if err != nil {
		return nil, fmt.Errorf("firestore: new client: %w", err)
	}
	return &FirestoreStore{client: client, collection: cfg.Collection}, nil
}

// Close releases the underlying client.
func (s *FirestoreStore) Close() error {
	return s.client.Close()
}

// Search filters by date range in Firestore and by club/artist in Go.
// Firestore has no case-insensitive or substring matching, and keeping the
// query to a range on a single field avoids needing composite indexes.
func (s *FirestoreStore) Search(ctx context.Context, q Query) ([]Event, error) {
	fq := s.client.Collection(s.collection).OrderBy("EventDate", firestore.Asc)
	if !q.From.IsZero() {
		fq = fq.Where("EventDate", ">=", dateKey(q.From))
	}
	if !q.To.IsZero() {
		fq = fq.Where("EventDate", "<=", dateKey(q.To))
	}
	// Without name filters every document matches, so let Firestore limit.
	if q.Club == "" && q.Artist == "" && q.Limit > 0 {
		fq = fq.Limit(q.Limit)
	}

	iter := fq.Documents(ctx)
	defer iter.Stop()

	var out []Event
	for {
		snap, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("firestore: search: %w", err)
		}
		var e Event
		if err := snap.DataTo(&e); err != nil {
			return nil, fmt.Errorf("firestore: decode %s: %w", snap.Ref.ID, err)
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

// Clubs scans ClubName on matching events. Firestore has no DISTINCT, but
// projecting a single field keeps the scan cheap at this collection size.
func (s *FirestoreStore) Clubs(ctx context.Context, from time.Time) ([]Club, error) {
	iter := s.client.Collection(s.collection).
		Where("EventDate", ">=", dateKey(from)).
		Select("ClubName").
		Documents(ctx)
	defer iter.Stop()

	counts := clubCounter{}
	for {
		snap, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("firestore: clubs: %w", err)
		}
		name, _ := snap.Data()["ClubName"].(string)
		if name != "" {
			counts[name]++
		}
	}
	return counts.sorted(), nil
}

// Get looks up an event by its Id field (not the Firestore document ID).
func (s *FirestoreStore) Get(ctx context.Context, id string) (Event, bool, error) {
	iter := s.client.Collection(s.collection).Where("Id", "==", id).Limit(1).Documents(ctx)
	defer iter.Stop()

	snap, err := iter.Next()
	if errors.Is(err, iterator.Done) {
		return Event{}, false, nil
	}
	if err != nil {
		return Event{}, false, fmt.Errorf("firestore: get %s: %w", id, err)
	}
	var e Event
	if err := snap.DataTo(&e); err != nil {
		return Event{}, false, fmt.Errorf("firestore: decode %s: %w", snap.Ref.ID, err)
	}
	return e, true, nil
}
