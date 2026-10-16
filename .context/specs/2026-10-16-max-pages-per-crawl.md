# Spec: Maximum pages per crawl
Spec ID: SPEC-01
Status: approved
Supersedes: —

## Problem and user

An operator who runs the crawler server cannot bound how much work one crawl
does. Today a crawl keeps following in-scope links (URLs that start with the
seed URL, `internal/crawler/crawler.go:120`) until it finds no new ones. On a
large site that means unbounded fetches, unbounded frontier pressure on the
shared 500-slot frontier (`internal/crawler/crawler.go:50`), and an
unpredictable crawl time. CLAUDE.md lists "configurable ... max pages per
crawl" as a crawler feature, but no limit exists yet.

What ships today, relevant to this change:
- **Crawl loop.** A worker pool of `WORKERS_COUNT` goroutines reads one
  frontier channel that all crawls share (`internal/crawler/crawler.go:58-112`).
  Each crawl tracks its own pending work and, once that work is drained, a
  completion goroutine sets the status to `done` and then writes `duration`
  and `visits` (`internal/crawler/crawler.go:135-146`).
- **What `visits` counts.** `visits` goes up by one only when `Fetch` returns
  no error (`internal/crawler/crawler.go:80-89`). The fetcher does not check
  the HTTP status code, so 4xx and 5xx responses count as fetched; transport
  errors and HTML tokenizer errors do not (`internal/fetcher/fetcher.go:29-49`).
- **Depth.** The README describes a crawl depth (`README.md:3`), but no depth
  limit exists in code. `robots.txt` handling and rate limiting do not exist
  either.
- **Config.** Config comes from environment variables read in `run()`.
  `WORKERS_COUNT` is parsed with `strconv.Atoi`, and an invalid or non-positive
  value fails startup (`cmd/server/main.go:55-62`). `.env.example` lists
  `WORKERS_COUNT` and the DB variables only (`.env.example:1-5`).
- **Storage.** `crawl_jobs` has the columns `status, url, duration, visits,
  created_at, completed_at` (`migrations/000002_create_crawl_jobs_table.up.sql`).
  `status` is free text. The in-memory store mirrors the same fields
  (`internal/store/memory_store.go`).
- **API.** `GET /crawls/{id}` returns `id, status, duration, visits`
  (`internal/api/getCrawl.go:13-18`).

## Goals / Non-goals

Goals
- G-1: An operator can cap the number of pages any single crawl fetches by
  setting `MAX_PAGES_PER_CRAWL`. Leaving it unset keeps today's unlimited
  behaviour.
- G-2: A crawl that hits the cap stops fetching and still reaches a final
  `done` status.
- G-3: An API client can tell whether a finished crawl was cut short by the
  cap.
- G-4: A bad value fails fast at startup instead of silently changing crawl
  behaviour.
- G-5: The cap holds exactly under concurrent workers, and the crawler stays
  free of data races (`go test -race`).

Non-goals
- NG-1: A per-request override, such as a `max_pages` field on `POST /crawls`.
  The limit is global server config only.
- NG-2: Implementing crawl depth, `robots.txt` handling or rate limiting. This
  spec only states how the limit will interact with them (see Assumptions).
- NG-3: Fixing the crawler-wide visited set. URLs are deduplicated across all
  crawls, not per crawl (`internal/crawler/crawler.go:21,115`). A URL dropped
  because of the limit therefore stays marked as visited for later crawls.
- NG-4: Changing the existing completion order, where `status` becomes `done`
  before `visits` and `duration` are written
  (`internal/crawler/crawler.go:137-141`).
- NG-5: Changing shutdown behaviour for crawls still running when the server
  stops.
- NG-6: Exposing the configured limit value in the API response.
- NG-7: Reloading the limit at runtime. A change takes effect on restart.

## Assumptions

The user asked for reasonable assumptions instead of blocking questions. Each
assumption below is a decision the user can overturn when approving this spec.

