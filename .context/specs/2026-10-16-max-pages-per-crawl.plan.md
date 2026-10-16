# Plan: Maximum pages per crawl
Spec: SPEC-01 — 2026-10-16-max-pages-per-crawl.md
Status: ready

> Draft only because the spec is still `Status: draft`. No open planning
> question remains. Flip to `ready` when the user approves SPEC-01 as written,
> including assumptions A-1 to A-12.

> **How to read the tasks.** CLAUDE.md puts this repo in teaching mode: the
> developer writes the code. Each task names the file, the idiom or stdlib API,
> and what the test must assert. It does not give the solution. Snippets are
> at most 5 lines, use different names, and only illustrate an idiom.

## 1. What already ships

| Layer | Fact | path:line |
|---|---|---|
| Crawler | `NewCrawler(logger, workersCount, fetcher, store)`. The shared frontier is `make(chan CrawlJob, 500)` | internal/crawler/crawler.go:47-56 |
| Crawler | Per-crawl `CrawlTracker` holds `totalDuration`, `totalVisits` (atomics) and `completeWG` | internal/crawler/crawler.go:32-36 |
| Crawler | Consumer-side `store` interface: `UpdateStatus`, `UpdateResult(ctx, id, duration, visits)` | internal/crawler/crawler.go:42-45 |
| Crawler | Worker loop: loads the tracker, fetches, calls `completeWG.Done()` on error, counts the visit on success, then submits the links *before* its own `Done()` | internal/crawler/crawler.go:62-101 |
| Crawler | Log context key for the crawl is `"ID"` | internal/crawler/crawler.go:77 |
| Crawler | `Submit` order: crawler-wide dedup (`LoadOrStore`), then the seed-prefix filter, then tracker `LoadOrStore` and `completeWG.Add(1)` | internal/crawler/crawler.go:114-126 |
| Crawler | Seed submit sets `in_progress` and starts the completion goroutine: `Wait()`, then `done`, then `UpdateResult` | internal/crawler/crawler.go:128-147 |
| Domain | `Crawl{ID, Status, URL, Duration, Visits}` | internal/crawler/crawl.go:10-16 |
| Store (mem) | Plain map, no mutex. `UpdateResult` writes `Duration` and `Visits` | internal/store/memory_store.go:12-14, 60-71 |
| Store (pg) | `Get` selects `status, url, duration, visits`. `UpdateResult` runs `UPDATE … SET duration, visits` | internal/store/postgres.go:31-34, 93-110 |
| Schema | `crawl_jobs` table, latest migration `000002` | migrations/000002_create_crawl_jobs_table.up.sql:1-9 |
| API | `getCrawlResponse{id, status, duration, visits}` | internal/api/getCrawl.go:13-18, 40-45 |
| API test | The test decodes into its own `getCrawlResponse{ID, Status}` type | internal/api/getCrawl_test.go:16-19 |
| API test | The API `mockStore` has only `Get` and `Save`, so the `UpdateResult` change does not touch it | internal/api/helpers_test.go:19-30 |
| Wiring | `main` prints `run()`'s error to stderr and exits 1 | cmd/server/main.go:24-28 |
| Wiring | The DB connection (35-50) runs before the `WORKERS_COUNT` parse (55-62). `NewCrawler` is called at 67 and `ListenAndServe` at 78-82 | cmd/server/main.go:35-82 |
| Tests | `mockStore` writes `status` with no lock and `close(done)` in `UpdateResult`. `mockFetcher` does `f.visits++` | internal/crawler/crawler_test.go:13-45 |
| Tests | `NewCrawler` is called with 1 worker at 72, 124, 175, 232 and 283. The `visits` assertion is at 260 | internal/crawler/crawler_test.go:72-77, 201-268 |
| Config | `.env.example` lists `WORKERS_COUNT` and the DB variables | .env.example:1-5 |
| Infra | Compose passes app env explicitly, so a new variable is not inherited | docker-compose.yml:6-8 |
| Tooling | `race: go test -race ./...`, `test`, `lint`, and `create-migration c=<name>` (`-seq`) | Makefile |
| Library | zerolog v1.34 provides `SyncWriter(w io.Writer) io.Writer` (mutex-wrapped writer) | $GOMODCACHE/github.com/rs/zerolog@v1.34.0/writer.go:48 |

