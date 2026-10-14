// Command findedmevents runs the EDM Events MCP server.
//
// By default it speaks MCP over stdio. Pass -http to serve the streamable
// HTTP transport instead, e.g. for a remote agent.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weironiottan/findEdmEventsMCP/internal/events"
	"github.com/weironiottan/findEdmEventsMCP/internal/server"
)

func main() {
	httpAddr := flag.String("http", "", "serve streamable HTTP on this address (e.g. :8080) instead of stdio")
	sample := flag.Bool("sample", false, "use built-in sample data instead of Firestore")
	cfg := events.FirestoreConfig{}
	flag.StringVar(&cfg.ProjectID, "project", envOr("FIRESTORE_PROJECT", "edm-events-project"), "GCP project ($FIRESTORE_PROJECT)")
	flag.StringVar(&cfg.DatabaseID, "database", envOr("FIRESTORE_DATABASE", "edm-events-database"), "Firestore database ($FIRESTORE_DATABASE)")
	flag.StringVar(&cfg.Collection, "collection", envOr("FIRESTORE_COLLECTION", "edm-events"), "Firestore collection ($FIRESTORE_COLLECTION)")
	flag.Parse()

	// stdout carries the MCP protocol in stdio mode, so log to stderr.
	log.SetOutput(os.Stderr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	store, closeStore, err := newStore(ctx, *sample, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer closeStore()
	srv := server.New(store)

	if *httpAddr != "" {
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
		log.Printf("listening on %s", *httpAddr)
		if err := http.ListenAndServe(*httpAddr, handler); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func newStore(ctx context.Context, sample bool, cfg events.FirestoreConfig) (events.Store, func(), error) {
	if sample {
		log.Print("using in-memory sample data")
		return events.NewMemoryStore(events.SampleEvents()), func() {}, nil
	}
	fs, err := events.NewFirestoreStore(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	return fs, func() { fs.Close() }, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
