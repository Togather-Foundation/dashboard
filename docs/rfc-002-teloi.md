# RFC 002 — What the dashboard is for

> **Authored autonomously by an AI agent (Claude) with minimal human oversight.**

- **Status:** Draft, for discussion
- **Date:** 2026-08-15
- **Scope:** The ends the dashboard serves, and what each requires from the server
- **Supersedes:** the two-line "what this is for" in README.md

## The problem this RFC exists to fix

The dashboard currently has a feature list, not a purpose. Its panels were chosen by what the API made available — coverage got built first because it needed no credentials, not because anyone decided it was the most important question. That is a reasonable way to start and a bad way to continue: it produces a page that displays things rather than one that settles questions.

The test this RFC applies to every panel: **who does what differently because of it?** A panel that no one acts on is a screensaver, however accurate.

## Proposed ends

Five, in priority order. The first three are what an MVP should serve; the last two are deferred but shape the design now.

---

### T1 — Notice ingestion has broken before a data consumer does

**Decision:** whether to intervene in a scraper today.
**Who:** whoever is on the hook for the node.
**Cadence:** daily glance, or push.

The failure mode that matters is silent: a source stops returning results, nothing errors, and the library simply thins out. Nobody notices until a consumer asks where the listings went. The signal is *absence*, which is why totals are the wrong instrument — they degrade gracefully and invisibly.

**Needs:** per-source last-run status and yield, plus the ability to compare today's yield to that source's own recent normal. A source that usually returns 200 events and returned 3 is broken even though it "succeeded".

**Status:** endpoint exists (`/admin/scraper/sources`), unverified, needs credentials. Historical comparison needs either retention on our side or run history from `/admin/scraper/sources/{name}/runs`.

---

### T2 — Decide whether the library is fit to publish or federate

**Decision:** can we point a partner, a consumer, or another node at this data and stand behind it?
**Who:** whoever represents the project externally.
**Cadence:** before each such commitment.

This is the provenance question, and it is the project's whole value proposition. A commons that cannot answer "where did this come from and what may you do with it" is just a scraper with good intentions.

**Needs:** license distribution, confidence distribution, source attribution per record, and a defensible quality floor.

**Status:** **partially unblocked, contrary to earlier assessment.** The public change feed (`/api/v1/feeds/changes`) carries `license_status` and `license_url` per change, so license mix is computable today without credentials. What is still missing is per-record source attribution — neither the events API nor the change feed emits `scraperSource` or `sourceUrl`. See `server-dependencies.md`.

Note the current answer to T2 is uncomfortable and worth stating plainly: **every record on staging is stamped `cc0`, including third-party listings.** A license field that is constant carries no information, so this telos is currently unanswerable not because data is missing but because the data is uniform by construction.

---

### T3 — Direct scarce contributor effort at the highest-value gap

**Decision:** what should a volunteer work on this week?
**Who:** whoever coordinates contributors.
**Cadence:** weekly.

A civic project's binding constraint is attention. The dashboard earns its keep if it converts "the data could be better" into a specific, finite, assignable task: *this venue has gone quiet*, *this district has no coverage*, *103 venues cannot be mapped*.

**Needs:** venue-level and geographic gap detection, ranked by something like impact rather than alphabetically.

**Status:** partially live. Quiet venues and missing-geo counts work today. Ranking by impact needs a notion of venue importance the API does not currently express.

---

### T4 — Show the commons is working *(deferred)*

**Decision:** funding, partnership, contributor recruitment.
**Who:** whoever writes the applications.
**Cadence:** occasional, but the data must be continuous.

Deferred as a panel, but it constrains T1–T3 now: **it requires history.** A snapshot cannot show growth. The change feed's monotonic `sequence_number` reaches back to 2026-03-25, so history exists and is queryable — but only as a stream, and only for events. If T4 matters, decide early whether the dashboard retains its own time series rather than recomputing from a feed each time.

---

### T5 — Understand who consumes the data *(deferred)*

**Decision:** what to build next, and for whom.
**Who:** product direction.

`/admin/reports/daily-usage` and per-key usage exist. Deferred because there is little consumption to measure yet, and because measuring it well means deciding what counts as a meaningful use rather than a request count.

---

## What this changes about the MVP

The current build serves **T3** well, **T1** not at all (no credentials), and **T2** barely. The `activeVenues`/`quietVenues` framing is right for T3. The data-quality tiles are the beginning of T2's quality floor.

Three concrete consequences:

1. **The provenance panel should not stay a stub.** License mix is computable from the public change feed today. Building it would surface the uniform-`cc0` problem visibly, which is more useful than a placeholder pointing at server#22.
2. **T1 needs a baseline, not a status.** "Last run: success" is nearly worthless without "and it returned a tenth of its usual yield". Design for comparison from the start.
3. **History is a now-decision, not a later one.** If T4 is real, the dashboard needs to persist observations. That is the difference between a stateless proxy and a service with a datastore, and it is much cheaper to decide before than after.

## Open questions

1. **Is T1 or T2 the priority?** They pull in different directions — T1 wants admin credentials and operational plumbing; T2 wants public-tier provenance and could stay credential-free. Doing both well at MVP is probably not realistic.
2. **Does the dashboard persist anything?** See consequence (3). This is the single largest architectural fork remaining after RFC 001.
3. **Who actually looks at this, and how often?** Every cadence above is an assumption. If the honest answer is "one person, occasionally", that argues for push notification on T1 rather than a page anyone must remember to open.
4. **Is a uniform `cc0` stamp acceptable?** If yes, T2 collapses to a much smaller question. If no, it is a server problem before it is a dashboard problem.

## Related

- [`rfc-001-auth-posture.md`](rfc-001-auth-posture.md) — how the dashboard authenticates
- [`server-dependencies.md`](server-dependencies.md) — what each telos needs from the server, and where it is tracked
