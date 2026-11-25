# MCP usage log

The MCP server can append one JSON line per tool call. It only records; it never
aggregates, scores, or feeds anything back into query results. Analysis is done
later, offline, by a separate tool or an AI pass over the file.

## Enabling it

Set `MCP_USAGE_LOG_FILE` to a writable path. When unset or blank, nothing is
written and the server behaves exactly as before.

```sh
MCP_USAGE_LOG_FILE=/data/usage/mcp-usage.jsonl
```

The container root is read-only, so mount a writable directory at the log's
parent. In Compose:

```yaml
environment:
  MCP_USAGE_LOG_FILE: /data/usage/mcp-usage.jsonl
volumes:
  - ./data/usage:/data/usage:rw
```

The directory must be writable by the container user (UID 10001). If the file
cannot be opened the server logs a warning and keeps serving without recording.

## Record shape

One JSON object per line, append-only:

```json
{"ts":"2026-11-25T14:54:40Z","tool":"graph_query","args":{"type":"Machine"},"resultIds":["machine/hp/vm128.md"],"resultCount":1,"durationMs":3,"ok":true}
```

| field | meaning |
|---|---|
| `ts` | UTC timestamp, RFC 3339 with nanoseconds |
| `tool` | tool name, for example `graph_query` |
| `args` | the caller's arguments, verbatim |
| `resultIds` | node IDs the call returned, so later analysis can tell which entities were actually pulled |
| `resultCount` | number of results |
| `durationMs` | wall time of the call |
| `ok` | `false` for transport errors and tool-level errors alike |
| `error` | failure message when `ok` is `false` |
| `feedback` | `useful` or `not_useful`, only on `graph_feedback` lines |
| `feedbackFor` | the node ID the caller judged |
| `note` | the caller's free-text remark, truncated to 500 bytes |

`resultIds` is best-effort: it is filled from list-shaped results and from the
single node of an entity lookup. A tool that returns neither shape logs an empty
list.

## Optional usefulness feedback

`graph_feedback` lets a caller say whether a lookup paid off. It is not required
and never changes graph content or query results.

```json
{"tool":"graph_feedback","args":{"useful":true,"forId":"machine/hp/vm128.md","note":"answered the role question"}}
```

Without a usage log attached, the tool still answers successfully with
`recorded: false`; it is a no-op rather than an error.

## What it deliberately does not do

- No rotation, compression, or retention policy: the file grows by one line per
  call. Truncate or rotate it externally if that matters.
- No querying over the log through MCP: read the file directly.
- No caller identity: the MCP SDK version in use does not surface `clientInfo`
  to the middleware, so concurrent clients cannot be told apart.
