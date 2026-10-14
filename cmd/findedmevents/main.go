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
	"strconv"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/weironiottan/findEdmEventsMCP/internal/events"
	"github.com/weironiottan/findEdmEventsMCP/internal/ratelimit"
	"github.com/weironiottan/findEdmEventsMCP/internal/server"
)

func main() {
	httpAddr := flag.String("http", "", "serve streamable HTTP on this address (e.g. :8080) instead of stdio")
	rps := flag.Float64("rate", envFloat("RATE_LIMIT_RPS", 2), "per-client-IP requests per second in HTTP mode, 0 to disable ($RATE_LIMIT_RPS)")
	burst := flag.Int("burst", 20, "per-client-IP burst size in HTTP mode")
	sample := flag.Bool("sample", false, "use built-in sample data instead of Firestore")
	cfg := events.FirestoreConfig{}
	flag.StringVar(&cfg.ProjectID, "project", envOr("FIRESTORE_PROJECT", "edm-events-project"), "GCP project ($FIRESTORE_PROJECT)")
	flag.StringVar(&cfg.DatabaseID, "database", envOr("FIRESTORE_DATABASE", "edm-events-database"), "Firestore database ($FIRESTORE_DATABASE)")
	flag.StringVar(&cfg.Collection, "collection", envOr("FIRESTORE_COLLECTION", "edm-events"), "Firestore collection ($FIRESTORE_COLLECTION)")
	flag.Parse()

	// stdout carries the MCP protocol in stdio mode, so log to stderr.
	log.SetOutput(os.Stderr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, closeStore, err := newStore(ctx, *sample, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer closeStore()
	srv := server.New(store)

	if *httpAddr != "" {
		// Stateless so any instance can serve any request; the tools are
		// read-only and never call back to the client.
		var handler http.Handler = mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
			&mcp.StreamableHTTPOptions{Stateless: true})
		if *rps > 0 {
			handler = ratelimit.New(*rps, *burst).Middleware(handler)
		}
		if err := serveHTTP(ctx, *httpAddr, handler); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

// serveHTTP serves until ctx is cancelled, then drains in-flight requests.
// Cloud Run sends SIGTERM and allows 10s before killing the instance.
func serveHTTP(ctx context.Context, addr string, handler http.Handler) error {
	hs := &http.Server{Addr: addr, Handler: handler}
	errc := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", addr)
		errc <- hs.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return hs.Shutdown(shutdownCtx)
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

func envFloat(key string, fallback float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil {
		return v
	}
	return fallback
}
