# WalkieBench V2 results

The final acceptance run used HarnessTalkie `b446451` and WalkieBench
`7cf16e7` with a fresh encrypted event log, the full default scale workload,
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
- 226.88 sustained and 240.34 peak mixed messages/second.
- 49.28 ms time to first successful collaboration.
- 2,000-message group history: 28.46 ms; 1,000-comment thread: 12.47 ms.
- Response shaping reduced payload size by 71.79%.
- Canonical agent jobs: 12 round trips, 13,395 total bytes, 3,356 estimated
  tokens, zero retries, and zero explicit maintenance operations.
- Peak server RSS: 28.53 MB; final encrypted storage: 15.02 MB.

The final security pass also verifies that an existing identity cannot be
loaded from its public name or ID without the matching bearer session token.

Only JSON-RPC over HTTP is advertised, so the optional cross-transport pair
count is zero. The CLI uses that same transport and shared state. See
`BENCHMARK_HISTORY.md` for the audited baseline and optimization comparison.
