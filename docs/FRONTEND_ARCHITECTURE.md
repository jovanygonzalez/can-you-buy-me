# Frontend Architecture — Astro shell + Flutter bid island

> **Decision:** Astro owns everything content/form/SEO-heavy (landing, catalog, register, login, Stripe setup). Flutter Web owns **only** the live bid screen, mounted same-origin under `/live`. Both talk to the same Go gRPC-Web backend; the `.proto` is the shared contract and a JWT in `localStorage` is the shared session.
>
> See `CLAUDE.md` → "Prescribed Tech Stack" and "Red Flags" for the governing rules. This doc is the concrete implementation plan.

---

## 1. Monorepo structure

```
can-you-buy-me/
├── api/                         # Go backend (already exists)
│   ├── cmd/server/main.go
│   ├── internal/ …
│   └── pkg/gen/<svc>/v1/         # Go stubs (generated from proto/v1)
├── app/                         # Flutter Web — LIVE BID SCREEN ONLY
│   ├── lib/
│   │   ├── main.dart            # reads JWT from localStorage, mounts BidScreen
│   │   ├── screens/bid/         # the one screen
│   │   ├── services/            # gRPC-Web bid client + WebSocket→NATS sub
│   │   ├── models/              # Dart stubs (generated) + view models
│   │   ├── theme/               # built from shared design tokens
│   │   └── widgets/
│   └── web/                     # Flutter web bootstrap (index.html, flutter.js)
├── web/                         # Astro — the shell (NEW)
│   ├── src/
│   │   ├── pages/
│   │   │   ├── index.astro              # landing
│   │   │   ├── login.astro / register.astro
│   │   │   ├── drops/[id].astro         # catalog/drop detail (SEO)
│   │   │   ├── account.astro            # profile + Stripe payment method
│   │   │   └── live/[auctionId].astro   # thin host page that boots Flutter
│   │   ├── lib/
│   │   │   ├── gen/                      # TS stubs (generated from proto/v1)
│   │   │   ├── rpc.ts                    # connect-web transport + clients
│   │   │   ├── auth.ts                   # JWT read/write in localStorage
│   │   │   └── stripe.ts                 # Stripe.js Elements + SetupIntent
│   │   ├── components/                   # forms, catalog cards (islands)
│   │   └── styles/tokens.css             # SINGLE source of design tokens
│   ├── public/live/                      # Flutter build output lands here (dev)
│   ├── astro.config.mjs
│   └── package.json
├── proto/v1/                     # SHARED contract — Go + TS + Dart all generate from here
│   ├── auth.proto   (package auth.v1)
│   ├── auction.proto (package auction.v1)
│   ├── payment.proto (package payment.v1)
│   └── health.proto (package health.v1)
└── buf.gen.yaml                  # codegen config for TS + Dart (NEW)
```

**Why `web/public/live/`:** Astro serves `public/` verbatim at the site root. Dropping the Flutter build under `public/live/` makes Flutter reachable at `https://<origin>/live/…` on the **same origin** as Astro — which is the whole reason the `localStorage` JWT handoff works without CORS or cross-subdomain cookie hacks.

---

## 2. Runtime topology

### Production (AWS)

The **frontend** (Astro + Flutter) must be one origin. The **backend** can be a separate subdomain, because gRPC-Web sends the JWT as an `Authorization` **metadata header** (not a cookie) — so it only needs CORS, not same-origin.

| Host | Origin behind it | Serves |
|---|---|---|
| `canyoubuyme.com` (CloudFront → S3) | S3 static bucket | Astro at `/*`, Flutter build at `/live/*` |
| `api.canyoubuyme.com` (ALB → Fargate) | Go gRPC-Web server (`HTTP_PORT`, default `8070`) | gRPC-Web RPCs, `/health`, `/webhooks/stripe` |
| `nats.canyoubuyme.com` (NLB → NATS) | NATS WebSocket (`8443`) | `wss://` bid stream subscription |

**CloudFront distribution (frontend) — ordered cache behaviors:**

1. `/live/*` → S3 origin. Flutter assets. `index.html` no-cache; hashed `*.js/*.wasm` cached 1y (`immutable`). Set `Content-Type` for `.wasm` if using CanvasKit/skwasm.
2. `/_astro/*` (and other hashed asset dirs) → S3 origin, cached 1y immutable.
3. `default (*)` → S3 origin, Astro HTML. Short TTL or invalidate on deploy.
   - SPA-style fallback for `/live/*` deep links: 403/404 → `/live/index.html` (CloudFront custom error response) so refresh inside the bid screen works.

