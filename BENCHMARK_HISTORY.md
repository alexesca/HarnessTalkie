# WalkieBench V2 history

All runs use fresh encrypted event logs and the default full workload: three
workers, 2,000 history messages, 1,000 comments, the real browser application,
and server resource sampling.

| Run | HarnessTalkie | WalkieBench | Result | Sustained / peak | TTFC | Wire bytes | Peak RSS | Storage |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Initial audit baseline | `db3d7ec` | `d40822b` | failed 2 of 22 scenarios | 226.17 / 242.17 msg/s | 9.23 ms | 3,309,243 | 20.38 MB | 1.21 MB |
| Correct V2, uncompressed checkpoints | `22b5e5f` | `1b5fe3d` | passed 25 of 25 | 227.02 / 233.13 msg/s | 55.58 ms | 3,374,387 | 32.77 MB | 144.61 MB |
| Compressed checkpoints | `950e91b` | `1b5fe3d` | passed 25 of 25 | 227.87 / 240.86 msg/s | 46.62 ms | 3,374,367 | 27.89 MB | 15.02 MB |
| Final security-validated | `b446451` | `7cf16e7` | passed 25 of 25 | 226.88 / 240.34 msg/s | 49.28 ms | 3,374,522 | 28.53 MB | 15.02 MB |

The original browser scenario and several V2 checks were weaker, so its wall
clock, time-to-first-collaboration, operation totals, and token accounting are
not directly comparable to the repaired benchmark. The final benchmark drives
the routed UI through DM, group, forum, notification, approval, and role
workflows. Its canonical agent-job accounting records 12 round trips, 13,395
wire bytes, 3,356 estimated tokens, no retries, no explicit maintenance
operations, and a 96.67 efficiency score. The final pass adds an
identity-takeover negative test and makes browser layout initialization
independent of a previously persisted `agent-browser` viewport.

Compressing durable V2 checkpoints reduced final-store growth from 144.61 MB
to 15.02 MB (89.6%) and peak RSS from 32.77 MB to 27.89 MB while preserving
throughput and every correctness gate. Final reliability counters are all
zero: message loss, ordering violations, resume failures, access-control
violations, and encryption violations.

Ignored raw scorecards are retained locally at
`artifacts/baseline-2026-09-10-full.json` and
`artifacts/final-2026-09-11-full.json`.
