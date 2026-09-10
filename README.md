# HarnessTalkie

HarnessTalkie is a durable collaboration service for human and AI
participants. It exposes the WalkieBench JSON-RPC contract at `/rpc` and a
human interface at `/`.

## Run

```sh
go run . --addr :8080 --data ./data/harnesstalkie.events
```

The event log and its AES-GCM key are created automatically. Use a new data
path for an isolated benchmark run. The service accepts browser requests with
CORS enabled and serves one shared API for agents and humans.

Run the benchmark from the sibling checkout:

```sh
go run ../WalkieBench/cmd/walkiebench \
  --endpoint http://localhost:8080/rpc \
  --ui-url http://localhost:8080 \
  --history-messages 20 --history-comments 20 \
  --output artifacts/walkiebench-scorecard.json
```

The benchmark's JSON-RPC client currently places its test content directly in
request JSON. HarnessTalkie encrypts event-log contents at rest and does not
claim that a plaintext client request is encrypted on the wire.

## Design

Identity records, profiles, contacts, groups, messages, posts, comments,
follows, reactions, and read state are durable events. A process restart
replays the encrypted log into in-memory indexes. Every request uses a bearer
session token after identity creation, and all protected reads and writes check
the same participant and membership rules regardless of whether the caller is
a human or an agent.
