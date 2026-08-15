# Server dependencies

> **Authored autonomously by an AI agent (Claude) with minimal human oversight.**

What the dashboard needs from `Togather-Foundation/server`, why, and where it is tracked.

The purpose of this register is to make the requirements conversation tractable: each row states the end it serves ([RFC 002](rfc-002-teloi.md)), so a discussion about priority is a discussion about which end matters, not about which feature someone fancies. Rows without a telos should not be built.

Last verified against `staging.toronto.togather.foundation` on 2026-08-15. Consolidated for discussion in [server#26](https://github.com/Togather-Foundation/server/issues/26).

## Blocking

Things without which a stated end cannot be served at all.

| # | Need | Serves | Tracked | Notes |
|---|---|---|---|---|
| B1 | **Admin credentials for a staging node** | T1, T5 | *not an issue — a process/access question* | The single largest blocker. `internal/sel` has typed clients for `/admin/scraper/sources` and `/admin/reports/daily-usage` written from the OpenAPI schema and marked `UNVERIFIED`. Nothing about T1 can be built or tested without a credential. This is an access decision, not engineering work, and should be resolved first because it is free. |
| B2 | **Per-record source attribution** | T2, T1 | [#22](https://github.com/Togather-Foundation/server/issues/22) | Neither the events API nor the change feed emits `scraperSource` or `sourceUrl`. Without it, no record can be traced to the scraper that produced it, so "which source is degrading" and "where did this claim come from" are both unanswerable from the public tier. |
| B3 | **Meaningful license variation** | T2 | [#20](https://github.com/Togather-Foundation/server/issues/20) finding 8 | Every record on staging is `cc0`, including third-party listings. A constant field carries no information. This is a data-correctness question before it is a dashboard question, and it may be the most consequential item here for the project's external claims. |

## Substantive

Things that materially improve an end without blocking it.

| # | Need | Serves | Tracked | Notes |
|---|---|---|---|---|
| S1 | **Aggregate counts on public collections** | T3, T4 | [#24](https://github.com/Togather-Foundation/server/issues/24) | The coverage panel paginates up to 12 × 200 records per load to compute figures the server could return directly. Works at current volume; will not scale, and the `truncated` flag already exists to admit when it gives up. |
| S2 | **Place data quality: names and geo** | T3 | [#23](https://github.com/Togather-Foundation/server/issues/23) | **779 of 1,200 places (64%) have no `geo`**, so geographic gap analysis — the most actionable form of T3 — is impossible. (103 of 186 when restricted to venues with events in a 30-day window; the collection-wide figure is the one that matters.) 77 records also carry the name-concatenation defect, which splits one venue across two keys unless normalised client-side, as `internal/coverage` currently does. |
| S3 | **HTML entities stored in place names** | T3 | [#23](https://github.com/Togather-Foundation/server/issues/23) | **76 place records** carry names with escapes intact (`&#8211;`, `&#8217;`, `&amp;`), surfacing in 41 events over a 30-day window. Every API consumer inherits this. Same class as S2. |
| S4 | **Run history per source** | T1 | *exists: `/admin/scraper/sources/{name}/runs`* | Needed for T1's baseline comparison. Unverified pending B1. |

## Watching

Not needed yet, but decisions here constrain the dashboard's architecture.

| # | Need | Serves | Notes |
|---|---|---|---|
| W1 | **Ingestion latency is not currently measurable** | T1 | The change feed exposes `source_timestamp` and `received_timestamp`, which would give lag between an event being published and ingested. In a 200-change sample they are **identical in every record**, so the pair currently carries no signal. Either the source timestamp is not being captured, or it is being overwritten with receipt time. Worth confirming intent before building on it. |
| W2 | **History retention** | T4 | The change feed's `sequence_number` is monotonic and reaches back to 2026-03-25, so event history is reconstructible. But it is a stream over events only — there is no history for source health or coverage. If T4 matters, the dashboard must persist its own observations. See RFC 002, open question 2. |
| W3 | **Out-of-region records** | T2, T3 | The Toronto node's places collection contains at least one Zürich venue. Either a federation artefact or a scraper leak; either way it affects coverage denominators and the meaning of "this node covers Toronto". |
| W4 | **CORS policy** | — | [#25](https://github.com/Togather-Foundation/server/issues/25). Not a data dependency, but RFC 001's design is deliberately built so that fixing it requires no dashboard change. Flagged so that stays true. |

## Already resolved by discovery

Recorded so the same ground is not re-covered:

- **License status on the public tier** — available now via `/api/v1/feeds/changes` (`license_status`, `license_url`), unauthenticated. Earlier assessments treated all provenance as blocked by #22; that is only true of the events endpoint.
- **Operational endpoints exist** — source health, diagnostics, review queue, and usage are all served today. The gap is access (B1), not implementation.
- **API documentation exists** — `/api/v1/openapi.json` fully specifies the surface. The thin MCP tool descriptions are a separate, narrower problem ([#20](https://github.com/Togather-Foundation/server/issues/20)).

## Suggested sequence

1. **B1** — costs nothing, unblocks the most.
2. **B3** — a correctness question with external consequences, and cheap to decide even if not cheap to fix.
3. **S2 + S3** — one batch of data repair, unblocks the most actionable part of T3.
4. **B2** — larger, and the payoff is mostly T2 which B3 may reframe.
5. **S1** — only when volume demands it.
