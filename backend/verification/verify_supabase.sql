-- Run after migrations and seed data. All result counts should match the notes.

-- Every table used by PostgreSQL repositories and every existing normalized
-- domain table should resolve in the public schema.
WITH required(table_name) AS (
  VALUES
    ('campus_state'), ('departments'), ('academic_years'), ('semesters'),
    ('management_users'), ('teachers'), ('students'), ('courses'), ('classes'),
    ('student_enrollments'), ('teacher_course_assignments'),
    ('course_offerings'), ('attendance_sessions'), ('attendance_records'),
    ('subject_marks'), ('semester_results'), ('fee_structures'),
    ('student_fee_records'), ('payments'), ('books'), ('book_copies'),
    ('library_issues'), ('rooms'), ('room_allocations')
)
SELECT table_name, to_regclass('public.' || quote_ident(table_name)) AS object
FROM required
ORDER BY table_name;

-- Query-specific indexes and overlap trigger.
SELECT schemaname, tablename, indexname
FROM pg_indexes
WHERE schemaname = 'public'
ORDER BY tablename, indexname;

SELECT tgname AS trigger_name, tgrelid::regclass AS table_name, tgenabled
FROM pg_trigger
WHERE NOT tgisinternal AND tgname = 'room_allocation_no_overlap';

-- Expected counts: 1 department, 1 management user, 1 teacher, 6 students,
-- 4 offerings, 24 active offering enrollments, 24 rooms, 2 sessions,
-- 12 attendance records, and 2 room allocation history rows.
SELECT 'departments' AS table_name, count(*) AS row_count FROM departments
UNION ALL SELECT 'management_users', count(*) FROM management_users
UNION ALL SELECT 'teachers', count(*) FROM teachers
UNION ALL SELECT 'students', count(*) FROM students
UNION ALL SELECT 'course_offerings', count(*) FROM course_offerings
UNION ALL SELECT 'active_offering_enrollments', count(*) FROM student_enrollments WHERE status='active'
UNION ALL SELECT 'attendance_sessions', count(*) FROM attendance_sessions
UNION ALL SELECT 'attendance_records', count(*) FROM attendance_records
UNION ALL SELECT 'rooms', count(*) FROM rooms
UNION ALL SELECT 'room_allocations', count(*) FROM room_allocations
UNION ALL SELECT 'campus_state', count(*) FROM campus_state
ORDER BY table_name;

-- Verify seeded bcrypt hashes have cost 10 and each account has its own salt.
SELECT 'management' AS role, count(*) AS accounts,
       count(DISTINCT substring(password_hash FROM 8 FOR 22)) AS unique_salts,
       bool_and(password_hash LIKE '$2a$10$%') AS valid_bcrypt_cost
FROM management_users
UNION ALL
SELECT 'teacher', count(*), count(DISTINCT substring(password_hash FROM 8 FOR 22)),
       bool_and(password_hash LIKE '$2a$10$%') FROM teachers
UNION ALL
SELECT 'student', count(*), count(DISTINCT substring(password_hash FROM 8 FOR 22)),
       bool_and(password_hash LIKE '$2a$10$%') FROM students;

-- Both queries must return zero rows when all active allocation intervals
-- satisfy the same room/teacher/course overlap rule as the trigger.
SELECT a1.id, a2.id, a1.room_id, a1.allocation_date
FROM room_allocations a1
JOIN room_allocations a2 ON a1.id < a2.id
WHERE a1.status='ACTIVE' AND a2.status='ACTIVE'
  AND a1.allocation_date=a2.allocation_date
  AND a1.start_time<a2.end_time AND a2.start_time<a1.end_time
  AND (a1.room_id=a2.room_id OR a1.teacher_id=a2.teacher_id OR a1.course_offering_id=a2.course_offering_id);

SELECT s.student_number, count(e.course_offering_id) AS active_offerings
FROM students s
LEFT JOIN student_enrollments e ON e.student_id=s.id AND e.status='active'
GROUP BY s.student_number
ORDER BY s.student_number;
