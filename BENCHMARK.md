# WalkieBench V2 results

The final acceptance run used HarnessTalkie `950e91b` and WalkieBench
`1b5fe3d` with a fresh encrypted event log, the full default scale workload,
server resource sampling, and the production embedded React application.

```sh
go run ../WalkieBench/cmd/walkiebench \
  --endpoint http://127.0.0.1:8080/rpc \
  --ui-url http://127.0.0.1:8080 \
  --profile full \
  --resource-command /path/to/resource-sampler \
  --output artifacts/final-2026-09-11-full.json
```

Result:

- 25/25 scenarios passed.
- Zero message loss, ordering violations, failed resumes, access-control
  violations, and encryption violations.
- 227.87 sustained and 240.86 peak mixed messages/second.
- 46.62 ms time to first successful collaboration.
- 2,000-message group history: 28.55 ms; 1,000-comment thread: 12.74 ms.
- Response shaping reduced payload size by 71.79%.
- Canonical agent jobs: 12 round trips, 13,397 total bytes, 3,356 estimated
  tokens, zero retries, and zero explicit maintenance operations.
- Peak server RSS: 27.89 MB; final encrypted storage: 15.02 MB.

Only JSON-RPC over HTTP is advertised, so the optional cross-transport pair
count is zero. The CLI uses that same transport and shared state. See
`BENCHMARK_HISTORY.md` for the audited baseline and optimization comparison.