Rules applied: CLAUDE.md "Go Standards" (wrap errors, interfaces defined at the consumer, documented goroutine exit, `-race` non-negotiable, table-driven tests, structured logs with crawl ID); CLAUDE.md "Project Structure" (`cmd/server` holds `main.go` only); `engineering-paved-path:postgresql-table-design` → *Booleans* (`BOOLEAN NOT NULL`) and *Safe Schema Evolution* (a non-volatile default does not rewrite the table).

## 2. Decisions

### D1 — Limit lives on `Crawler` as `maxPages int`, with 0 meaning unlimited
Add a positional `maxPages int` parameter to `NewCrawler`, stored on the struct. Config validation (D7) never produces 0, so the zero value can safely mean "no limit". A tiny `limited() bool` helper keeps the checks readable.
*Why:* one knob, zero value = today's behaviour, and existing tests only need `0` added at each call.
*Rejected:* functional options `WithMaxPages(n)` (good idiom, but heavy for one knob); `*int` (nil checks at every use).
*Serves:* AC-1, AC-2, NFR-3.

### D2 — Exact cap by reserving budget before each fetch
Add a `reserved atomic.Int64` to `CrawlTracker`. Before calling `Fetch`, the worker calls `tracker.tryReserve(limit)`, a compare-and-swap loop that increments only while `reserved < limit`. If `Fetch` fails, the worker releases the slot with `Add(-1)`. A success keeps it. So `totalVisits ≤ reserved ≤ N` always holds.
Idiom (CAS loop, illustrative names):
```go
for cur := slots.Load(); cur < max; cur = slots.Load() {
    if slots.CompareAndSwap(cur, cur+1) { return true } // won the slot
}
return false
```
*Why:* this holds A-2 (no fetch can push the count past N, and in-flight fetches are not cancelled) and A-3 (a failed fetch returns its slot, but URLs dropped meanwhile stay dropped). AC-3 follows: once `totalVisits == N`, `reserved == N`, so every later reservation fails.
*Rejected:* checking `totalVisits < N` before fetching (two workers pass the check together and overshoot, which breaks AC-2); a mutex held across `Fetch` (serializes the crawl); cancelling in-flight fetches (excluded by A-2).
*Serves:* AC-2, AC-3, AC-9.

### D3 — Also drop newly discovered URLs in `Submit` once the cap is definitely reached
In `Submit`, add the check **after** dedup and the prefix filter (crawler.go:115-122), after the tracker load and **before** `completeWG.Add(1)`: if limited and `totalVisits >= N`, drop the URL. A dropped job never touches the WaitGroup. At dequeue (D2), a failed reservation calls `completeWG.Done()` as the error path already does.
*Why:* A-4 says the crawl stops queueing new URLs, which keeps the shared 500-slot frontier free for other crawls (AC-7). Placing the check after dedup and prefix matches A-6 ("in-scope, not-yet-visited") and NG-3 (a dropped URL stays visited). Checking `totalVisits` rather than `reserved` lets queued URLs use budget freed by failed fetches.
*Rejected:* dropping only at dequeue (frontier pressure on other crawls); checking before dedup (would count already-visited URLs toward `limit_reached`).
*Serves:* AC-3, AC-5, AC-7.

### D4 — `limit_reached` is a per-tracker `atomic.Bool` set by CompareAndSwap
Both drop points (D2 and D3) call one helper, e.g. `markLimitReached`. It calls `flag.CompareAndSwap(false, true)`, and only the call that wins logs (D10). The completion goroutine reads `flag.Load()` and passes it to `UpdateResult`.
*Why:* `CompareAndSwap` gives "set + exactly one winner" with no `sync.Once` and no mutex. A crawl that never drops a URL keeps `false`, which covers A-6, the exact-N edge case and a seed that fails to fetch.
*Rejected:* deriving the flag from `visits == N` (wrong for A-3 and for the exact-N case).
*Serves:* AC-5, AC-6, NFR-1.

