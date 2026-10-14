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

