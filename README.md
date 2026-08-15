# dashboard

Operational reporting for a [Togather](https://togather.foundation) SEL node — data provenance, coverage, source health, and API usage.

**Status: MVP. Coverage panel is live against real data; three panels are stubs pending credentials or upstream fixes.**

## What this is for

Two audiences, in priority order:

1. **Operational** — is ingestion healthy? Which sources have gone quiet, which venues have no upcoming events, where is coverage thinning out? The useful signal here is *absence*, not totals. Totals only ever go up; gaps tell you what to do on Monday.
2. **Provenance** — where did this data come from, under what license, with what confidence, and how fresh is it? This is the claim the SEL makes about itself, and it should be inspectable.

API usage and product analytics come later, once there is more to measure.

## Running it

```sh
cp .env.example .env     # optional: admin credentials
make run                 # → http://127.0.0.1:8080
```

Without credentials it runs in public-only mode: the coverage panel works fully,
and the admin panels report why they are unavailable rather than failing.

`make check` runs fmt, vet and tests.

## Architecture

```
cmd/bff/          Go service. Holds the admin JWT; serves web/ and /api/*.
internal/sel/     Typed client for the node's HTTP API, split public/admin.
internal/coverage/ Aggregation over public event data.
web/              Static frontend. No build step, no dependencies.
```

The BFF exposes a fixed, enumerated set of read-only endpoints. There is no
general-purpose passthrough — adding one is an explicit edit, so the privileged
surface stays auditable. It refuses to bind to anything but loopback, because
this MVP has no browser-facing authentication yet.

## Panel status

| Panel | State | Blocker |
|---|---|---|
| Coverage & gaps | **Live** — real data, no credentials needed | — |
| Source health | Stub — client written, untested | Needs admin credentials |
| API usage | Stub — client written, untested | Needs admin credentials |
| Provenance mix | Stub — reports the blockage | [server#22](https://github.com/Togather-Foundation/server/issues/22) |

The admin clients in `internal/sel` were written from the OpenAPI schema and are
marked `UNVERIFIED` — no credentials were available when they were authored, so
their response shapes have not been confirmed against a live node.

## Before extending it

Read [`docs/rfc-001-auth-posture.md`](docs/rfc-001-auth-posture.md). Nearly all of this app's data sits behind `/admin/*`, which means — unlike [`web-viewer`](https://github.com/Togather-Foundation/web-viewer) — it cannot be a static site without a security tradeoff. That decision determines the deployment topology, so it should be settled before the first commit of application code.

The RFC recommends a thin backend-for-frontend. It is a draft and the open questions at the end are real ones.

## What the node already provides

Most of the feature set has endpoints today (see `/api/v1/openapi.json` on any node):

| Need | Endpoint | Auth |
|---|---|---|
| Source health & run history | `/admin/scraper/sources`, `/admin/scraper/sources/{name}/runs` | JWT |
| Ingestion diagnostics | `/admin/scraper/diagnostics` | JWT |
| Review queue | `/admin/review-queue` | JWT |
| API usage | `/admin/reports/daily-usage` | JWT |
| Incremental changes | `/feeds/changes` | none |
| Event & place coverage | `/events`, `/places` | none |

The unauthenticated rows are worth noting: coverage and freshness panels can be built with no credentials at all, which the RFC leans on.

## Known upstream gaps

- [server#22](https://github.com/Togather-Foundation/server/issues/22) — the public API declares a `sel:*` provenance vocabulary it never emits. Admin endpoints cover most of what this dashboard needs, so this is not a blocker, but it does mean public-tier provenance panels are currently impossible.
- [server#24](https://github.com/Togather-Foundation/server/issues/24) — no aggregate counts on public collections; coverage maths over the public tier means paginating.
- [server#25](https://github.com/Togather-Foundation/server/issues/25) — open CORS policy. Do not build anything that depends on it; see the RFC.

## License

Not yet chosen — no other `Togather-Foundation` repository declares one, so this is an org-level decision.
