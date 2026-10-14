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

`cloudbuild.yaml` builds the image, pushes it to Artifact Registry and deploys
it to Cloud Run. One-time setup:

```sh
PROJECT=edm-events-project
REGION=us-central1
PROJECT_NUMBER=$(gcloud projects describe $PROJECT --format='value(projectNumber)')
BUILD_SA=$PROJECT_NUMBER-compute@developer.gserviceaccount.com  # Cloud Build's default SA
RUNTIME_SA=find-edm-events-mcp@$PROJECT.iam.gserviceaccount.com

gcloud services enable run.googleapis.com cloudbuild.googleapis.com artifactregistry.googleapis.com
gcloud artifacts repositories create containers --repository-format=docker --location=$REGION

# Identity the service runs as: read access to Firestore only.
gcloud iam service-accounts create find-edm-events-mcp
gcloud projects add-iam-policy-binding $PROJECT --member=serviceAccount:$RUNTIME_SA --role=roles/datastore.viewer

# Let Cloud Build deploy to Cloud Run as that identity.
gcloud projects add-iam-policy-binding $PROJECT --member=serviceAccount:$BUILD_SA --role=roles/run.admin
gcloud iam service-accounts add-iam-policy-binding $RUNTIME_SA --member=serviceAccount:$BUILD_SA --role=roles/iam.serviceAccountUser
```

Then deploy:

```sh
gcloud builds submit --config cloudbuild.yaml
```

The service is public (`--allow-unauthenticated`). Each client IP is limited
to 2 requests/second with bursts of 20 (`-rate`/`$RATE_LIMIT_RPS`, `-burst`;
`-rate 0` disables). Limits are per instance, and `_MAX_INSTANCES` (default 3)
caps total throughput and cost.

```sh
URL=$(gcloud run services describe find-edm-events-mcp --region=$REGION --format='value(status.url)')
claude mcp add --transport http find-edm-events-remote $URL
```

HTTP mode is stateless, so requests can land on any instance.
