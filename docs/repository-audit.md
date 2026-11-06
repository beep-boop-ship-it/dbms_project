# Current repository audit

## Architecture and request flow

The project has a React/Vite client and a Go `net/http` API. The frontend calls `src/api.js`; `useCampusData` loads role-specific resources and invokes mutation methods; `src/App.jsx` selects the portal pages. The Go API routes and handlers live in `backend/internal/transport/http.go`. There is no separate service package: repository methods in `backend/internal/store` implement persistence and most domain validation directly.

`DATA_MODE=demo` uses the synchronized in-memory `Memory` repository. `DATA_MODE=postgres` creates a `pgxpool` repository and fails startup if it cannot connect. The frontend has no database client and no automatic Demo fallback.

## PostgreSQL persistence map

| Feature | PostgreSQL persistence currently used | Notes |
|---|---|---|
| Authentication / profile | `students`, `teachers`, `management_users`, `departments` | Password lookup and role-specific user lookup use normalized tables. |
| Courses / class offerings | `course_offerings`, `courses`, `classes`, `student_enrollments` | Offerings determine teacher authorization; roster query returns every active enrollment. `teacher_course_assignments` is seeded but not read by current code. |
| Session attendance | `attendance_sessions`, `attendance_records` | Full roster validation and writes run in a serializable transaction. |
| Rooms / allocations | `rooms`, `room_allocations` | Availability, claim, release, history, management editing use these tables. Claims and releases are transactional; an overlap trigger protects the room, teacher, and offering intervals. |
| Legacy summary attendance / marks | `campus_state.payload` JSONB | These are summary DTOs keyed by course code, not student-level records. |
| Fees | `campus_state.payload` JSONB | The current UI edits one aggregate amount/due/paid object; no payment ledger is used. |
| Library inventory and issues | `campus_state.payload` JSONB | The current API stores whole book and issue arrays in the snapshot, not `books`, `book_copies`, or `library_issues` tables. |
| Notices | `campus_state.payload` JSONB | Notice updates mutate the JSON snapshot. |
| Ready check | `campus_state` | `Snapshot` initializes the row if it is missing; `/ready` therefore also ensures a row exists. |

The existing migration also defines `subject_marks`, `semester_results`, fee/payment tables, and relational library tables. Current Go code does not read or write those tables. They are preserved in the setup because they already exist in the project schema, but they are not represented as completed repository paths.

## Database compatibility checks

- The Go SQL references only tables and columns present in the ordered migrations. PostgreSQL parameters compare UUID identifiers through `::text` or bind UUID values; dates and times are cast explicitly at insert/query boundaries.
- Password hash columns are `text`, which can hold the complete bcrypt encoding. Generated UUIDs are supplied by Go, so no database extension is required.
- The `attendance_records` primary key prevents duplicate student records per session; the status check allows `present`, `absent`, `late`, and `excused`. Session uniqueness is enforced by class/date/time and offering/date/time indexes.
- The room trigger serializes claims on the room row and rejects interval conflicts for the room, teacher, and offering. Room status, capacity, and allocation status have checks and indexes.
- The seed inserts six students, four course offerings with 24 active enrollments, two sessions with 12 attendance rows, 24 rooms, and one active plus one released allocation. The campus snapshot seeds the currently used aggregate UI modules.
- The SQL was applied to a fresh local PostgreSQL 18.6 database both as separate files and through `backend/supabase_setup.sql`. The seeded Go repository integration test passed against the database.

## Authentication and transport findings

See [authentication and password security audit](authentication-security-audit.md). The PostgreSQL and Demo login paths now both verify bcrypt hashes; authorization remains enforced by API role middleware and repository ownership checks for session attendance and teacher room actions. The wider application still has data-scoping and token-storage concerns that are documented separately.

## Supabase deployment boundary

The script creates objects in the normal `public` schema, uses no extensions, creates no Supabase Auth dependency, and adds no grants or RLS policies. The Go API is the trusted database client; credentials belong only in its server environment. Supabase's shared transaction pooler does not support prepared statements, so `NewPostgres` selects pgx simple protocol when the connection URL uses port `6543`; direct and session-pooler connections retain pgx's default query mode. For long-running API servers, prefer the direct connection or the session pooler where network IP support requires it. Follow Supabase's SSL guidance and use `sslmode=verify-full` with the project root certificate where practical.

## Remaining persistence and domain gaps

PostgreSQL mode is compatible with the current API contract, but it does not mean every displayed feature has a normalized multi-user data model. Summary attendance, marks, fees, library, and notices share one JSONB snapshot across users. There is no payment-recording API, signup, password update/reset flow, result history API, or production provisioning workflow. These are code-scope limitations; the SQL does not add fake endpoints or pretend those workflows are implemented.

The frontend calls `api.uploadCircular` when a notice is submitted with a file, but `src/api.js` has no such method and the Go API has no upload route or file-storage integration. Text-only notices are supported. No storage table or object bucket is added to the SQL because file upload is not implemented by the current backend.