| ID | Assumption | Rationale |
|---|---|---|
| A-1 | A **fetched page** is a URL whose fetch returned without error, the same event that increments `visits` today. That includes non-2xx HTTP responses, because the fetcher does not tell them apart. Transport errors, timeouts and HTML parse errors do not count. URLs skipped by dedup or the seed-prefix filter do not count. | This keeps a single meaning for "page" across the API (`visits`) and the limit, so `visits <= MAX_PAGES_PER_CRAWL` is directly observable. Failed fetches yield no links, so they cannot grow a crawl. |
| A-2 | The cap is **exact**: a crawl never records more than `MAX_PAGES_PER_CRAWL` fetched pages, whatever `WORKERS_COUNT` is. Fetches already in flight when the cap is reached are allowed to finish, not cancelled. The crawler starts no fetch that could take the count past the cap. | An exact cap is the only version that a test can assert. Cancelling in-flight HTTP requests adds complexity and gains nothing, because the exact cap already bounds the count. |
| A-3 | Holding the cap exact can leave a crawl with fewer than `MAX_PAGES_PER_CRAWL` visits after it stopped because of the limit. This happens when URLs were dropped while the remaining budget was held by in-flight fetches that then failed. This outcome is accepted. | The alternative is to hold URLs until in-flight fetches settle. That adds waiting and complexity for a rare case, and the result stays within the cap. |
| A-4 | Once the cap is reached for a crawl, the crawl stops queueing newly discovered URLs. Its URLs already in the frontier are discarded without being fetched. The crawl finishes as soon as its in-flight fetches complete. | This matches the request: "stops queueing new URLs and finishes". |
| A-5 | A crawl that stops because of the cap ends with status `done`, not `failed` or a new status. A new boolean field `limit_reached` on `GET /crawls/{id}` reports that the crawl was cut short. | Hitting a configured cap is a successful, intended outcome. A new status value would break clients that wait for `done`. A separate field keeps `status` meaning lifecycle only. |
| A-6 | `limit_reached` is `true` only when at least one in-scope, not-yet-visited URL of the crawl went unfetched because of the cap. A site with exactly `MAX_PAGES_PER_CRAWL` reachable pages gives `limit_reached: false`. It is `false` for unlimited crawls, for crawls still running, and for crawls stored before this change. | The field answers the useful question "are there pages I did not get?". This is more informative than repeating `visits == limit`. |
| A-7 | `limit_reached` is persisted with the crawl in both stores. The Postgres schema gains one non-null boolean value per crawl that defaults to false, so existing rows read as `false`. That needs a new forward migration with a matching down migration. | The API reads crawls from the store, and the value cannot be derived later because the configured limit may change between restarts. |
| A-8 | `MAX_PAGES_PER_CRAWL` is read once at startup. Unset or empty means no limit. A positive base-10 integer that fits in an `int` is the limit. `0`, negative numbers, non-numeric text, values with surrounding whitespace and out-of-range numbers are startup errors. | This follows the `WORKERS_COUNT` convention (`cmd/server/main.go:55-62`). Treating `0` as "unlimited" would give one setting two spellings and hide typos. The user defined "unlimited" as unset. |
| A-9 | The limit is global, from the environment only. A `max_pages` field sent in a `POST /crawls` body is ignored, as all unknown fields already are (`internal/api/createCrawl.go:25`). | The user said "Configure it with MAX_PAGES_PER_CRAWL". A per-request override is a separate feature (NG-1). |
| A-10 | The limit applies to each crawl independently. Reaching it in one crawl does not affect other crawls that share the worker pool and frontier. | "Per crawl" is in the variable name. All crawls share one frontier, so the limit must not stall it. |
| A-11 | Depth (when built) and the page cap are independent stop conditions, and whichever is hit first stops the crawl from following links. A URL rejected by depth is not "dropped because of the limit" for `limit_reached`. Once `robots.txt` is built, a disallowed URL is never fetched and does not count. | Depth and `robots.txt` do not exist yet. Recording the intended interaction keeps the later specs consistent. |
| A-12 | The seed URL is always the first page fetched. With `MAX_PAGES_PER_CRAWL=1`, only the seed is fetched. | The seed is queued at submit time (`internal/api/createCrawl.go:59`), before any other URL exists. |