**Why not route the backend through the same CloudFront?** gRPC-Web request paths are `/<package>.<Service>/<Method>` (e.g. `/auth.v1.AuthService/Login`) — they can't be captured by a single clean path prefix without the backend rejecting a rewritten path. A dedicated `api.` subdomain + CORS is simpler and standard. The backend already exposes `grpcweb.WithOriginFunc(...)`, so allow-list the frontend origin there.

### Local dev

| Process | Command | URL |
|---|---|---|
| Backend (gRPC-Web + webhooks) | `cd api && make run` | `http://localhost:8070` |
| NATS WS | docker (`make docker-up`) | `ws://localhost:8443` |
| Astro shell | `cd web && npm run dev` | `http://localhost:4321` |
| Flutter (integration) | `cd app && flutter build web --base-href /live/ --output ../web/public/live` | served by Astro at `:4321/live/` |
| Flutter (isolated dev) | `cd app && flutter run -d chrome` | own port; use while iterating only |

**Same-origin in dev:** Astro on `:4321` serves the Flutter build out of `public/live/`, so both share `localStorage` on `localhost:4321`. You trade Flutter hot-reload for true same-origin; iterate Flutter standalone (`flutter run`) when working on the bid UI, then rebuild into `public/live` to test the integrated handoff.

**CORS in dev:** Astro `:4321` → backend `:8070` is cross-origin, so set the backend's `WithOriginFunc` to allow `http://localhost:4321` (gate behind `ENVIRONMENT=development`). NATS WS dev is `no_tls` per `nats-server.conf`.

---

## 3. Stub generation from `proto/v1` (TS + Dart)

Use **Buf** to drive codegen for the frontend stubs (Go stays on the existing `protoc` Makefile target). Client library: **Connect-ES** with the **gRPC-Web transport**, which is wire-compatible with the backend's `improbable-eng/grpc-web` handler.

### `buf.gen.yaml` (repo root)

```yaml
version: v2
inputs:
  - directory: proto/v1
plugins:
  # ---- TypeScript (Astro) ----
  - remote: buf.build/bufbuild/es
    out: web/src/lib/gen
    opt: target=ts
  - remote: buf.build/connectrpc/es
    out: web/src/lib/gen
    opt: target=ts
  # ---- Dart (Flutter) ----
  - remote: buf.build/protocolbuffers/dart
    out: app/lib/gen
```

Generate:

```bash
buf generate            # writes TS → web/src/lib/gen, Dart → app/lib/gen
```

> Pin `buf` and run it in CI so stubs never drift from `proto/v1`. Optionally add a `make proto-web` target in `api/Makefile` (or a root `Makefile`) that shells out to `buf generate`, keeping a single "regenerate everything" entry point.

### Astro: TS dependencies

```bash
cd web
npm i @connectrpc/connect @connectrpc/connect-web @bufbuild/protobuf
npm i @stripe/stripe-js
```

### Astro: transport + clients (`web/src/lib/rpc.ts`)

```ts
import { createGrpcWebTransport } from "@connectrpc/connect-web";
import { createClient } from "@connectrpc/connect";
import { AuthService } from "./gen/auth/v1/auth_connect";
import { PaymentService } from "./gen/payment/v1/payment_connect";
import { getToken } from "./auth";

const transport = createGrpcWebTransport({
  baseUrl: import.meta.env.PUBLIC_API_URL, // dev: http://localhost:8070  prod: https://api.canyoubuyme.com
  interceptors: [
    (next) => async (req) => {
      const tok = getToken();
      if (tok) req.header.set("Authorization", `Bearer ${tok}`);
      return next(req);
    },
  ],
});

export const authClient = createClient(AuthService, transport);
export const paymentClient = createClient(PaymentService, transport);
```

### Flutter: Dart client

```bash
cd app
flutter pub add grpc protobuf web   # grpc package speaks gRPC-Web on web targets
```

Dart stubs come from `buf generate` into `app/lib/gen`. On the web target the `grpc` package uses `GrpcWebClientChannel.xhr(Uri.parse(apiUrl))` and sends the same `Authorization: Bearer <jwt>` metadata.

