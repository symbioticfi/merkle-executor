# Runtime batching and measured scale

The daemon creates one shared read scheduler and registers the five automatic
integrations through `internal/service.DefaultIntegrations`. Runtime preparations
use bounded workers. The scheduler coalesces currently ready reads across those
workers, deduplicates immutable contexts within each collected wave, and partitions
the result before dispatch. A slow protocol API does not hold every other executor
at a global stage barrier.

Reviewed caller-independent getters can share Multicall3. A numerical lens keeps
its original executor, caller, numbered pin and code override. Full simulations
execute the original executor with its authorized caller. These calls remain
isolated JSON-RPC members; they are never wrapped in Multicall3. Merkl also retains
its reviewed four-getter protocol aggregate for a one-token claim.

## Policy and interpretation

The effective policy comes from strict YAML, with defaults in
`internal/config/defaults.yaml`:

| Resource | Default limit |
| --- | ---: |
| Concurrent preparations | 16 |
| Read partition timeout | 30 seconds |
| Logical members per partition | 256 |
| Encoded RPC request | 2,097,152 bytes |
| RPC response | 33,554,432 bytes |
| Calldata plus override code per partition | 1,048,576 bytes |
| Estimated gas per partition | 30,000,000 |
| Reserved output per partition | 16,777,216 bytes |
| Default undeclared call gas / output charge | 1,000,000 / 65,536 bytes |
| Concurrent RPC partitions | 4 |
| Accepted outstanding reads | 16,384 |
| Accepted outstanding byte charge | 134,217,728 bytes |
| Ready collection window | 1 ms |

Every budget applies together. In these fixtures the gas budget limited a getter
aggregate to 30 members, below the 256-member ceiling. Each numerical lens reserves
30,000,000 gas and therefore occupies its own partition with the default policy.
Read errors do not trigger a fallback to individual requests. Fresh closing
canonical reads remain separate observations even when they use the same height.

`Scheduler.Stats()` reports outstanding accepted work, including in-flight work;
its byte charge covers retained requests and reserved results. It is a budget
measurement, not the Go heap size. Its queue peaks are lifetime high-water marks,
including identity discovery. Dispatch and sharing counters can be differenced
between preparation runs. They count logical scheduler requests; Multicall members
and HTTP requests must be measured at the transport separately.

## Local fixture evidence

`cmd/merkle-executor/automatic_scale_test.go` exercises the actual default automatic
registration factory, YAML defaults, startup identity discovery, permission HTTP
client, shared scheduler, recipe discovery/cache, numerical lens, assembly and
full simulation witness validation. Four executors share each account. Alternating
groups prepare either a Merkl claim or a generic READ → min → equality → masked
transfer recipe. The one-executor case is Merkl only.

`TestAutomaticCLICompositionScale` also calls `runBotWithHTTP` → `service.Run` in
plan mode at all five scales. This proves CLI/service composition. Cold and warm
measurements use one SDK planner with that same service registration factory and
one recipe cache; separate CLI invocations construct separate caches.

All endpoints are local `httptest` HTTP/RPC servers. The official Merkl hostname
is rewritten to a local API fixture; the injected transport blocks other external
HTTP destinations. No external chain, live API, signing credential, broadcast or
deployment is involved. These are transport/API-fixture results; EVM execution
semantics are covered separately by the local Anvil integration tests.

The tests assert full simulation and lens counts, exact authorized claim bytes,
generic numerical witnesses, cross-executor getter aggregation, actual shared read
deliveries, queue release, and configured wire/concurrency/queue bounds. They also
exercise a blocked Merkl API while generic work reaches simulation, a failed
metadata partition that its executors cannot consume through retries, and full
simulation results with 16-KiB accepted versus 70-KiB rejected trailing output.
Latency is recorded without a timing performance assertion.

The following sample was collected on 2026-11-07 with Go 1.27.1, darwin/arm64,
Apple M4 Pro, default GOMAXPROCS 14. Collection order depends on goroutine and
network readiness, so HTTP/aggregate/dispatch counts vary across runs.

Startup is measured separately from each preparation below. It performs one
ordinary `eth_chainId` HTTP request plus these scheduler arrays:

| Executors | Startup arrays | Startup logical scheduler reads |
| ---: | ---: | ---: |
| 1 | 3 | 4 |
| 16 | 4 | 34 |
| 64 | 7 | 130 |
| 256 | 20 | 514 |
| 1,024 | 74 | 2,050 |

Each preparation acquires permissions for every executor and rewards for every
Merkl executor. Warming caches immutable recipes; it does not cache permissions,
rewards or full simulations:

| Executors | Permission / reward HTTP | Full simulations / lenses | Recipe event reads cold / warm |
| ---: | ---: | ---: | ---: |
| 1 | 1 / 1 | 1 / 0 | 1 / 0 |
| 16 | 16 / 8 | 16 / 8 | 16 / 0 |
| 64 | 64 / 32 | 64 / 32 | 64 / 0 |
| 256 | 256 / 128 | 256 / 128 | 256 / 0 |
| 1,024 | 1,024 / 512 | 1,024 / 512 | 1,024 / 0 |

