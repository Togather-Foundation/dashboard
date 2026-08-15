# RFC 001 — Authentication posture for the dashboard

> **Authored autonomously by an AI agent (Claude) with minimal human oversight.**

- **Status:** Draft, for discussion
- **Date:** 2026-08-15
- **Scope:** How `dashboard` authenticates against a SEL node, and where credentials live
- **Decision needed before:** any code that touches `/admin/*`

## Why this needs deciding first

The dashboard's subject matter is operational: source health, review queues, ingestion diagnostics, API usage. Nearly all of it sits behind `/admin/*`. Unlike [`web-viewer`](https://github.com/Togather-Foundation/web-viewer) — which reads a genuinely public, unauthenticated API and therefore needs no backend at all — this app cannot be a pure static site without making a security tradeoff we would regret.

Getting this wrong is expensive to undo, because it determines the deployment topology, not just a library choice.

## What the node actually offers

From `/api/v1/openapi.json` on a running node, the endpoints split into three tiers:

| Tier | Count | Auth | Examples |
|---|---|---|---|
| Public | 20 | none | `GET /events`, `GET /feeds/changes`, `GET /events.ics`, `GET /.well-known/sel-profile` |
| Admin | 31 | `bearerAuth` (JWT) | `/admin/review-queue`, `/admin/scraper/sources`, `/admin/reports/daily-usage`, `/admin/developers` |
| Developer | 5 | `devJWT` / `devCookie` | `/api/v1/dev/api-keys`, `/api/v1/dev/api-keys/{id}/usage` |

Five schemes are declared: `apiKey`, `bearerAuth`, `cookieAuth`, `devCookie`, `devJWT`.

Three facts shape the decision:

1. **Admin is overwhelmingly JWT-bearer, not cookie.** 31 of 32 admin operations use `bearerAuth`. A bearer token is sent deliberately by the client, so it is not subject to CSRF the way an ambient cookie is.

2. **Exactly one endpoint requires a cookie.** `GET /admin/scraper/events` uses `cookieAuth`, described in the spec as "used by same-origin SSE endpoints." This is not an oversight — the browser `EventSource` API cannot set an `Authorization` header, so server-sent events must authenticate ambiently. It is the single most awkward endpoint in the design and it drives much of what follows.

3. **A meaningful slice of dashboard data needs no auth at all.** `/feeds/changes` and `/events` are public. Coverage and freshness panels can be built against them without any credential.

## Constraint: the open CORS policy

