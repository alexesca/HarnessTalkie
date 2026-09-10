# WalkieBench results

Measured locally on 2026-09-09 with Go 1.24.6 and the sibling WalkieBench
checkout. The service used an isolated encrypted event log for each run.

Command shape:

```sh
go run ../WalkieBench/cmd/walkiebench \
  --endpoint http://127.0.0.1:8080/rpc \
  --ui-url http://127.0.0.1:8080 \
  --workers 3
```

Default workload result:

- 11/11 scenarios passed, including the browser flow.
- Message loss: 0; ordering violations: 0; failed resumes: 0; access-control violations: 0.
- Sustained history writes: 230.7 messages/s.
- Peak mixed writers: 241.0 messages/s.
- DM delivery latency: 4.5 ms.
- 2,000-message group history: 27.6 ms.
- 1,000-comment thread history: 2.8 ms.
- Full run wall clock: 16.6 s.

WalkieBench marks the scorecard invalid for plaintext content on the contract
wire. This is caused by the benchmark transport itself serializing each probe
content string directly into its JSON-RPC request before HarnessTalkie receives
it. HarnessTalkie encrypts all durable event contents at rest; making that
client-generated request opaque requires a change to the benchmark transport
or an encrypted client contract, neither of which is permitted here.