Transport and scheduler counters from one `TestAutomaticRegistryScale` run of the
earlier rewrite, retained as a separate sample. The RAM-only follow-up reran all
scale regressions; its fresh benchmark results are recorded below:

| Executors | Cache | RPC arrays / wire members | Multicall calls / members | Cross-executor aggregates | Dispatched logical reads / shared deliveries | Ready waves |
| ---: | --- | ---: | ---: | ---: | ---: | ---: |
| 1 | cold | 7 / 10 | 3 / 15 | 0 | 19 / 0 | 7 |
| 1 | warm | 6 / 9 | 3 / 15 | 0 | 18 / 0 | 6 |
| 16 | cold | 25 / 78 | 12 / 170 | 6 | 227 / 62 | 13 |
| 16 | warm | 25 / 63 | 14 / 177 | 6 | 211 / 62 | 12 |
| 64 | cold | 94 / 318 | 51 / 695 | 31 | 917 / 236 | 47 |
| 64 | warm | 90 / 264 | 52 / 682 | 31 | 855 / 234 | 49 |
| 256 | cold | 377 / 1,336 | 231 / 2,849 | 134 | 3,741 / 868 | 186 |
| 256 | warm | 361 / 1,043 | 205 / 2,770 | 119 | 3,437 / 916 | 175 |
| 1,024 | cold | 1,438 / 5,276 | 899 / 11,174 | 531 | 14,789 / 3,644 | 654 |
| 1,024 | warm | 1,404 / 4,132 | 864 / 11,112 | 504 | 13,636 / 3,773 | 587 |

All preparation RPC requests in this sample were arrays. The largest observed
array had 19 wire members; the largest Multicall had 30 members. Maximum encoded
request, encoded response, and calldata-plus-override sizes were respectively
66,044, 40,412, and 31,676 bytes. Observed endpoint concurrency never exceeded four.
The queue returned to zero outstanding requests/bytes after every preparation.

| Executors | Lifetime peak accepted reads | Lifetime peak charged bytes |
| ---: | ---: | ---: |
| 1 | 7 | 470,274 |
| 16 | 102 | 6,849,902 |
| 64 | 128 | 8,534,784 |
| 256 | 256 | 17,069,568 |
| 1,024 | 256 | 17,069,568 |

One-pass `BenchmarkAutomaticRegistryScale` results for RAM-only source `e11a4d8`,
collected separately from the race run and the preceding counter sample:

| Executors | Cache | Time / op | Bytes / op | Allocations / op | RPC HTTP / op | Distinct observed read keys |
| ---: | --- | ---: | ---: | ---: | ---: | ---: |
| 1 | cold | 13.76 ms | 1,661,992 | 13,184 | 7 | 15 |
| 1 | warm | 9.50 ms | 941,880 | 8,305 | 6 | 14 |
| 16 | cold | 27.09 ms | 17,673,256 | 123,200 | 25 | 135 |
| 16 | warm | 23.62 ms | 16,087,640 | 116,317 | 21 | 119 |
| 64 | cold | 79.17 ms | 68,803,768 | 487,107 | 86 | 531 |
| 64 | warm | 76.66 ms | 65,167,992 | 469,952 | 91 | 467 |
| 256 | cold | 297.51 ms | 271,038,944 | 1,951,464 | 359 | 2,115 |
| 256 | warm | 277.78 ms | 255,208,184 | 1,883,641 | 369 | 1,859 |
| 1,024 | cold | 1,132.44 ms | 1,042,425,704 | 7,740,151 | 1,454 | 8,451 |
| 1,024 | warm | 987.99 ms | 969,391,504 | 7,421,462 | 1,359 | 7,427 |

Benchmark timing/allocation excludes fixture construction, startup discovery and
warm-up preparation. It includes production planning plus concurrent local server
handling, JSON/ABI fixture encoding and metric collection. Bytes/op is cumulative
process allocation during the measured operation, not retained heap or runtime-only
memory. Distinct observed read keys expand Multicall getters and include complete
call context/pin; repeated fresh `latest` or closing observations can share a key.
That key count does not mean those observations may legally be cached or removed.
The high cumulative allocation at 1,024 executors remains visible in the evidence;
bounded scheduler queues do not establish low total allocation.

These measurements demonstrate bounded cross-executor batching on the supported
automatic preparation path. A one-pass local fixture benchmark does not establish
theoretical optimality, production RPC throughput or a latency guarantee.

## Reproduce

```sh
go test ./cmd/merkle-executor -run '^TestAutomatic(RegistryScale|CLICompositionScale|SlowAPI|FailedBatch|SimulationOutput)' -count=1 -v
go test -race ./cmd/merkle-executor -run '^TestAutomatic(RegistryScale|CLICompositionScale|SlowAPI|FailedBatch|SimulationOutput)' -count=1
go test ./cmd/merkle-executor -run '^$' -bench '^BenchmarkAutomaticRegistryScale$' -benchtime=1x -count=1 -benchmem
```

Repository acceptance still requires `make check` and `make integration`. The
latter runs local ephemeral Anvil fixtures. Neither the local gates nor these API
fixtures are deployment or live API evidence.