---

## 4. JWT handoff (Astro → Flutter), same-origin

```
1. Astro /login  → authClient.login()  → backend returns LoginResponse.jwt_token
2. auth.ts: localStorage.setItem("cybm_jwt", token)
3. User clicks "Enter auction" → <a href="/live/{auctionId}">
4. Astro /live/[auctionId].astro renders a minimal page that boots Flutter (public/live build)
5. Flutter main.dart: reads window.localStorage["cybm_jwt"]  (package:web)
        ├─ missing/expired → window.location = "/login?next=/live/{id}"
        └─ present → opens gRPC-Web bid channel (Authorization header)
                   + NATS WS subscription to auction.{id}.bids
```

`web/src/lib/auth.ts`:

```ts
const KEY = "cybm_jwt";
export const getToken  = () => localStorage.getItem(KEY);
export const setToken  = (t: string) => localStorage.setItem(KEY, t);
export const clearToken = () => localStorage.removeItem(KEY);
```

Flutter read (`app/lib/main.dart`, using `package:web`):

```dart
import 'package:web/web.dart' as web;
final jwt = web.window.localStorage.getItem('cybm_jwt');
if (jwt == null) web.window.location.href = '/login?next=${web.window.location.pathname}';
```

**Security note:** the token is JS-readable by design — gRPC-Web and the NATS WS client both need it from JS. This is identical to a Flutter-only setup; it does **not** worsen the XSS posture. Keep JWT TTL short and refresh via re-login for the MVP.

---

## 5. Where each Phase-1/2 piece lives

| Feature (from CLAUDE.md phases) | Lives in | Notes |
|---|---|---|
| Register / Login (Phase 1) | **Astro** | gRPC-Web → `AuthService`; store JWT |
| Stripe SetupIntent (Phase 1) | **Astro** | `stripe.js` Elements; SetupIntent created via `PaymentService` gRPC-Web call |
| Catalog / drop pages (Phase 2) | **Astro** | Static/SSG for SEO; reads from Redis-cached catalog via backend |
| Live bid screen (Phase 2) | **Flutter** | gRPC-Web `PlaceBid` writes + NATS WS reads; the only Flutter surface |
| Admin close-auction (Phase 2) | backend only | manual endpoint, no UI |

---

## 6. Design-token discipline (one source)

`web/src/styles/tokens.css` defines CSS variables (`--cybm-color-bg`, `--cybm-font-display`, spacing scale…). Mirror the same values into a single Dart file `app/lib/theme/tokens.dart` (generated or hand-mirrored, but **derived from the CSS file, never invented separately**). A tiny script that reads `tokens.css` and emits `tokens.dart` keeps them in lockstep and is worth writing once. This is the only defense against the bid screen looking like a different product than the login.

---

## 7. Implementation order (suggested)

1. **Scaffold `web/`** — `npm create astro@latest`, add `tokens.css`, landing + login/register pages (static, no backend yet).
2. **Codegen** — add `buf.gen.yaml`, run `buf generate`, commit TS + Dart stubs; wire `rpc.ts`.
3. **Auth flow end-to-end** — Astro login → backend `AuthService` → JWT in `localStorage`. (Enable dev CORS on backend for `localhost:4321`.)
4. **Stripe SetupIntent** in Astro `/account`.
5. **Flutter bid screen** — strip the demo `main.dart`, read JWT, wire gRPC-Web `PlaceBid` + NATS WS subscription.
6. **Integrate** — `flutter build web --base-href /live/ --output ../web/public/live`; verify the handoff at `:4321/live/{id}`.
7. **Deploy** — S3 + CloudFront (frontend), ALB+Fargate (`api.`), NLB (`nats.`); set `PUBLIC_API_URL` + production `WithOriginFunc` allow-list.

---

## Open decisions to confirm

- **Astro render mode:** `output: 'static'` (S3-only, simplest) vs `'server'` with an adapter. For the MVP, static + client-side gRPC-Web islands covers login/Stripe/catalog — recommend **static** unless a drop page needs request-time SSR.
- **Domain shape:** single apex for frontend + `api.`/`nats.` subdomains (recommended), vs everything under one CloudFront. This doc assumes the former.
- **Token TTL / refresh:** MVP uses short-lived JWT + re-login. Refresh tokens are post-MVP.
