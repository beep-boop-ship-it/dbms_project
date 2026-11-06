package store

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"college-management/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Postgres persists the same API state as demo mode. The business mutations are
// serialized through a row lock so concurrent administrative writes are atomic.
type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(ctx context.Context, url string) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 8
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 45 * time.Minute
	// Supabase's shared transaction pooler (port 6543) does not support
	// session-scoped prepared statements. Simple protocol keeps pgx compatible.
	if cfg.ConnConfig.Port == 6543 {
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	}
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err = p.Ping(ctx); err != nil {
		p.Close()
		return nil, err
	}
	return &Postgres{p}, nil
}
func (p *Postgres) Close() { p.pool.Close() }
func (p *Postgres) Authenticate(ctx context.Context, v domain.Credentials) (domain.User, error) {
	var u domain.User
	var hash string
	var err error
	switch v.Role {
	case "student":
		err = p.pool.QueryRow(ctx, "SELECT id::text,full_name,email,student_number,department_id::text,password_hash FROM students WHERE lower(email)=lower($1) OR lower(student_number)=lower($1)", v.Username).Scan(&u.ID, &u.Name, &u.Email, &u.StudentID, &u.Department, &hash)
		u.Role = "student"
	case "teacher":
		err = p.pool.QueryRow(ctx, "SELECT id::text,full_name,email,department_id::text,password_hash FROM teachers WHERE lower(email)=lower($1)", v.Username).Scan(&u.ID, &u.Name, &u.Email, &u.Department, &hash)
		u.Role = "teacher"
	case "management":
		err = p.pool.QueryRow(ctx, "SELECT id::text,full_name,email,password_hash FROM management_users WHERE lower(email)=lower($1)", v.Username).Scan(&u.ID, &u.Name, &u.Email, &hash)
		u.Role = "management"
	default:
		return domain.User{}, ErrForbidden
	}
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(v.Password)) != nil {
		return domain.User{}, ErrForbidden
	}
	return u, nil
}
func (p *Postgres) GetUser(ctx context.Context, id string) (domain.User, error) {
	var u domain.User
	query := `SELECT id::text,full_name,email,role,student_id,department FROM (
		SELECT id,full_name,email,'student'::text AS role,student_number AS student_id,department_id::text AS department FROM students
		UNION ALL SELECT id,full_name,email,'teacher'::text,''::text,department_id::text FROM teachers
		UNION ALL SELECT id,full_name,email,'management'::text,''::text,''::text FROM management_users
	) AS accounts WHERE id::text=$1`
	err := p.pool.QueryRow(ctx, query, id).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.StudentID, &u.Department)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	return u, err
}
func (p *Postgres) GetStudentByNumber(ctx context.Context, n string) (domain.User, error) {
	var u domain.User
	err := p.pool.QueryRow(ctx, "SELECT id::text,full_name,email,student_number,department_id::text FROM students WHERE lower(student_number)=lower($1)", n).Scan(&u.ID, &u.Name, &u.Email, &u.StudentID, &u.Department)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	u.Role = "student"
	return u, err
}
func (p *Postgres) ListCourseOfferings(ctx context.Context, teacher string, management bool) ([]domain.CourseOffering, error) {
	rows, err := p.pool.Query(ctx, `SELECT co.id::text,c.code,c.title,co.teacher_id::text,co.class_section_id::text,cl.class_name||' · Section '||cl.section,co.schedule,co.capacity,count(se.student_id)::int FROM course_offerings co JOIN courses c ON c.id=co.course_id JOIN classes cl ON cl.id=co.class_section_id LEFT JOIN student_enrollments se ON se.course_offering_id=co.id AND se.status='active' WHERE co.status='active' AND ($1::boolean OR co.teacher_id::text=$2) GROUP BY co.id,c.code,c.title,co.teacher_id,co.class_section_id,cl.class_name,cl.section,co.schedule,co.capacity ORDER BY c.code`, management, teacher)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.CourseOffering{}
	for rows.Next() {
		var v domain.CourseOffering
		if err = rows.Scan(&v.ID, &v.Code, &v.Subject, &v.TeacherID, &v.ClassSectionID, &v.Section, &v.Schedule, &v.Capacity, &v.Enrolled); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p *Postgres) ListRooms(ctx context.Context) ([]domain.Room, error) {
	rows, err := p.pool.Query(ctx, `SELECT id::text,room_number,building,floor::text,room_type,capacity,status FROM rooms ORDER BY building,floor,room_number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Room{}
	for rows.Next() {
		var r domain.Room
		var status string
		if err = rows.Scan(&r.ID, &r.Room, &r.Block, &r.Floor, &r.RoomType, &r.Capacity, &status); err != nil {
			return nil, err
		}
		r.Use = r.RoomType
		r.Status = titleStatus(status)
		out = append(out, r)
	}
	return out, rows.Err()
}
func titleStatus(s string) string {
	switch strings.ToUpper(s) {
	case "AVAILABLE":
		return "Available"
	case "OCCUPIED":
		return "Occupied"
	case "MAINTENANCE":
		return "Maintenance"
	default:
		return s
	}
}
func (p *Postgres) GetCourseRoster(ctx context.Context, teacher, offering string) ([]domain.Student, error) {
	var assigned string
	err := p.pool.QueryRow(ctx, "SELECT id::text FROM course_offerings WHERE id::text=$1 AND teacher_id::text=$2 AND status='active'", offering, teacher).Scan(&assigned)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrForbidden
	}
	if err != nil {
		return nil, err
	}
	rows, err := p.pool.Query(ctx, `SELECT s.id::text,s.student_number,s.full_name,true FROM student_enrollments e JOIN students s ON s.id=e.student_id WHERE e.course_offering_id::text=$1 AND e.status='active' ORDER BY s.student_number`, offering)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Student{}
	for rows.Next() {
		var v domain.Student
		if err = rows.Scan(&v.ID, &v.EnrollmentNumber, &v.Name, &v.Active); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p *Postgres) AttendanceSessions(ctx context.Context, teacher, offering string) ([]domain.AttendanceSession, error) {
	if _, err := p.GetCourseRoster(ctx, teacher, offering); err != nil {
		return nil, err
	}
	rows, err := p.pool.Query(ctx, `SELECT id::text,to_char(session_date,'YYYY-MM-DD'),to_char(starts_at,'HH24:MI') FROM attendance_sessions WHERE teacher_id::text=$1 AND course_offering_id::text=$2 ORDER BY session_date DESC,starts_at DESC`, teacher, offering)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.AttendanceSession{}
	for rows.Next() {
		var id, date, start string
		if err = rows.Scan(&id, &date, &start); err != nil {
			return nil, err
		}
		v, e := p.GetAttendanceSession(ctx, teacher, id)
		if e != nil {
			return nil, e
		}
		v.SessionDate = date
		v.StartTime = start
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p *Postgres) GetAttendanceSession(ctx context.Context, teacher, id string) (domain.AttendanceSession, error) {
	var v domain.AttendanceSession
	err := p.pool.QueryRow(ctx, `SELECT id::text,course_offering_id::text,teacher_id::text,to_char(session_date,'YYYY-MM-DD'),to_char(starts_at,'HH24:MI'),created_at::text FROM attendance_sessions WHERE id::text=$1`, id).Scan(&v.ID, &v.CourseOfferingID, &v.TeacherID, &v.SessionDate, &v.StartTime, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	if err != nil {
		return v, err
	}
	if v.TeacherID != teacher {
		return v, ErrForbidden
	}
	rows, err := p.pool.Query(ctx, `SELECT student_id::text,upper(status),coalesce(remarks,'') FROM attendance_records WHERE session_id::text=$1 ORDER BY student_id`, id)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	v.Attendance = []domain.AttendanceEntry{}
	for rows.Next() {
		var a domain.AttendanceEntry
		if err = rows.Scan(&a.StudentID, &a.Status, &a.Remarks); err != nil {
			return v, err
		}
		v.Attendance = append(v.Attendance, a)
	}
	return v, rows.Err()
}
func (p *Postgres) SaveAttendanceSession(ctx context.Context, teacher string, v domain.AttendanceSession, create bool) (domain.AttendanceSession, error) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return v, err
	}
	defer tx.Rollback(ctx)
	var courseID, classID string
	err = tx.QueryRow(ctx, `SELECT course_id::text,class_section_id::text FROM course_offerings WHERE id::text=$1 AND teacher_id::text=$2 AND status='active' FOR UPDATE`, v.CourseOfferingID, teacher).Scan(&courseID, &classID)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrForbidden
	}
	if err != nil {
		return v, err
	}
	rows, err := tx.Query(ctx, `SELECT s.id::text,s.student_number,s.full_name,true FROM student_enrollments e JOIN students s ON s.id=e.student_id WHERE e.course_offering_id::text=$1 AND e.status='active' ORDER BY s.student_number`, v.CourseOfferingID)
	if err != nil {
		return v, err
	}
	students := []domain.Student{}
	for rows.Next() {
		var s domain.Student
		if err = rows.Scan(&s.ID, &s.EnrollmentNumber, &s.Name, &s.Active); err != nil {
			rows.Close()
			return v, err
		}
		students = append(students, s)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return v, err
	}
	if err = validateAttendanceEntries(students, v); err != nil {
		return v, err
	}
	if create {
		v.ID = newUUID()
		err = tx.QueryRow(ctx, `INSERT INTO attendance_sessions(id,teacher_id,course_id,class_id,course_offering_id,session_date,starts_at) VALUES($1,$2,$3,$4,$5,$6::date,$7::time) RETURNING created_at::text`, v.ID, teacher, courseID, classID, v.CourseOfferingID, v.SessionDate, v.StartTime).Scan(&v.CreatedAt)
	} else {
		var created string
		err = tx.QueryRow(ctx, `UPDATE attendance_sessions SET session_date=$3::date,starts_at=$4::time WHERE id::text=$1 AND teacher_id::text=$2 AND course_offering_id::text=$5 RETURNING created_at::text`, v.ID, teacher, v.SessionDate, v.StartTime, v.CourseOfferingID).Scan(&created)
		if err == nil {
			v.CreatedAt = created
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return v, ErrNotFound
		}
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "23505" || pgErr.Code == "40001") {
			return v, ErrConflict
		}
		return v, err
	}
	if !create {
		if _, err = tx.Exec(ctx, "DELETE FROM attendance_records WHERE session_id::text=$1", v.ID); err != nil {
			return v, err
		}
	}
	for _, a := range v.Attendance {
		if _, err = tx.Exec(ctx, `INSERT INTO attendance_records(session_id,student_id,status,remarks) VALUES($1,$2,$3,$4)`, v.ID, a.StudentID, strings.ToLower(a.Status), a.Remarks); err != nil {
			return v, err
		}
	}
	v.TeacherID = teacher
	if err = tx.Commit(ctx); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "40001" {
			return v, ErrConflict
		}
		return v, err
	}
	return v, nil
}
func (p *Postgres) Snapshot(ctx context.Context) (domain.State, error) {
	var raw []byte
	err := p.pool.QueryRow(ctx, "SELECT payload FROM campus_state WHERE id=1").Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		s := domain.State{}
		b, _ := json.Marshal(s)
		if _, err = p.pool.Exec(ctx, "INSERT INTO campus_state(id,payload) VALUES(1,$1) ON CONFLICT(id) DO NOTHING", b); err != nil {
			return s, err
		}
		return p.Snapshot(ctx)
	}
	if err != nil {
		return domain.State{}, err
	}
	var s domain.State
	err = json.Unmarshal(raw, &s)
	if s.Attendance == nil {
		s.Attendance = []domain.Attendance{}
	}
	if s.Marks == nil {
		s.Marks = []domain.Mark{}
	}
	if s.Rooms == nil {
		s.Rooms = []domain.Room{}
	}
	if s.Notices == nil {
		s.Notices = []domain.Notice{}
	}
	if s.Courses == nil {
		s.Courses = []domain.Course{}
	}
	if s.Books == nil {
		s.Books = []domain.Book{}
	}
	if s.Issues == nil {
		s.Issues = []domain.Issue{}
	}
	if s.Offerings == nil {
		s.Offerings = []domain.CourseOffering{}
	}
	if s.Students == nil {
		s.Students = []domain.Student{}
	}
	if s.Enrollments == nil {
		s.Enrollments = []domain.Enrollment{}
	}
	if s.Sessions == nil {
		s.Sessions = []domain.AttendanceSession{}
	}
	if s.RoomAllocations == nil {
		s.RoomAllocations = []domain.RoomAllocation{}
	}
	return s, err
}
func (p *Postgres) mutate(ctx context.Context, fn func(*domain.State) error) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var raw []byte
	err = tx.QueryRow(ctx, "SELECT payload FROM campus_state WHERE id=1 FOR UPDATE").Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		s := domain.State{}
		raw, _ = json.Marshal(s)
		_, err = tx.Exec(ctx, "INSERT INTO campus_state(id,payload) VALUES(1,$1)", raw)
	}
	if err != nil {
		return err
	}
	var s domain.State
	if err = json.Unmarshal(raw, &s); err != nil {
		return err
	}
	if err = fn(&s); err != nil {
		return err
	}
	raw, err = json.Marshal(s)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE campus_state SET payload=$1,updated_at=now() WHERE id=1", raw); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Postgres) UpdateAttendance(ctx context.Context, code string, a, t int) error {
	return p.mutate(ctx, func(s *domain.State) error {
		for i := range s.Attendance {
			r := &s.Attendance[i]
			if r.Code == code {
				if t < 1 || a < 0 || a > t {
					return ErrInvalid
				}
				r.Attended = a
				r.Total = t
				return nil
			}
		}
		return ErrNotFound
	})
}
func (p *Postgres) UpdateMarks(ctx context.Context, code string, a, b int) error {
	return p.mutate(ctx, func(s *domain.State) error {
		for i := range s.Marks {
			r := &s.Marks[i]
			if r.Code == code {
				if a < 0 || a > 30 || b < 0 || b > 20 {
					return ErrInvalid
				}
				r.Internal = a
				r.Midterm = b
				r.Total = a + b
				return nil
			}
		}
		return ErrNotFound
	})
}
func (p *Postgres) AddNotice(ctx context.Context, n domain.Notice) error {
	return p.mutate(ctx, func(s *domain.State) error {
		if n.Title == "" || n.Type == "" {
			return ErrInvalid
		}
		s.Notices = append([]domain.Notice{n}, s.Notices...)
		return nil
	})
}
func (p *Postgres) UpdateFees(ctx context.Context, v domain.Fees) error {
	return p.mutate(ctx, func(s *domain.State) error {
		if v.Amount < 0 || v.Due == "" || v.Paid < s.Fees.Paid || v.Paid > v.Amount {
			return ErrInvalid
		}
		s.Fees = v
		return nil
	})
}

func (p *Postgres) SaveBook(ctx context.Context, b domain.Book, create bool) error {
	return p.mutate(ctx, func(s *domain.State) error {
		if b.Title == "" || b.Author == "" || b.Category == "" || b.TotalCopies < 0 {
			return ErrInvalid
		}
		if create {
			for _, old := range s.Books {
				if b.ISBN != "" && old.ISBN == b.ISBN {
					return ErrConflict
				}
			}
			b.ID = newUUID()
			b.AvailableCopies = b.TotalCopies
			s.Books = append(s.Books, b)
			return nil
		}
		for i := range s.Books {
			old := &s.Books[i]
			if old.ID == b.ID {
				issued := old.TotalCopies - old.AvailableCopies
				if b.TotalCopies < issued {
					return ErrInvalid
				}
				b.AvailableCopies = b.TotalCopies - issued
				*old = b
				return nil
			}
		}
		return ErrNotFound
	})
}
func (p *Postgres) IssueBook(ctx context.Context, v domain.Issue) error {
	return p.mutate(ctx, func(s *domain.State) error {
		due, dateErr := time.Parse("2006-01-02", v.DueAt)
		if v.StudentID == "" || dateErr != nil || due.Before(time.Now().Truncate(24*time.Hour)) {
			return ErrInvalid
		}
		idx := -1
		for i := range s.Books {
			if s.Books[i].ID == v.BookID {
				idx = i
				break
			}
		}
		if idx < 0 {
			return ErrNotFound
		}
		if s.Books[idx].AvailableCopies < 1 {
			return ErrConflict
		}
		for _, x := range s.Issues {
			if x.BookID == v.BookID && x.StudentID == v.StudentID && x.ReturnedAt == "" {
				return ErrConflict
			}
		}
		v.ID = newUUID()
		v.BookTitle = s.Books[idx].Title
		v.IssuedAt = time.Now().Format("2006-01-02")
		s.Books[idx].AvailableCopies--
		s.Issues = append(s.Issues, v)
		return nil
	})
}
func (p *Postgres) ReturnBook(ctx context.Context, id, returned string) error {
	return p.mutate(ctx, func(s *domain.State) error {
		if returned == "" {
			returned = time.Now().Format("2006-01-02")
		}
		returnDate, dateErr := time.Parse("2006-01-02", returned)
		if dateErr != nil {
			return ErrInvalid
		}
		for i := range s.Issues {
			x := &s.Issues[i]
			if x.ID == id {
				if x.ReturnedAt != "" {
					return ErrConflict
				}
				issuedDate, err := time.Parse("2006-01-02", x.IssuedAt)
				if err != nil || returnDate.Before(issuedDate) {
					return ErrInvalid
				}
				x.ReturnedAt = returned
				for j := range s.Books {
					if s.Books[j].ID == x.BookID {
						s.Books[j].AvailableCopies++
						return nil
					}
				}
				return ErrNotFound
			}
		}
		return ErrNotFound
	})
}

func (p *Postgres) AvailableRooms(ctx context.Context, teacher, offering, date, start, end string) ([]domain.Room, error) {
	if err := validateInterval(date, start, end); err != nil {
		return nil, err
	}
	capacity := 0
	if offering != "" {
		students, err := p.GetCourseRoster(ctx, teacher, offering)
		if err != nil {
			return nil, err
		}
		capacity = len(students)
	} else if teacher != "" {
		return nil, ErrForbidden
	}
	rows, err := p.pool.Query(ctx, `SELECT r.id::text,r.room_number,r.building,r.floor::text,r.room_type,r.capacity,r.status FROM rooms r WHERE r.status<>'MAINTENANCE' AND r.capacity >= $4 AND NOT EXISTS(SELECT 1 FROM room_allocations a WHERE a.room_id=r.id AND a.allocation_date=$1::date AND a.status='ACTIVE' AND $2::time<a.end_time AND a.start_time<$3::time) ORDER BY r.building,r.floor,r.room_number`, date, start, end, capacity)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRooms(rows)
}
func (p *Postgres) RoomAllocations(ctx context.Context, teacher string, management bool, roomID string) ([]domain.RoomAllocation, error) {
	rows, err := p.pool.Query(ctx, `SELECT a.id::text,a.room_id::text,r.room_number,a.course_offering_id::text,c.title,a.teacher_id::text,t.full_name,a.class_section_id::text,cl.class_name||' · Section '||cl.section,to_char(a.allocation_date,'YYYY-MM-DD'),to_char(a.start_time,'HH24:MI'),to_char(a.end_time,'HH24:MI'),a.status,a.claimed_at::text,coalesce(a.released_at::text,'') FROM room_allocations a JOIN rooms r ON r.id=a.room_id JOIN course_offerings o ON o.id=a.course_offering_id JOIN courses c ON c.id=o.course_id JOIN teachers t ON t.id=a.teacher_id JOIN classes cl ON cl.id=a.class_section_id WHERE ($1::boolean OR a.teacher_id::text=$2) AND ($3='' OR a.room_id::text=$3) ORDER BY a.allocation_date DESC,a.start_time DESC`, management, teacher, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.RoomAllocation{}
	for rows.Next() {
		var a domain.RoomAllocation
		if err = rows.Scan(&a.ID, &a.RoomID, &a.RoomNumber, &a.CourseOfferingID, &a.CourseName, &a.TeacherID, &a.TeacherName, &a.ClassSectionID, &a.ClassSection, &a.AllocationDate, &a.StartTime, &a.EndTime, &a.Status, &a.ClaimedAt, &a.ReleasedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (p *Postgres) ClaimRoom(ctx context.Context, teacher string, a domain.RoomAllocation) (domain.RoomAllocation, error) {
	var result domain.RoomAllocation
	if err := validateInterval(a.AllocationDate, a.StartTime, a.EndTime); err != nil {
		return result, err
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	var courseID, classID, courseName, section, teacherName string
	var enrolled int
	var courseCapacity int
	err = tx.QueryRow(ctx, `SELECT o.course_id::text,o.class_section_id::text,c.title,cl.class_name||' · Section '||cl.section,t.full_name,o.capacity,(SELECT count(*)::int FROM student_enrollments e WHERE e.course_offering_id=o.id AND e.status='active') FROM course_offerings o JOIN courses c ON c.id=o.course_id JOIN classes cl ON cl.id=o.class_section_id JOIN teachers t ON t.id=o.teacher_id WHERE o.id::text=$1 AND o.teacher_id::text=$2 AND o.status='active' FOR UPDATE OF o`, a.CourseOfferingID, teacher).Scan(&courseID, &classID, &courseName, &section, &teacherName, &courseCapacity, &enrolled)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrForbidden
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "40001" {
			return result, ErrConflict
		}
		return result, err
	}
	var roomNumber, building, roomType, status string
	var floor, roomCapacity int
	err = tx.QueryRow(ctx, `SELECT room_number,building,floor,room_type,capacity,status FROM rooms WHERE id::text=$1 FOR UPDATE`, a.RoomID).Scan(&roomNumber, &building, &floor, &roomType, &roomCapacity, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrNotFound
	}
	if err != nil {
		return result, err
	}
	if status == "MAINTENANCE" {
		return result, ErrConflict
	}
	if roomCapacity < enrolled || courseCapacity < enrolled {
		return result, ErrInvalid
	}
	var conflict bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM room_allocations x WHERE x.status='ACTIVE' AND x.allocation_date=$1::date AND $2::time<x.end_time AND x.start_time<$3::time AND (x.room_id=$4::uuid OR x.teacher_id=$5::uuid OR x.course_offering_id=$6::uuid))`, a.AllocationDate, a.StartTime, a.EndTime, a.RoomID, teacher, a.CourseOfferingID).Scan(&conflict)
	if err != nil {
		return result, err
	}
	if conflict {
		return result, ErrConflict
	}
	a.ID = newUUID()
	a.TeacherID = teacher
	a.TeacherName = teacherName
	a.RoomNumber = roomNumber
	a.CourseName = courseName
	a.ClassSection = section
	a.ClassSectionID = classID
	a.Status = "ACTIVE"
	err = tx.QueryRow(ctx, `INSERT INTO room_allocations(id,room_id,course_offering_id,teacher_id,class_section_id,allocation_date,start_time,end_time,status) VALUES($1,$2,$3,$4,$5,$6::date,$7::time,$8::time,'ACTIVE') RETURNING claimed_at::text`, a.ID, a.RoomID, a.CourseOfferingID, teacher, classID, a.AllocationDate, a.StartTime, a.EndTime).Scan(&a.ClaimedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "23P01" || pgErr.Code == "40001" || pgErr.Code == "23505") {
			return result, ErrConflict
		}
		return result, err
	}
	if _, err = tx.Exec(ctx, "UPDATE rooms SET status='OCCUPIED' WHERE id::text=$1", a.RoomID); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "40001" {
			return result, ErrConflict
		}
		return result, err
	}
	result = a
	return result, nil
}
func (p *Postgres) ReleaseRoom(ctx context.Context, teacher, id string, management bool) error {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var roomID, owner, status string
	err = tx.QueryRow(ctx, "SELECT room_id::text,teacher_id::text,status FROM room_allocations WHERE id::text=$1 FOR UPDATE", id).Scan(&roomID, &owner, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !management && owner != teacher {
		return ErrForbidden
	}
	if status != "ACTIVE" {
		return ErrConflict
	}
	if _, err = tx.Exec(ctx, "UPDATE room_allocations SET status='RELEASED',released_at=now() WHERE id::text=$1", id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE rooms SET status='AVAILABLE' WHERE id::text=$1 AND status<>'MAINTENANCE' AND NOT EXISTS(SELECT 1 FROM room_allocations WHERE room_id::text=$1 AND status='ACTIVE')`, roomID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Postgres) SaveRoom(ctx context.Context, r domain.Room, create bool) error {
	if r.Room == "" || r.Capacity < 1 {
		return ErrInvalid
	}
	if len(strings.Fields(r.Floor)) == 0 {
		return ErrInvalid
	}
	floor, err := strconv.Atoi(strings.Fields(r.Floor)[0])
	if err != nil || floor < 0 {
		return ErrInvalid
	}
	roomType := strings.ToUpper(r.RoomType)
	switch roomType {
	case "CLASSROOM", "LABORATORY", "SEMINAR_HALL", "LECTURE_HALL":
	default:
		return ErrInvalid
	}
	status := strings.ToUpper(r.Status)
	if status == "" {
		status = "AVAILABLE"
	}
	switch status {
	case "AVAILABLE", "OCCUPIED", "MAINTENANCE":
	default:
		return ErrInvalid
	}
	if create {
		if status == "OCCUPIED" {
			return ErrInvalid
		}
		_, err = p.pool.Exec(ctx, `INSERT INTO rooms(id,room_number,building,floor,room_type,capacity,status) VALUES($1,$2,$3,$4,$5,$6,$7)`, newUUID(), r.Room, r.Block, floor, roomType, r.Capacity, status)
	} else {
		tx, e := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if e != nil {
			return e
		}
		defer tx.Rollback(ctx)
		var exists, active bool
		if err = tx.QueryRow(ctx, "SELECT true FROM rooms WHERE id::text=$1 FOR UPDATE", r.ID).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM room_allocations WHERE room_id::text=$1 AND status='ACTIVE')", r.ID).Scan(&active); err != nil {
			return err
		}
		if status == "OCCUPIED" && !active {
			return ErrInvalid
		}
		if status == "MAINTENANCE" && active {
			return ErrConflict
		}
		if _, err = tx.Exec(ctx, `UPDATE rooms SET room_number=$2,building=$3,floor=$4,room_type=$5,capacity=$6,status=$7 WHERE id::text=$1`, r.ID, r.Room, r.Block, floor, roomType, r.Capacity, status); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrConflict
			}
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "40001" {
				return ErrConflict
			}
			return err
		}
		return nil
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return err
	}
	return nil
}
func scanRooms(rows pgx.Rows) ([]domain.Room, error) {
	out := []domain.Room{}
	for rows.Next() {
		var r domain.Room
		var status string
		if err := rows.Scan(&r.ID, &r.Room, &r.Block, &r.Floor, &r.RoomType, &r.Capacity, &status); err != nil {
			return nil, err
		}
		r.Use = r.RoomType
		r.Status = titleStatus(status)
		out = append(out, r)
	}
	return out, rows.Err()
}
