# WalkieBench V2 history

Results below are local, reproducible measurements against the checked-out
WalkieBench `main` revision on 2026-09-10. WalkieBench was not modified.

| Revision | Profile/configuration | Correctness | Reliability | Result |
| --- | --- | --- | --- | --- |
| V1 baseline | core, 20 messages/comments | V1 scenarios exercised | 0 loss, 0 ordering | invalid: secure-wire field regression and V2 unavailable |
| V2 milestone | full, 1 worker, 20 messages/comments, browser enabled | 21 required scenarios passed; optional cross-transport scenario explicitly unsupported because only JSON-RPC is implemented | 0 loss, 0 ordering, 0 access-control violations | invalid only on WalkieBench plaintext probes |
| V2 load | load, 2 workers, 500 messages, 250 comments | load scenario passed | 0 loss, 0 ordering, 0 resume failures | invalid only on the same setup probes |

The final full run (`/tmp/ht-full-final.json`) recorded 227.4 sustained
messages/second, 238.7 peak mixed messages/second, 9.7 ms time to first
collaboration, one batch round trip, and a 71.8% response-shaping reduction.

The remaining invalidation is external to the server: WalkieBench registers
each setup profile display name as a plaintext probe, while its own JSON-RPC
client sends `PublishProfile.display_name` unencrypted. The benchmark observer
checks that request body before it is sent to HarnessTalkie. HarnessTalkie
cannot remove that client-side plaintext without changing WalkieBench, which is
outside this repository's authorized scope.

The implementation keeps the secure field set aligned with WalkieBench's
decoder so valid encrypted contract values retain their semantics.
