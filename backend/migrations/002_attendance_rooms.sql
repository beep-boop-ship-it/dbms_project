CREATE TABLE IF NOT EXISTS course_offerings (
  id uuid PRIMARY KEY,
  course_id uuid NOT NULL REFERENCES courses(id),
  teacher_id uuid NOT NULL REFERENCES teachers(id),
  class_section_id uuid NOT NULL REFERENCES classes(id),
  capacity integer NOT NULL CHECK (capacity > 0),
  schedule text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','inactive')),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS course_offerings_teacher_idx ON course_offerings(teacher_id,status);

ALTER TABLE student_enrollments ADD COLUMN IF NOT EXISTS course_offering_id uuid REFERENCES course_offerings(id);
ALTER TABLE student_enrollments ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active';
ALTER TABLE student_enrollments DROP CONSTRAINT IF EXISTS student_enrollments_status_check;
ALTER TABLE student_enrollments ADD CONSTRAINT student_enrollments_status_check CHECK(status IN ('active','withdrawn','completed'));
CREATE UNIQUE INDEX IF NOT EXISTS student_enrollments_offering_student_uq ON student_enrollments(course_offering_id,student_id) WHERE course_offering_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS student_enrollments_offering_active_idx ON student_enrollments(course_offering_id,status,student_id);

ALTER TABLE attendance_sessions ADD COLUMN IF NOT EXISTS course_offering_id uuid REFERENCES course_offerings(id);
CREATE UNIQUE INDEX IF NOT EXISTS attendance_sessions_offering_time_uq ON attendance_sessions(course_offering_id,session_date,starts_at) WHERE course_offering_id IS NOT NULL;
ALTER TABLE attendance_records DROP CONSTRAINT IF EXISTS attendance_records_status_check;
ALTER TABLE attendance_records ADD CONSTRAINT attendance_records_status_check CHECK(status IN ('present','absent','late','excused'));

CREATE TABLE IF NOT EXISTS rooms (
  id uuid PRIMARY KEY,
  room_number varchar(40) NOT NULL UNIQUE,
  building varchar(120) NOT NULL,
  floor integer NOT NULL,
  room_type varchar(32) NOT NULL CHECK(room_type IN ('CLASSROOM','LABORATORY','SEMINAR_HALL','LECTURE_HALL')),
  capacity integer NOT NULL CHECK(capacity > 0),
  status varchar(20) NOT NULL DEFAULT 'AVAILABLE' CHECK(status IN ('AVAILABLE','OCCUPIED','MAINTENANCE')),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS room_allocations (
  id uuid PRIMARY KEY,
  room_id uuid NOT NULL REFERENCES rooms(id),
  course_offering_id uuid NOT NULL REFERENCES course_offerings(id),
  teacher_id uuid NOT NULL REFERENCES teachers(id),
  class_section_id uuid NOT NULL REFERENCES classes(id),
  allocation_date date NOT NULL,
  start_time time NOT NULL,
  end_time time NOT NULL,
  status varchar(16) NOT NULL DEFAULT 'ACTIVE' CHECK(status IN ('ACTIVE','RELEASED','EXPIRED')),
  claimed_at timestamptz NOT NULL DEFAULT now(),
  released_at timestamptz,
  CHECK(end_time > start_time),
  CHECK((status='RELEASED') = (released_at IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS room_allocations_room_interval_idx ON room_allocations(room_id,allocation_date,start_time,end_time) WHERE status='ACTIVE';
CREATE INDEX IF NOT EXISTS room_allocations_teacher_active_idx ON room_allocations(teacher_id,allocation_date,start_time) WHERE status='ACTIVE';
CREATE INDEX IF NOT EXISTS room_allocations_offering_active_idx ON room_allocations(course_offering_id,allocation_date,start_time) WHERE status='ACTIVE';

-- Serialize claims for a room by locking its parent row before testing overlap.
-- This avoids relying on a frontend check or requiring the btree_gist extension.
CREATE OR REPLACE FUNCTION prevent_room_allocation_overlap() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.status = 'ACTIVE' THEN
    PERFORM 1 FROM rooms WHERE id = NEW.room_id FOR UPDATE;
    IF EXISTS (
      SELECT 1 FROM room_allocations existing
      WHERE existing.room_id = NEW.room_id
        AND existing.allocation_date = NEW.allocation_date
        AND existing.status = 'ACTIVE'
        AND existing.id IS DISTINCT FROM NEW.id
        AND NEW.start_time < existing.end_time
        AND existing.start_time < NEW.end_time
    ) THEN
      RAISE EXCEPTION 'room is already allocated during this interval' USING ERRCODE = '23P01';
    END IF;
    IF EXISTS (
      SELECT 1 FROM room_allocations existing
      WHERE existing.teacher_id = NEW.teacher_id
        AND existing.allocation_date = NEW.allocation_date
        AND existing.status = 'ACTIVE'
        AND existing.id IS DISTINCT FROM NEW.id
        AND NEW.start_time < existing.end_time
        AND existing.start_time < NEW.end_time
    ) THEN
      RAISE EXCEPTION 'teacher already has an allocation during this interval' USING ERRCODE = '23P01';
    END IF;
    IF EXISTS (
      SELECT 1 FROM room_allocations existing
      WHERE existing.course_offering_id = NEW.course_offering_id
        AND existing.allocation_date = NEW.allocation_date
        AND existing.status = 'ACTIVE'
        AND existing.id IS DISTINCT FROM NEW.id
        AND NEW.start_time < existing.end_time
        AND existing.start_time < NEW.end_time
    ) THEN
      RAISE EXCEPTION 'course offering already has an allocation during this interval' USING ERRCODE = '23P01';
    END IF;
    IF EXISTS (SELECT 1 FROM rooms WHERE id=NEW.room_id AND status='MAINTENANCE') THEN
      RAISE EXCEPTION 'room is under maintenance' USING ERRCODE = '23514';
    END IF;
  END IF;
  RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS room_allocation_no_overlap ON room_allocations;
CREATE TRIGGER room_allocation_no_overlap BEFORE INSERT OR UPDATE OF room_id,allocation_date,start_time,end_time,status ON room_allocations
FOR EACH ROW EXECUTE FUNCTION prevent_room_allocation_overlap();
