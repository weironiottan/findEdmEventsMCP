# findEdmEventsMCP
An MCP Server for the EDM Events DB that will eventually connect to  an Agent

Built with the official [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk).

## Layout

```
cmd/findedmevents/   entrypoint (stdio by default, -http for streamable HTTP)
internal/events/     Event model, Store interface, in-memory store + sample data
internal/server/     MCP server and tool handlers
```

## Tools

| Tool            | Description                                         |
|-----------------|-----------------------------------------------------|
| `search_events` | Filter by `club`, `artist`, `from`/`to` (YYYY-MM-DD, `from` defaults to today), `limit` |
| `get_event`     | Fetch one event by `id`                             |
| `list_clubs`    | Clubs with upcoming events and their event counts   |

`search_events` returns `truncated: true` when more events matched than `limit`.

## Data source

Reads from Firestore using Application Default Credentials
(`gcloud auth application-default login`). Defaults can be overridden with
flags or env vars:

| Flag          | Env var                | Default               |
|---------------|------------------------|-----------------------|
| `-project`    | `FIRESTORE_PROJECT`    | `edm-events-project`  |
| `-database`   | `FIRESTORE_DATABASE`   | `edm-events-database` |
| `-collection` | `FIRESTORE_COLLECTION` | `edm-events`          |

Pass `-sample` to use built-in sample data instead.

## Usage

```sh
make test       # run tests
make build      # build bin/findedmevents
make run-http   # serve streamable HTTP on :8080
make inspect    # open the MCP Inspector (requires Node)
```

Add to Claude Code over stdio:

```sh
claude mcp add find-edm-events -- $(pwd)/bin/findedmevents
```


## Deploy to Cloud Run

A Cloud Build trigger on pushes to `main` runs `cloudbuild.yaml`, which builds
the Dockerfile, pushes the image to the `cloud-run-source-deploy` Artifact
Registry repo, and deploys the `findedmeventsmcp-service` Cloud Run service.
The service runs as the default Compute service account and reads Firestore
via Application Default Credentials.

The service is public. Each client IP is limited to 2 requests/second with
bursts of 20 (`-rate`/`$RATE_LIMIT_RPS`, `-burst`; `-rate 0` disables).
Limits are per instance, and `_MAX_INSTANCES` (default 3) caps total
throughput and cost.

```sh
URL=$(gcloud run services describe findedmeventsmcp-service --region=us-central1 --format='value(status.url)')
claude mcp add --transport http find-edm-events-remote $URL
```

HTTP mode is stateless, so requests can land on any instance.

## Debugging

Every MCP request is logged to stderr as one JSON line with the method, tool,
arguments, duration, and any error. Cloud Logging parses these into structured
fields. Tool failures return HTTP 200 with `isError` set, so search the logs
rather than relying on status codes:

```sh
gcloud logging read 'resource.labels.service_name="findedmeventsmcp-service" AND jsonPayload.isError=true' --limit 20
```