### D5 — Persist the flag by extending `UpdateResult` with `limitReached bool`
New signature: `UpdateResult(ctx, id, duration, visits, limitReached)`. The contract exists in four places, and every one changes in a single slice (S1): the `store` interface (crawler.go:42-45), `MemoryCrawlStore.UpdateResult`, `PostgresCrawlStore.UpdateResult`, and the crawler test `mockStore`.
*Why:* it is the same write as `visits`, so the NG-4 completion order is untouched.
*Rejected:* a separate `MarkLimitReached` method (an extra write and a new ordering window); a `CrawlResult` struct parameter (cleaner, and it avoids a bare-bool parameter, but it is a wider refactor; note it as a possible follow-up).
*Serves:* AC-5, AC-6.

### D6 — Migration `000003` adds `limit_reached boolean NOT NULL DEFAULT false`
Create it with `make create-migration c=add_limit_reached_to_crawl_jobs`. The up file runs `ALTER TABLE … ADD COLUMN`; the down file runs `ALTER TABLE … DROP COLUMN IF EXISTS`. `Get` selects and scans the new column. `UpdateResult` sets it. `Save` keeps its INSERT unchanged, because the default covers new crawls and they always start `false`.
*Why:* postgresql-table-design *Booleans* and *Safe Schema Evolution* apply. Existing rows read `false` (A-7).
*Rejected:* a nullable column (tri-state is not needed).
*Serves:* AC-5, AC-6.

### D7 — Parsing lives in a new `internal/config` package, called first in `run()`
`config.ParseMaxPages(raw string) (int, error)`: `""` → `0, nil` (unlimited). Otherwise `strconv.Atoi`, and a value `< 1` is rejected. Every error is wrapped with `fmt.Errorf("MAX_PAGES_PER_CRAWL: %w", err)` or carries the variable name. `run()` calls it with `os.Getenv("MAX_PAGES_PER_CRAWL")` **before** the DB connection (main.go:35), so a bad value fails without Postgres. The existing exit path (main.go:24-28) supplies stderr and exit code 1.
*Why:* a pure function can take a table-driven test, while `run()` needs a DB. CLAUDE.md says `cmd/server` is "main.go only, no business logic". `Atoi` already rejects `" 10"`, `"10.5"`, `"abc"`, and out-of-range values (`ErrRange`).
*Rejected:* inline parsing in main.go (untestable); viper (the repo reads env with `os.Getenv`); `os.LookupEnv` (unset and empty mean the same thing here).
*Serves:* AC-1, AC-8.

### D8 — Make the crawler test doubles race-safe and per-crawl
- `mockFetcher`: count fetches per URL behind a `sync.Mutex`, or with an `atomic.Int64` total plus a mutex-guarded map. Add an optional `failURLs` set (returns an error) and an optional `delay` so that workers overlap in flight.
- `mockStore`: put a `sync.Mutex` around recorded state, keep a `map[uuid.UUID]` of `{status, visits, limitReached}`, and replace `close(done)` with sending the crawl ID on a buffered `chan uuid.UUID`. Closing twice panics once two crawls finish (AC-7). Existing tests that do `<-s.done` keep working with a send.
- Logger capture uses `zerolog.New(zerolog.SyncWriter(&buf))`, because a bare `bytes.Buffer` races under several workers.

*Rejected:* `testing/synctest` (available in Go 1.25, but a new concept that the plan does not need).
*Serves:* AC-9, NFR-1, NFR-3.

### D9 — `MemoryCrawlStore` stays without a mutex
No criterion runs it concurrently: crawler tests use `mockStore`, store tests use one goroutine, and production wires Postgres (main.go:52-53). Fixing it is a separate change, in line with the spec's scope discipline (NG-3, NG-4).
*Serves:* none. This is a recorded non-change, reported as a follow-up.