The node currently reflects any `Origin` with `Access-Control-Allow-Credentials: true` ([server#25](https://github.com/Togather-Foundation/server/issues/25)). A credentialed cross-origin preflight for `DELETE /admin/events/{id}` from an arbitrary origin returns 204.

This matters here in two directions:

- **We must not build on it.** An architecture that works *because* CORS is wide open will break the moment #25 is fixed with a proper allowlist. Any cross-origin design must assume the dashboard's origin will need explicit allowlisting.
- **We should not add to the risk.** Putting a long-lived admin session in a browser that also talks to arbitrary origins under the current policy widens the blast radius of #25 rather than containing it.

The safest posture is one where fixing #25 requires no change to the dashboard at all.

## Options

### A. Pure SPA, admin JWT held in the browser

The dashboard is static files. It calls `POST /admin/login`, stores the JWT, and attaches it to `/admin/*` calls cross-origin.

- Simplest to deploy — static hosting, same as `web-viewer`.
- **The admin JWT is reachable from JavaScript.** Any XSS anywhere in the app — including via a dependency — yields full node admin. Review queues and scraper controls are not low-value targets; `POST /admin/api-keys` returns a usable key in its response body.
- Requires the node to allowlist the dashboard origin with credentials, so it is permanently coupled to CORS policy.
- Cannot use the SSE endpoint at all cross-origin without cookies, which reintroduces CSRF exposure.
- Token refresh and logout are hard to do well client-side.

**Assessment:** rejected. The convenience is real, but "admin credentials in browser storage, cross-origin, on a node with an open CORS policy" is the combination that turns one XSS into a full compromise.

### B. Thin backend-for-frontend (BFF) — *recommended*

A small server component sits between the browser and the node. It holds admin credentials server-side, exposes a narrow same-origin API to its own frontend, and issues the browser an ordinary `HttpOnly`, `SameSite=Strict` session cookie.

```
browser ──same-origin, session cookie──▶ BFF ──bearerAuth JWT──▶ SEL node /admin/*
   │
   └──────direct, no credentials───────▶ SEL node /events, /feeds/changes
```

- **The admin JWT never reaches the browser.** XSS in the dashboard cannot exfiltrate it.
- **No CORS involvement for admin traffic** — it is same-origin to the BFF, and the BFF is a server-side client. Fixing #25 requires no dashboard change.
- The BFF can expose only the operations the dashboard needs. Nothing forces it to proxy `DELETE /admin/events/{id}` just because the node offers it.
- SSE works naturally: the BFF holds the upstream connection and re-emits to its own origin, so `cookieAuth` stays same-origin as intended.
- Public data can still be fetched straight from the node, keeping the BFF off the hot path for the read-heavy panels.
- Costs a deployable service — no longer a static upload. Needs a place to keep one secret.

**Assessment:** recommended. The cost is one small service; the benefit is that the highest-value credential in the system never enters a browser.

### C. Serve the dashboard from the node itself

Ship the dashboard as same-origin static assets from the SEL node, reusing the existing admin session cookie directly.

- Strongest security posture available: same-origin throughout, no CORS, no second credential store, no new service.
- SSE and `cookieAuth` work exactly as designed.
- **Couples the dashboard's release cycle to the server's.** A CSS fix ships as a server deploy. This is precisely why `dashboard` was split into its own repo.
- Constrains the frontend stack to whatever the server can serve, and puts dashboard bugs in the server's blast radius.

**Assessment:** viable, and genuinely the most secure. Worth reconsidering if operational simplicity later outweighs release independence — but it argues against the decision already made to separate the repos.

## Recommendation

**Option B.** Concretely:

1. **Split the data path by sensitivity.** Public panels (coverage, freshness, change feed) fetch the node directly with no credentials. Only admin panels traverse the BFF. This keeps most of the app working even if admin auth is misconfigured, and makes the privileged surface small enough to audit.
2. **BFF holds one credential**, from the environment, never in the repo. It obtains and refreshes the admin JWT itself.
3. **Browser gets a session cookie only** — `HttpOnly`, `Secure`, `SameSite=Strict`. No token is readable from JS.
4. **Allowlist explicitly at the BFF.** Do not depend on the node's current permissive CORS.
5. **Proxy narrowly.** Enumerate the upstream operations the BFF forwards. Default-deny; add endpoints as panels need them. Destructive operations (`DELETE /admin/events/{id}`, `POST /admin/scraper/trigger-all`) should require a deliberate decision to expose, not arrive by default.
6. **Read-only first.** The MVP should not proxy any mutating endpoint. Review-queue actions are a later, separate discussion.

## Open questions

1. **Who are the users?** A single operator and a public read-only view are very different products. If any part of the dashboard is to be publicly visible, the public/admin split in (1) becomes a hard architectural boundary rather than a convenience.
2. **One node or many?** Federation is core to SEL's premise. A multi-node dashboard needs credentials per node, which strengthens the case for B — a BFF can hold several, a browser holding several is worse than holding one.
3. **Is there an existing admin UI on the node?** If `/admin/login` already backs a first-party interface, some of this may be solved, and C becomes more attractive.
4. **What is the deploy target?** B's cost is entirely "somewhere to run a small service." If that is already solved for the org, B is nearly free; if not, that is the real tradeoff against C.
5. **Should the dashboard hold an `apiKey` too?** `POST /api/v1/auth/token` takes one. Relevant only if the dashboard ever writes.

## Related

- [server#25](https://github.com/Togather-Foundation/server/issues/25) — CORS reflects arbitrary origins with credentials; the constraint driving much of this
- [server#22](https://github.com/Togather-Foundation/server/issues/22) — provenance fields absent from the public API (scoped: admin endpoints cover most of it)
- [server#24](https://github.com/Togather-Foundation/server/issues/24) — no aggregate counts on public collections
- [web-viewer](https://github.com/Togather-Foundation/web-viewer) — the unauthenticated counterpart; useful contrast for what needs no backend