## User stories

- US-1: As an operator, I want to set `MAX_PAGES_PER_CRAWL` so that no single
  crawl fetches more pages than I allow.
- US-2: As an API client, I want to see whether a finished crawl was cut short
  by the limit so that I know the results are partial.
- US-3: As an operator, I want a bad limit value to stop the server at startup
  so that I find a misconfiguration before crawls run with it.

## Acceptance criteria (EARS)

- **AC-1** (US-1): While `MAX_PAGES_PER_CRAWL` is unset or empty, the crawler
  shall fetch every in-scope URL it discovers for a crawl, with no cap on
  `visits`.
- **AC-2** (US-1): While `MAX_PAGES_PER_CRAWL` is set to N, the crawler shall
  record at most N fetched pages per crawl, so `GET /crawls/{id}` returns
  `visits` ≤ N for any `WORKERS_COUNT` ≥ 1.
- **AC-3** (US-1): When a crawl's fetched-page count reaches N, the crawler
  shall not start a fetch for any further URL of that crawl.
- **AC-4** (US-1): When a crawl stops because of the limit, the crawler shall
  set that crawl's status to `done` after its in-flight fetches complete.
- **AC-5** (US-2): When a crawl finishes after at least one of its in-scope,
  unvisited URLs went unfetched because of the limit, `GET /crawls/{id}` shall
  return `"limit_reached": true`.
- **AC-6** (US-2): When a crawl finishes without any URL going unfetched
  because of the limit, `GET /crawls/{id}` shall return
  `"limit_reached": false`.
- **AC-7** (US-1): While one crawl is stopped by the limit, the crawler shall
  continue to fetch the queued URLs of other crawls that are still running.
- **AC-8** (US-3): If `MAX_PAGES_PER_CRAWL` is `0`, negative, or not a
  positive base-10 integer, then the server shall exit with a non-zero status
  before it accepts HTTP connections, with an error message on stderr that
  names `MAX_PAGES_PER_CRAWL`.
- **AC-9** (US-1): When `go test -race ./...` runs, the crawler test suite
  shall pass, including a test that crawls with `MAX_PAGES_PER_CRAWL` set and
  at least 2 workers, and with more in-scope pages than the limit.

AC-1 to AC-9 is one more than the usual count for a single setting, because the
request bundles a config contract (AC-1, AC-8), a crawl stop rule (AC-2 to
AC-4, AC-7), an API field (AC-5, AC-6) and a race requirement (AC-9).

### External contract: `GET /crawls/{id}`

```json
{ "id": "…", "status": "done", "duration": 1234, "visits": 50, "limit_reached": true }
```

`limit_reached` is always present and is a JSON boolean. `POST /crawls` is
unchanged.

## Edge cases