### D10 — Log line: `Info`, keys `"ID"` and `"max_pages"`, message `"max pages per crawl reached"`
The line is emitted by the D4 CAS winner, so it appears on the **first drop**, and a crawl that reaches exactly N pages with nothing left to fetch logs nothing. The `"ID"` key reuses crawler.go:77.
*Serves:* NFR-1.

### D11 — Compose passes the variable through
Add `- MAX_PAGES_PER_CRAWL=${MAX_PAGES_PER_CRAWL:-}` under `app.environment`, which keeps it empty (unlimited) when unset.
*Serves:* AC-1, AC-2 (the operator path under `make up`).

## 3. Tasks

### 3.1 S1 — Contract and storage
Files: `internal/crawler/crawl.go`, `internal/crawler/crawler.go` (the `store` interface and the completion call only), `internal/crawler/crawler_test.go` (`mockStore` signature only), `internal/store/memory_store.go`, `internal/store/memory_store_test.go`, `internal/store/postgres.go`, `migrations/000003_add_limit_reached_to_crawl_jobs.{up,down}.sql`. Depends on: nothing.
- [ ] T1 Add `LimitReached bool` to `Crawl` → AC-5, AC-6 → `TestCrawlStore_UpdateResult` in internal/store/memory_store_test.go
- [ ] T2 Change `UpdateResult` in all four copies (D5). The completion goroutine passes `false` for now, so the build stays green → AC-5, AC-6 → `TestCrawlStore_UpdateResult`, plus `go build ./...`
- [ ] T3 `MemoryCrawlStore.UpdateResult` writes `LimitReached`. New table-driven test `TestCrawlStore_UpdateResult` in internal/store/memory_store_test.go, with rows `limit_reached_true_persisted`, `limit_reached_false_persisted`, `unknown_id_returns_ErrNotFound`, `saved_crawl_reads_false` (asserts with `Get`) → AC-5, AC-6 → `TestCrawlStore_UpdateResult`
- [ ] T4 Migration 000003 up/down (D6) → AC-5, AC-6 → manual check §6 (Postgres)
- [ ] T5 `PostgresCrawlStore.Get` adds `limit_reached` to SELECT and Scan; `UpdateResult` sets it (D6) → AC-5, AC-6 → manual check §6 (Postgres). The repo has no DB test lane.

