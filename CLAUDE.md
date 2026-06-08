# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Can You Buy Me** is a real-time auction platform (drop model) MVP focused on validating market traction in Mexico. It supports massive concurrent traffic bursts where thousands of users simultaneously bid on limited-supply items (collectibles, art, limited sneakers) at scheduled drop times.

**Key Philosophy:** MVP-first development. Prioritize the critical bidding flow and user validation. Manual admin operations and future scaling considerations come later.

## Prescribed Tech Stack (Non-Negotiable)

The following technologies are mandated and must be used:

- **Frontend (shell):** Astro for everything that is content-heavy, form-heavy, or SEO-relevant — landing, catalog/drop pages, register/login, and Stripe SetupIntent. Ships near-zero JS, indexable, fast first load.
- **Frontend (real-time island):** Flutter Web for the **live bid screen only**, mounted same-origin under a route (e.g. `/live`). This is the one surface where real-time WebSocket streams + complex stateful UI justify Flutter's payload.
- **Frontend contract:** Both clients are generated from the same `proto/v1/*.proto` (TS stubs for Astro via `protoc-gen-ts`, Dart stubs for Flutter). The `.proto` is the single source of truth; models are never hand-duplicated. Session is a JWT in `localStorage`, shared same-origin between Astro and Flutter.
- **Backend:** Go (microservices or modular monolith)
- **Client→Server (Writes):** gRPC-Web (bids sent directly from Flutter to Go backend)
- **Server→Client (Reads/Real-time):** WebSockets connected to NATS JetStream
- **Read Cache:** Redis (ElastiCache) for static catalog, prevents thundering herd
- **Primary Database:** PostgreSQL (RDS) for users, payment tracking, audit
- **Message Bus:** NATS JetStream (single node for MVP, scales to 3+ nodes later)
- **Infrastructure:** AWS (ECS/Fargate or EC2, S3 + CloudFront, RDS, ElastiCache)

**Why this stack:** Designed to handle microsecond-latency pub/sub to thousands of concurrent clients during drop events. NATS JetStream is the single source of truth for auction events and enables future Event Sourcing + CQRS evolution.

**Why the hybrid frontend (Astro + Flutter):** Flutter Web is poor for SEO (canvas-rendered) and ships a 2-5 MB initial payload — bad for a consumer marketing/landing funnel in Mexico (mobile data, mid-range devices). Astro inverts both: indexable, near-zero JS. Stripe Elements/`stripe.js` is also more natural in Astro than in Flutter Web. Flutter is reserved for the one surface where it wins decisively: the real-time bidding screen. The seam runs inside the authenticated app (Stripe setup in Astro, bidding in Flutter), which is acceptable **only** because both share one origin, one `.proto` contract, and one JWT. See `docs/FRONTEND_ARCHITECTURE.md` for the concrete integration plan. The "no REST intermediaries for real-time" rule still holds: the bid path is gRPC-Web + WebSocket→NATS, never REST.

## Development Phases (In Priority Order)

### Phase 1: Trust & Money (Critical Path)
- User registration and authentication (JWT-based)
- Stripe Setup Intents integration (validate payment method without pre-auth)
- PostgreSQL schema for users and payment metadata
- Minimal auth backend endpoints (gRPC)

### Phase 2: Auction Engine (Core Value)
- Static catalog loaded into Redis (manually managed for MVP)
- gRPC-Web bid handler in Go backend
- NATS JetStream pub/sub for live bid updates
- WebSocket connection from Flutter to NATS for real-time streams
- Manual auction open/close via admin endpoint (e.g., `/admin/close-auction`)

### Phase 3: Infrastructure & Deployment
- Docker containerization for Go and NATS
- AWS ECS/Fargate or EC2 deployment
- S3 + CloudFront for Flutter Web frontend
- RDS PostgreSQL and ElastiCache Redis setup

### Phase 4: Peripheral (Low Priority)
- Admin dashboard (not required for MVP)
- Automated charge logic (handle manually via Stripe dashboard)
- Contact/support forms (use Google Forms or simple email)

## MVP Constraints & Manual Operations

The MVP deliberately relies on manual/admin operations to reduce scope:

1. **Auction timing:** No auto-start/stop. Use manual endpoint to open and close auctions.
2. **Charging:** No automatic payment capture. Admin manually processes charges via Stripe dashboard the next day.
3. **Catalog management:** Items loaded via SQL scripts or direct Redis insertion, not an admin UI.
4. **No complex fund retention:** Don't implement pre-auth or hold logic during bids. Just verify Stripe has a valid card on file.
5. **Support:** Use external tools (Google Forms, mailto links) rather than custom ticketing.

**Rationale:** Every hour spent on admin tooling delays validation of the core bidding loop.

## Architecture Highlights

### Real-Time Flow
```
Flutter User → gRPC-Web → Go Backend → Validates & Publishes to NATS JetStream
                                              ↓
                                         Redis (Catalog Cache)
                                              ↓
                                         PostgreSQL (Audit/History)
                                              ↓
                                         NATS broadcast to all WebSocket connections
                                              ↓
                                         All Flutter clients see bid update (milliseconds)
```

### Data Flow Patterns
- **Writes (bids, registration):** gRPC-Web → Go handler → NATS publish → PostgreSQL audit
- **Reads (catalog, current bid):** Flutter → Go → Redis cache (fast) + PostgreSQL fallback
- **Real-time (live updates):** Flutter WebSocket → NATS JetStream → bidding events

## Future Scaling Vision (Post-MVP)

Once the MVP validates the market:

1. **Event Sourcing + CQRS:** Use NATS JetStream as the immutable log; separate read and write models.
2. **Multi-country expansion:** Geographic routing via NATS subject partitioning across regional superclusteres.
3. **Intelligent bidding:** Dynamic hold logic and tie-breaking automation.
4. **Cryptographic audit:** Leverage NATS JetStream as source of truth for dispute resolution and legal compliance.

**Constraint:** Do not over-architect for these goals during MVP. Build foundations (clean event flow, idempotent handlers, audit trail) but avoid premature Event Sourcing framework setup.

## Key Integration Points

### Stripe
- **Setup Intents:** Validate and store payment methods without charging.
- **Manual charging:** For MVP, use Stripe dashboard to process charges. Future: automate.
- **Documentation:** https://stripe.com/docs/payments/setup-intents

### NATS JetStream
- **Single node for MVP:** Sufficient for thousands of concurrent clients on one drop event.
- **Subject naming:** Design subjects hierarchically (e.g., `auctions.{auctionID}.bids`) for future multi-country expansion.
- **Persistence:** Enable JetStream durability; all bid events are audit-critical.

### gRPC-Web
- **Flutter integration:** Use `grpc` package for Dart; ensure TLS/mTLS for production.
- **Proto definition:** Define services early (Register, Login, PlaceBid, etc.); regenerate Dart stubs from proto.

## Development Guidelines

### Code Organization (Go Backend)
```
/cmd/server/main.go          # Entry point
/internal/auth/              # JWT, user validation
/internal/auction/           # Bidding logic, NATS publishing
/internal/stripe/            # Stripe Setup Intent and charge handlers
/internal/cache/             # Redis client and catalog ops
/internal/db/                # PostgreSQL migrations and queries
/proto/                       # .proto definitions for gRPC-Web
/migrations/                  # SQL migration files
```

### Code Organization (Monorepo)
```
/api/                         # Go backend (gRPC-Web + NATS + Postgres)
/app/                         # Flutter Web — LIVE BID SCREEN ONLY
/web/                         # Astro — landing, catalog, auth, Stripe setup (the shell)
/proto/v1/                    # Shared .proto contract (Go + TS + Dart generated from here)
```

### Code Organization (Astro shell — `/web/`)
```
/web/src/pages/              # File-based routes: index, /login, /register, /drops/[id], /account
/web/src/pages/live/         # Thin host page that boots the Flutter Web build
/web/src/lib/grpc/           # Generated TS stubs (protoc-gen-ts) + grpc-web client wrapper
/web/src/lib/auth.ts         # JWT read/write in localStorage (same-origin handoff to Flutter)
/web/src/lib/stripe.ts       # Stripe.js Elements + SetupIntent flow
/web/src/components/         # Astro/island components (forms, catalog cards)
/web/src/styles/tokens.css   # Design tokens — SINGLE source, mirrored into Flutter theme
```

