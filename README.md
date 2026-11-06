  # Myriad Technical College — College Management System

  React/Vite frontend and Go `net/http` API for student, teacher, and management workflows. The browser always uses the Go API; `DATA_MODE` selects the API's repository.

  ## Repository audit

  See [docs/repository-audit.md](docs/repository-audit.md) for the current repository and database compatibility audit, and [docs/authentication-security-audit.md](docs/authentication-security-audit.md) for password, token, and authorization findings.

  ## Requirements

  - Go 1.23+
  - Node.js 20+
  - PostgreSQL 14+ for persistent mode (Supabase PostgreSQL is compatible)

  ## Run in demo mode

  Terminal 1:

  ```sh
  cd backend
  cp .env.example .env
  go run ./cmd/server
  ```

  The server defaults to `localhost:8080`, `DATA_MODE=demo`, and `FRONTEND_URL=http://localhost:5173`. Demo records live in a synchronized in-memory store and reset when the backend restarts.

  Terminal 2, from the repository root:

  ```sh
  npm install
  cp .env.example .env
  npm run dev
  ```

  Demo accounts (password `demo1234`):

  | Role | Username |
  |---|---|
  | Student | `2024BCS1021` |
  | Teacher | `nisha@northstar.edu` |
  | Management | `admin@northstar.edu` |

  ## API and configuration

  The frontend uses `VITE_API_BASE_URL`, defaulting to `http://localhost:8080/api/v1`. Requests have a 12-second timeout and display server errors; the frontend contains no demo records or local data fallback.

  Backend configuration is loaded centrally from environment variables:

  ```env
  APP_ENV=development
  DATA_MODE=demo
  SERVER_ADDR=localhost:8080
  PORT=
  FRONTEND_URL=http://localhost:5173
  DATABASE_URL=
  JWT_SECRET=replace-with-a-long-random-secret-at-least-32-chars
  ```

  When `PORT` is set, the API listens on `0.0.0.0:$PORT`; otherwise it uses `SERVER_ADDR`. Production requires a `JWT_SECRET` of at least 32 characters. Do not commit real credentials.

  Implemented routes:

  - `GET /health`, `GET /ready`
  - `POST /api/v1/auth/login`, `GET /api/v1/auth/me`
  - `GET /api/v1/students/me/attendance`, `GET /api/v1/students/me/marks`
  - `GET /api/v1/students/me/fees`, `GET /api/v1/faculty/me/courses`
  - `GET /api/v1/rooms`, `GET /api/v1/notices`
  - `POST /api/v1/attendance`, `POST /api/v1/marks`, `POST /api/v1/notices`
  - `PUT /api/v1/fees/current`
  - `GET /api/v1/management/library/books`, `POST/PUT /api/v1/management/library/books`
  - `GET/POST /api/v1/management/library/issues`, `POST /api/v1/management/library/issues/{id}/return`
  - `GET /api/v1/students/me/library/issues`
  - `GET /api/v1/courses/{id}/students`, `POST /api/v1/attendance/sessions`, `GET/PUT /api/v1/attendance/sessions/{id}`
  - `GET /api/v1/rooms/available`, `GET /api/v1/rooms/allocations`, `GET /api/v1/rooms/allocations/current`
  - `POST /api/v1/rooms/allocations`, `POST /api/v1/rooms/allocations/{id}/release`
  - `GET/POST /api/v1/management/rooms`, `PUT /api/v1/management/rooms/{id}`, `GET /api/v1/management/rooms/{id}/allocations`

  The API returns JSON `{ "error": "..." }` errors. Login credentials are checked by the selected repository, and API routes enforce role authorization server-side.

  ## PostgreSQL and Supabase

  ### Create the database

  In Supabase, open **SQL Editor → New query**, paste the contents of [backend/supabase_setup.sql](backend/supabase_setup.sql), and run it. This consolidated script creates the current schema, the room overlap trigger and indexes, development seed data, and verification queries. It is intended for a fresh development/test project. For a production database, execute only the schema migrations and do not run the development seed:

  ```sh
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f backend/migrations/001_initial.sql
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f backend/migrations/002_attendance_rooms.sql
  ```

  `backend/seeds/demo.sql` is development-only. It creates one department, one management user, one teacher, six students, four offerings, 24 enrollments, two attendance sessions, 12 attendance records, 24 rooms, two allocation-history rows, and initial JSONB data for the currently snapshot-backed modules. It contains bcrypt hashes only. The supported development password is documented separately below; never use those accounts in production.

  To run the verification queries later, paste [backend/verification/verify_supabase.sql](backend/verification/verify_supabase.sql) into SQL Editor or run:

  ```sh
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f backend/verification/verify_supabase.sql
  ```

  The output lists all expected objects, indexes and trigger state; row counts; bcrypt cost and salt diversity; active allocation overlaps; and active enrollment counts. The overlap query should return zero rows.

  ### Configure the Go backend

  Set these values in the **backend host's secret environment configuration**, never in the Vite frontend environment:

  ```env
  APP_ENV=production
  DATA_MODE=postgres
  DATABASE_URL=postgresql://postgres:<password>@<host>:5432/postgres?sslmode=verify-full&sslrootcert=<path-to-supabase-root-cert>
  FRONTEND_URL=https://your-frontend.example
  JWT_SECRET=<random-secret-at-least-32-characters>
  ```

  Use the exact connection URL copied from Supabase. Long-running API hosts should use the direct connection when their network supports it, or the session pooler if IPv4 connectivity requires it. Supabase's shared transaction pooler uses port `6543` and does not support prepared statements; this backend detects that port and switches pgx to simple protocol. Supabase documents its [connection modes and pooler limitations](https://supabase.com/docs/guides/database/connecting-to-postgres) and [SSL verification options](https://supabase.com/docs/guides/platform/ssl-enforcement). Prefer `sslmode=verify-full` with the root certificate; `sslmode=require` encrypts the connection but does not verify the server identity.

  Start the Go service from `backend` with `go run ./cmd/server`. Start the Vite app with `VITE_API_BASE_URL=https://your-api-host/api/v1`; the browser talks only to the Go API. PostgreSQL startup fails rather than silently switching to Demo mode. Check `/health` for process liveness and `/ready` for database readiness.

  Demo credentials (password `demo1234`; development only):

  | Role | Username |
  |---|---|
  | Student | `2024BCS1021` |
  | Teacher | `nisha@northstar.edu` |
  | Management | `admin@northstar.edu` |

  PostgreSQL login verifies bcrypt in Go using `golang.org/x/crypto/bcrypt`; each seed account has an independent salt and a cost-10 hash. No database trigger handles passwords. The PostgreSQL schema does not provide signup, password update, or password reset routes.

  ### Troubleshooting

  - `DATA_MODE=postgres requires a valid PostgreSQL DATABASE_URL`: use a `postgres://` or `postgresql://` URL including the exact host, database, and credentials from Supabase.
  - TLS or certificate errors: use the root certificate from Supabase Database settings and `sslmode=verify-full`; percent-encode reserved characters in the database password.
  - Connection timeout/refused: confirm the host's network can reach the chosen endpoint and port. Use the session pooler for IPv4-only networks if direct connections are IPv6-only.
  - `prepared statement ... does not exist` on pooler: use the Supabase transaction pooler URL on port `6543`; the backend automatically selects pgx simple protocol for this mode. For other pooler modes, copy the matching Supabase connection URL.
  - `relation does not exist` or missing column errors: apply migrations in numeric order to the same database before starting the backend.
  - Empty course/room data or failed login: apply the development seed after migrations. Do not use development seed accounts for production.
  - `/ready` reports unavailable: inspect API logs for the database connection error without sharing the connection URL or password.

  ## Checks

  ```sh
  cd backend && GOCACHE=/tmp/go-build go test ./...
  cd .. && npm run build
  ```

  ## Scope and current limitations

  This project is compatible with the database behavior currently implemented in Go, but it is not a complete production campus-management database application. Attendance sessions, offerings, enrollment, authentication, rooms, and room allocations use normalized PostgreSQL tables. Summary attendance, marks, fees, library, and notices still use a shared JSONB snapshot; the existing normalized marks/results/payment/library tables are not wired to those APIs. Student ownership, payments, audit trails, account provisioning, and password reset workflows remain incomplete. Review the [database compatibility audit](docs/repository-audit.md) and [authentication security audit](docs/authentication-security-audit.md) before using real student data.