### 3.2 S2 — Crawler limit
Files: `internal/crawler/crawler.go`, `internal/crawler/crawler_test.go`. Depends on: S1.
- [ ] T6 Make the test doubles race-safe and per-crawl (D8). Existing assertions stay unchanged → AC-9, NFR-3 → `TestCrawler_Run_JobsProcessed` under `make race`
- [ ] T7 Add the `maxPages` parameter to `NewCrawler` (D1) and pass `0` at the five existing test call sites → AC-1, NFR-3 → `TestCrawler_Run_JobsProcessed`, `TestCrawler_Run_MaxPages/unlimited_fetches_every_page`
- [ ] T8 `CrawlTracker` gets `reserved atomic.Int64`, `limitReached atomic.Bool`, and the `tryReserve` / `release` / `markLimitReached` helpers (D2, D4) → AC-2, AC-3 → `TestCrawler_Run_MaxPages`
- [ ] T9 Worker: when limited, reserve before `Fetch`. A failed reservation marks the flag, calls `Done()` and continues. A fetch error releases the slot before `Done()` → AC-2, AC-3, AC-4 → `TestCrawler_Run_MaxPages/cap_holds_with_concurrent_workers`, `/failed_fetch_does_not_count`
- [ ] T10 `Submit`: add the D3 drop check after the prefix filter and before `completeWG.Add(1)` → AC-3, AC-5, AC-7 → `TestCrawler_Run_MaxPages/limit_one_fetches_only_seed`, `TestCrawler_Run_MaxPages_OtherCrawlsContinue`
- [ ] T11 Completion goroutine passes `tracker.limitReached.Load()` to `UpdateResult` → AC-5, AC-6 → `TestCrawler_Run_MaxPages` (all rows assert `limitReached`)
- [ ] T12 CAS winner logs once (D10). Adds no goroutine: if one is added anyway, comment its exit condition → NFR-1, NFR-4 → `TestCrawler_Run_MaxPages_LogsOnce`
- [ ] T13 Write tests in internal/crawler/crawler_test.go (each waits on `mockStore.done`, then asserts recorded `status == done`, `visits`, fetch count and `limitReached`):
  - `TestCrawler_Run_MaxPages`, table-driven:
    - `unlimited_fetches_every_page` (max 0, 2 workers, 6 pages → 6, false)
    - `cap_holds_with_concurrent_workers` (max 5, 4 workers, seed + 20 children, delay > 0 → visits == 5, fetches == 5, true)
    - `limit_one_fetches_only_seed` (max 1 → 1, true)
    - `exact_page_count_reports_false` (max 3, 3 pages → 3, false)
    - `fewer_pages_than_limit_reports_false`
    - `failed_fetch_does_not_count` (1 worker, max 2, seed → a (fails), b, c → visits 2, true)
    - `seed_fetch_fails_reports_false` (max 1 → 0, false)
  - `TestCrawler_Run_MaxPages_OtherCrawlsContinue`: two seeds on distinct hosts, max 3, crawl A has 10 pages and crawl B has 3 → A gives 3/true, B gives 3/false.
  - `TestCrawler_Run_MaxPages_LogsOnce`: SyncWriter buffer, exactly one line with the D10 message whose `ID` equals the crawl ID and whose `max_pages` equals 5.

  → AC-1–AC-7, AC-9, NFR-1 → the tests named

### 3.3 S3 — API field
Files: `internal/api/getCrawl.go`, `internal/api/getCrawl_test.go`. Depends on: S1 (T1).
- [ ] T14 Add `LimitReached bool \`json:"limit_reached"\`` (no `omitempty`, so the field is always present) to `getCrawlResponse` and map it → AC-5, AC-6 → `TestHandler_GetCrawl`
- [ ] T15 In the test type, add `LimitReached *bool` (a pointer, so a missing field decodes as `nil`). New rows: `limit_reached_true_returned` and `limit_reached_false_always_present` (asserts non-nil and false) → AC-5, AC-6 → `TestHandler_GetCrawl`

### 3.4 S4 — Config and wiring
Files: `internal/config/config.go` (new), `internal/config/config_test.go` (new), `cmd/server/main.go`, `.env.example`, `docker-compose.yml`. T16 and T17 have no dependency; T18 depends on S2 (T7).
- [ ] T16 `config.ParseMaxPages` (D7) → AC-1, AC-8 → `TestParseMaxPages` in internal/config/config_test.go
- [ ] T17 `TestParseMaxPages` rows:
  - accepted: `empty_is_unlimited` (`""` → 0), `positive_is_limit` (`"50"` → 50)
  - rejected: `zero_rejected`, `negative_rejected` (`"-5"`), `non_numeric_rejected` (`"abc"`), `decimal_rejected` (`"10.5"`), `surrounding_space_rejected` (`" 10"`), `overflow_rejected` (`"99999999999999999999"`)
  - every rejected row asserts that `err.Error()` contains `MAX_PAGES_PER_CRAWL`

  → AC-8, AC-1 → `TestParseMaxPages`
- [ ] T18 `run()` parses first, before `pgxpool.ParseConfig`, and passes the value to `NewCrawler` → AC-1, AC-2, AC-8 → manual check §6 (AC-8)
- [ ] T19 `.env.example` gets `# Max pages fetched per crawl. Empty = no limit; otherwise a positive integer.` followed by `MAX_PAGES_PER_CRAWL=` → NFR-2 → review
- [ ] T20 Pass the variable through compose (D11) → AC-1, AC-2 → manual check §6 (Postgres)