| Case | Expected behaviour | Covered by |
|---|---|---|
| `MAX_PAGES_PER_CRAWL=1` | Only the seed is fetched. If the seed has in-scope links, `limit_reached: true`. | AC-2, AC-5, A-12 |
| `MAX_PAGES_PER_CRAWL=1` and the seed fetch fails | `visits: 0`, status `done`, `limit_reached: false` (no links were discovered) | AC-4, AC-6, A-1 |
| Site has exactly N reachable in-scope pages | All N are fetched, `limit_reached: false` | AC-6, A-6 |
| Site has fewer than N pages | Same as today: every page is fetched, `limit_reached: false` | AC-1, AC-6 |
| Limit is reached while other workers are mid-fetch for the same crawl | Those fetches finish and none takes `visits` past N. Status becomes `done` after they finish. | AC-2, AC-4, A-2 |
| In-flight fetches fail after URLs were dropped for the limit | `visits` < N, `limit_reached: true` | AC-5, A-3 |
| Fetch returns 404 or 500 | Counts as a fetched page toward N | AC-2, A-1 |
| Transport error or timeout | Does not count toward N | AC-2, A-1 |
| URL already visited (dedup) or outside the seed prefix | Not counted. Not "dropped because of the limit". | A-1, A-6 |
| Two crawls running, one hits its limit | The other keeps fetching up to its own N | AC-7, A-10 |
| `MAX_PAGES_PER_CRAWL=0` | Startup error | AC-8, A-8 |
| `MAX_PAGES_PER_CRAWL=-5`, `abc`, `10.5`, `" 10"`, or larger than `int` | Startup error | AC-8, A-8 |
| `MAX_PAGES_PER_CRAWL=` (empty string) | Unlimited | AC-1, A-8 |
| Client sends `{"url": "…", "max_pages": 5}` | The field is ignored and the global limit applies | NG-1, A-9 |
| `GET` on a crawl still `in_progress` | `"limit_reached": false` | AC-6, A-6 |
| `GET` on a crawl stored before this change | `"limit_reached": false` | A-6, A-7 |
| `GET` between the `done` status write and the result write | `visits` and `limit_reached` may still read their initial values, as `visits` does today | NG-4 |
| A URL dropped for the limit is later submitted by another crawl | It is still deduplicated by the crawler-wide visited set and not fetched | NG-3 |
| Server receives SIGTERM while a limited crawl is running | Today's shutdown behaviour applies | NG-5 |
| Operator changes `MAX_PAGES_PER_CRAWL` and restarts | New crawls use the new value. Finished crawls keep their stored `limit_reached`. | NG-7, A-7 |

## Non-functional requirements

- NFR-1 (observability): When a crawl first reaches its limit, the crawler
  shall emit exactly one structured info-level log line for that crawl. The
  line carries the crawl ID and the configured limit, using the crawl-ID key
  the crawler already uses for log context.
- NFR-2 (configuration): `.env.example` shall list `MAX_PAGES_PER_CRAWL` with
  an empty value and a comment stating that empty means no limit and that the
  value must otherwise be a positive integer, as CLAUDE.md requires for every
  variable.
- NFR-3 (compatibility): With `MAX_PAGES_PER_CRAWL` unset, every existing test
  in `internal/crawler`, `internal/api` and `internal/store` shall pass with
  unchanged assertions about `status` and `visits`.
- NFR-4 (concurrency): Every goroutine added for this feature shall have a
  documented exit condition, as CLAUDE.md requires.

## Inputs and provenance

| Input | Provenance tag | Note |
|---|---|---|
| `MAX_PAGES_PER_CRAWL` | [deterministic: server config at startup] | Parsed once and validated per A-8 |
| Fetch outcome (success or error) | [deterministic: `internal/fetcher`] | Decides whether a page counts (A-1) |
| Extracted links | [deterministic: `internal/fetcher` HTML tokenizer] | Source of the URLs that may be dropped by the limit |
| Crawl record (`status`, `visits`, `limit_reached`) | [reused: stored crawl from `POST /crawls` and crawl completion] | Read by `GET /crawls/{id}` |

No model calls are involved.

## Untrusted inputs

- **Fetched page bodies and the links extracted from them** are third-party
  content. They are data, never instructions. They can influence which URLs
  get queued, but never the limit, the count or `limit_reached`. They are not
  rendered back by the API.
- **`POST /crawls` request bodies** are user-submitted. A `max_pages` field or
  any other extra field has no effect (A-9).
- **`MAX_PAGES_PER_CRAWL`** is operator-controlled but still validated. A
  malformed value is rejected at startup (AC-8), never coerced.

## Open questions

None. Every gap was resolved as an assumption (A-1 to A-12) at the user's
request. The user can overturn any of them when approving this spec.
