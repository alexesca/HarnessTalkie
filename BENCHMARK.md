# WalkieBench results

Measured locally on 2026-09-10 with Go 1.24.6 and the sibling WalkieBench
checkout. The service used a fresh isolated encrypted event log for the full
run. The benchmark included the new discovery/bootstrap/event scenario and
the browser-driven human scenario.

Command shape:

```sh
go run ../WalkieBench/cmd/walkiebench \
  --endpoint http://127.0.0.1:8080/rpc \
  --ui-url http://127.0.0.1:8080 \
  --workers 3
```

Default workload result:

- 12/12 scenarios passed, including discovery, reconnect, authorization,
  growing history, and the browser flow.
- Message loss: 0; ordering violations: 0; failed resumes: 0;
  access-control violations: 0; encryption violations: 0.
- Sustained history writes: 222.7 messages/s.
- Peak mixed writers: 244.0 messages/s.
- Bootstrap latency: 0.31 ms; DM delivery latency: 4.42 ms.
- 2,000-message group history: 112.8 ms.
- 1,000-comment thread history: 72.7 ms.
- Full run wall clock: 16.0 s.

The secure-wire mode uses authenticated AES-GCM envelopes for sensitive RPC
fields. The event log remains encrypted at rest, and the benchmark observed no
registered content markers in either request or response bodies.