### Code Organization (Flutter bid island — `/app/`)
```
/app/lib/main.dart           # Entry point — reads JWT from localStorage, mounts BidScreen
/app/lib/screens/bid/        # The live auction/bid screen (the only screen)
/app/lib/services/           # gRPC-Web bid client, WebSocket→NATS subscription
/app/lib/models/             # Dart stubs generated from proto/v1 + view models
/app/lib/theme/              # Theme built from the shared design tokens
/app/lib/widgets/            # Reusable bid UI components
```

### Database First
- **PostgreSQL migrations:** Version all schema changes. Start minimal (users, auctions, bids, payments).
- **Audit trail:** Every bid and charge attempt logged with timestamp and user ID.
- **No premature optimization:** Single table for bids is fine; normalization can follow once you understand query patterns.

### Testing
- **Unit tests on Go:** Critical for auction logic (bid validation, tie-breaking, state transitions).
- **Integration tests:** gRPC server + mock NATS, Redis, PostgreSQL. Simulate bid races.
- **Flutter:** Focus on UI responsiveness, not gRPC plumbing (test harness will mock services).

### Secrets & Deployment
- **Environment variables:** Stripe API keys, JWT secret, database credentials, NATS connection strings.
- **No secrets in code:** Use `.env` or AWS Secrets Manager in production.
- **Docker:** Build single images for Go and NATS; use docker-compose for local dev.

## Common Commands (To Be Implemented)

Once the project structure is in place:

```bash
# Backend (Go)
go mod tidy                    # Update dependencies
go run ./cmd/server/main.go    # Run backend locally
go test ./...                  # Run all tests
go test ./internal/auction -v  # Run auction tests verbosely
go build -o bin/server ./cmd/server/main.go  # Build binary

# Frontend (Flutter)
flutter pub get                # Install dependencies
flutter run -d chrome          # Run on Chrome for web
flutter test                   # Run tests

# Database
psql postgresql://user:pass@localhost/auction_db -f migrations/001_init.sql  # Apply migrations

# Docker & Local Dev
docker-compose up              # Start local NATS, Redis, Postgres
docker-compose down            # Tear down local services
```

## Red Flags & Constraints

1. **Don't build admin panels yet.** If you're tempted to add item management UI, resist. Manual SQL inserts are faster.
2. **Don't implement complex payment hold logic.** Stripe Setup Intents + manual charging is sufficient for MVP.
3. **Don't over-engineer NATS clustering.** Single node is fine. Architect for multi-node, but don't deploy it until you need it.
4. **Don't skip audit logging.** Every bid, registration, and payment attempt must be logged with timestamp and user ID (future compliance requirement).
5. **Test under load early.** Build a load test harness (locust, k6, or ghz) that simulates 1000+ concurrent bids. This is not "nice to have"—it's the core validation.
6. **Keep Flutter scoped to bidding.** If you're tempted to build login, catalog, or account screens in Flutter, stop — those belong in Astro. Every non-bid screen added to Flutter erodes the reason for the split (SEO, fast load) and doubles maintenance. Flutter owns `/live`, nothing else.
7. **One design-token source, one origin.** Colors/typography/spacing live in `/web/src/styles/tokens.css` and are mirrored into the Flutter theme — never define them twice by hand. Serve Astro and Flutter from the **same domain** so the JWT in `localStorage` is shared without CORS or cross-subdomain cookie hacks.

## References

- **README.md:** Business context, user flow, and full technical vision.
- **Stripe Docs:** https://stripe.com/docs/payments/setup-intents
- **gRPC-Web:** https://grpc.io/docs/platforms/web/
- **NATS JetStream:** https://docs.nats.io/nats-concepts/jetstream
- **Flutter:** https://flutter.dev/docs

## Future Updates to This File

As the codebase grows:
- Document actual repository structure and entry points.
- Add specific build/test/deployment commands.
- Record architectural decisions and rationale (ADRs).
- Update as the MVP validates and new phases begin.