## 4. Traceability

| AC | Task | Test | Commit |
|---|---|---|---|
| AC-1 | T7, T16, T18, T20 | `TestCrawler_Run_MaxPages/unlimited_fetches_every_page`, `TestCrawler_Run_JobsProcessed`, `TestParseMaxPages/empty_is_unlimited` | — |
| AC-2 | T8, T9, T18, T20 | `TestCrawler_Run_MaxPages/cap_holds_with_concurrent_workers`, `/limit_one_fetches_only_seed` | — |
| AC-3 | T8, T9, T10 | `TestCrawler_Run_MaxPages/cap_holds_with_concurrent_workers` (fetch count == N) | — |
| AC-4 | T9 | `TestCrawler_Run_MaxPages` (every row asserts `status == done`), `/seed_fetch_fails_reports_false` | — |
| AC-5 | T1–T5, T10, T11, T14, T15 | `TestCrawler_Run_MaxPages/cap_holds_with_concurrent_workers`, `/failed_fetch_does_not_count`, `TestCrawlStore_UpdateResult/limit_reached_true_persisted`, `TestHandler_GetCrawl/limit_reached_true_returned`, manual Postgres check | — |
| AC-6 | T1–T5, T11, T14, T15 | `TestCrawler_Run_MaxPages/exact_page_count_reports_false`, `/seed_fetch_fails_reports_false`, `TestCrawlStore_UpdateResult/saved_crawl_reads_false`, `TestHandler_GetCrawl/limit_reached_false_always_present` | — |
| AC-7 | T10 | `TestCrawler_Run_MaxPages_OtherCrawlsContinue` | — |
| AC-8 | T16, T17, T18 | `TestParseMaxPages` (rejected rows), manual AC-8 check | — |
| AC-9 | T6, T13 | `make race` (includes `cap_holds_with_concurrent_workers`) | — |
| NFR-1 | T12 | `TestCrawler_Run_MaxPages_LogsOnce` | — |
| NFR-2 | T19 | review of `.env.example` | — |
| NFR-3 | T6, T7 | existing `TestCrawler_*`, `TestHandler_*`, `TestCrawlStore_*` with unchanged status/visits assertions | — |
| NFR-4 | T12 | review: no new goroutine, or a documented exit; `make race` | — |

## 5. Execution

Order: **S1 → S2 → S4 (T18)**. **S3** can start after T1. **T16/T17** can start at any time.

- S1 must land first, because it changes the four-copy `UpdateResult` contract in one go so that `go build ./...` stays green.
- Inside S2, order T6 → T7 → T13 (tests first, red) → T8–T12 (green). This red-green flow suits a learner.
- Parallel groups once S1 is done: {S2}, {S3}, {S4: T16, T17, T19, T20}. They touch disjoint files. T18 (main.go) waits for S2's `NewCrawler` signature.

## 6. Verification

Repo commands (Makefile):
- `make vet` (runs `go fmt ./...`, then `go vet ./...`)
- `make lint` (`golangci-lint run`)
- `make test` (`go test -v ./...`)
- `make race` (`go test -race ./...`), required for AC-9
- Focused: `go test -race -count=1 -run MaxPages ./internal/crawler`

Manual checks:
- **AC-8:** run `MAX_PAGES_PER_CRAWL=0 go run ./cmd/server; echo $?` and expect a non-zero exit with a stderr line naming `MAX_PAGES_PER_CRAWL`. Repeat with `-5`, `abc` and `10.5`. No DB is needed, because of D7.
- **Postgres (T4, T5, T20):**
  1. Put `MAX_PAGES_PER_CRAWL=3` in `.env`, then `make up-build`. The `migrate` service applies 000003.
  2. `POST /crawls` with a multi-page site. Once `GET /crawls/{id}` shows `done`, expect `visits` ≤ 3 and `limit_reached: true`.
  3. Run migrate `down 1`, then `up`, to confirm the down migration is reversible.
