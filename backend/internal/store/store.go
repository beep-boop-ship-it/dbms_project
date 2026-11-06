package store

import (
	"context"
	crand "crypto/rand"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
	"time"

	"college-management/backend/internal/domain"
)

var ErrNotFound = errors.New("record not found")
var ErrForbidden = errors.New("not authorized")
var ErrConflict = errors.New("record conflicts with current state")
var ErrInvalid = errors.New("invalid request")

type Repository interface {
	Authenticate(context.Context, domain.Credentials) (domain.User, error)
	GetUser(context.Context, string) (domain.User, error)
	GetStudentByNumber(context.Context, string) (domain.User, error)
	ListCourseOfferings(context.Context, string, bool) ([]domain.CourseOffering, error)
	ListRooms(context.Context) ([]domain.Room, error)
	Snapshot(context.Context) (domain.State, error)
	UpdateAttendance(context.Context, string, int, int) error
	UpdateMarks(context.Context, string, int, int) error
	AddNotice(context.Context, domain.Notice) error
	UpdateFees(context.Context, domain.Fees) error
	SaveBook(context.Context, domain.Book, bool) error
	IssueBook(context.Context, domain.Issue) error
	ReturnBook(context.Context, string, string) error
	GetCourseRoster(context.Context, string, string) ([]domain.Student, error)
	AttendanceSessions(context.Context, string, string) ([]domain.AttendanceSession, error)
	GetAttendanceSession(context.Context, string, string) (domain.AttendanceSession, error)
	SaveAttendanceSession(context.Context, string, domain.AttendanceSession, bool) (domain.AttendanceSession, error)
	AvailableRooms(context.Context, string, string, string, string, string) ([]domain.Room, error)
	RoomAllocations(context.Context, string, bool, string) ([]domain.RoomAllocation, error)
	ClaimRoom(context.Context, string, domain.RoomAllocation) (domain.RoomAllocation, error)
	ReleaseRoom(context.Context, string, string, bool) error
	SaveRoom(context.Context, domain.Room, bool) error
	Close()
}

var users = map[string]struct {
	User domain.User
	Hash []byte
}{
	"2024bcs1021":         demoUser(domain.User{ID: "u-student", Name: "Aarav Sharma", Email: "aarav@northstar.edu", Role: "student", StudentID: "2024BCS1021", Program: "B.Tech · Computer Science", Semester: "Semester 5"}),
	"nisha@northstar.edu": demoUser(domain.User{ID: "u-teacher", Name: "Nisha Khanna", Email: "nisha@northstar.edu", Role: "teacher", Department: "Computer Science"}),
	"admin@northstar.edu": demoUser(domain.User{ID: "u-admin", Name: "Mira Kapoor", Email: "admin@northstar.edu", Role: "management", Department: "Administration"}),
}

func demoUser(u domain.User) struct {
	User domain.User
	Hash []byte
} {
	h, err := bcrypt.GenerateFromPassword([]byte("demo1234"), bcrypt.DefaultCost)
	if err != nil {
		panic("generate demo credential hash: " + err.Error())
	}
	return struct {
		User domain.User
		Hash []byte
	}{User: u, Hash: h}
}

func authenticate(_ context.Context, c domain.Credentials) (domain.User, error) {
	entry, ok := users[normalize(c.Username)]
	if !ok || c.Password == "" || bcrypt.CompareHashAndPassword(entry.Hash, []byte(c.Password)) != nil || (c.Role != "" && c.Role != entry.User.Role) {
		return domain.User{}, ErrForbidden
	}
	return entry.User, nil
}
func user(_ context.Context, id string) (domain.User, error) {
	for _, v := range users {
		if v.User.ID == id {
			return v.User, nil
		}
	}
	return domain.User{}, ErrNotFound
}
func studentByNumber(number string) (domain.User, error) {
	for _, v := range users {
		if v.User.StudentID == number {
			return v.User, nil
		}
	}
	return domain.User{}, ErrNotFound
}
func (*Memory) GetStudentByNumber(_ context.Context, n string) (domain.User, error) {
	return studentByNumber(n)
}
func (m *Memory) ListCourseOfferings(_ context.Context, teacher string, management bool) ([]domain.CourseOffering, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.CourseOffering{}
	for _, c := range m.state.Offerings {
		if !management && c.TeacherID != teacher {
			continue
		}
		count := 0
		for _, e := range m.state.Enrollments {
			if !e.Active || e.CourseOfferingID != c.ID {
				continue
			}
			for _, s := range m.state.Students {
				if s.ID == e.StudentID && s.Active {
					count++
					break
				}
			}
		}
		c.Enrolled = count
		out = append(out, c)
	}
	return out, nil
}
func (m *Memory) ListRooms(context.Context) ([]domain.Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]domain.Room{}, m.state.Rooms...), nil
}
func normalize(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func seed() domain.State {
	courses := []domain.Course{{"CS-301", "Design and Analysis of Algorithms", "CSE · Section A", "Mon, Wed · 9:00 AM", "A103", 62}, {"CS-303", "Database Management Systems", "CSE · Section A", "Mon, Thu · 10:30 AM", "B204", 58}, {"CS-305", "Operating Systems", "CSE · Section B", "Tue, Fri · 9:00 AM", "A203", 60}, {"CS-307", "Computer Networks", "CSE · Section A", "Wed, Fri · 12:00 PM", "B103", 56}}
	attendance := []domain.Attendance{{courses[0].Subject, courses[0].Code, 23, 26}, {courses[1].Subject, courses[1].Code, 21, 24}, {courses[2].Subject, courses[2].Code, 18, 23}, {courses[3].Subject, courses[3].Code, 20, 22}}
	marks := []domain.Mark{{courses[0].Subject, courses[0].Code, 24, 18, 42, 50}, {courses[1].Subject, courses[1].Code, 22, 17, 39, 50}, {courses[2].Subject, courses[2].Code, 26, 19, 45, 50}, {courses[3].Subject, courses[3].Code, 21, 16, 37, 50}}
	rooms := make([]domain.Room, 0, 24)
	for _, block := range []string{"A", "B"} {
		for floor := 1; floor <= 3; floor++ {
			for n := 1; n <= 4; n++ {
				i := (int(block[0]) + floor + n) % 8
				status := "Available"
				if i == 0 {
					status = "Maintenance"
				}
				number := block + string(rune('0'+floor)) + "0" + string(rune('0'+n))
				rooms = append(rooms, domain.Room{ID: uuidFor("room:" + number), Room: number, Block: block, Floor: string(rune('0'+floor)) + " Floor", Capacity: 30, Status: status, Use: "Classroom", RoomType: "CLASSROOM"})
			}
		}
	}
	students := []domain.Student{{ID: uuidFor("student:2024BCS1021"), EnrollmentNumber: "2024BCS1021", Name: "Aarav Sharma", Active: true}, {ID: uuidFor("student:2024BCS1022"), EnrollmentNumber: "2024BCS1022", Name: "Meera Patel", Active: true}, {ID: uuidFor("student:2024BCS1023"), EnrollmentNumber: "2024BCS1023", Name: "Kabir Rao", Active: true}, {ID: uuidFor("student:2024BCS1024"), EnrollmentNumber: "2024BCS1024", Name: "Isha Nair", Active: true}, {ID: uuidFor("student:2024BCS1025"), EnrollmentNumber: "2024BCS1025", Name: "Arjun Menon", Active: true}, {ID: uuidFor("student:2024BCS1026"), EnrollmentNumber: "2024BCS1026", Name: "Diya Shah", Active: true}}
	offerings := make([]domain.CourseOffering, 0, len(courses))
	enrollments := make([]domain.Enrollment, 0, len(courses)*len(students))
	for _, course := range courses {
		id := uuidFor("offering:" + course.Code)
		offerings = append(offerings, domain.CourseOffering{ID: id, Code: course.Code, Subject: course.Subject, TeacherID: "u-teacher", ClassSectionID: uuidFor("section:" + course.Code), Section: course.Section, Schedule: course.Schedule, Room: course.Room, Capacity: 80, Enrolled: len(students)})
		for _, student := range students {
			enrollments = append(enrollments, domain.Enrollment{StudentID: student.ID, CourseOfferingID: id, Active: true})
		}
	}
	today := time.Now().Format("2006-01-02")
	activeRoomID := uuidFor("room:B204")
	allocations := []domain.RoomAllocation{{ID: uuidFor("allocation:seed"), RoomID: activeRoomID, RoomNumber: "B204", CourseOfferingID: uuidFor("offering:CS-303"), CourseName: "Database Management Systems", TeacherID: "u-teacher", TeacherName: "Nisha Khanna", ClassSectionID: uuidFor("section:CS-303"), ClassSection: "CSE · Section A", AllocationDate: today, StartTime: "09:00", EndTime: "11:00", Status: "ACTIVE", ClaimedAt: time.Now().Format(time.RFC3339)}}
	for i := range rooms {
		if rooms[i].Room == "B204" {
			rooms[i].Status = "Occupied"
		}
	}
	return domain.State{Attendance: attendance, Marks: marks, Rooms: rooms, Notices: []domain.Notice{{"n1", "End semester examination schedule released", "Today · Academics", "Examination"}, {"n2", "Library hours extended for examination week", "Yesterday · Library", "Campus"}, {"n3", "DBMS project review submissions due Friday", "04 Nov · Department", "Academic"}, {"n4", "Updated fee payment instructions", "02 Nov · Accounts", "Fees"}}, Fees: domain.Fees{Amount: 42500, Due: "2026-11-15", Term: "Semester 5 · 2026–27", Paid: 0}, Courses: courses, Books: []domain.Book{{ID: "book-001", ISBN: "9780131103627", Title: "The Art of Computer Programming", Author: "Donald E. Knuth", Category: "Computing", TotalCopies: 3, AvailableCopies: 2}, {ID: "book-002", ISBN: "9780321751041", Title: "Introduction to Algorithms", Author: "Thomas H. Cormen et al.", Category: "Computing", TotalCopies: 4, AvailableCopies: 4}}, Issues: []domain.Issue{{ID: "issue-001", BookID: "book-001", BookTitle: "The Art of Computer Programming", StudentID: "2024BCS1021", StudentName: "Aarav Sharma", IssuedAt: "2026-10-28", DueAt: "2026-11-18"}}, Offerings: offerings, Students: students, Enrollments: enrollments, Sessions: []domain.AttendanceSession{}, RoomAllocations: allocations}
}
func uuidFor(v string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(v))
	return fmt.Sprintf("00000000-0000-4000-8000-%012x", h.Sum64()&0xffffffffffff)
}
func newUUID() string {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		return uuidFor(fmt.Sprintf("%d", time.Now().UnixNano()))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type Memory struct {
	mu    sync.RWMutex
	state domain.State
}

func NewMemory() *Memory { return &Memory{state: seed()} }
func (*Memory) Authenticate(c context.Context, v domain.Credentials) (domain.User, error) {
	return authenticate(c, v)
}
func (*Memory) GetUser(c context.Context, id string) (domain.User, error) { return user(c, id) }
func (m *Memory) Snapshot(context.Context) (domain.State, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state, nil
}
func (m *Memory) UpdateAttendance(_ context.Context, code string, attended, total int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.state.Attendance {
		r := &m.state.Attendance[i]
		if r.Code == code {
			if total < 1 || attended < 0 || attended > total {
				return ErrInvalid
			}
			r.Attended = attended
			r.Total = total
			return nil
		}
	}
	return ErrNotFound
}
func (m *Memory) UpdateMarks(_ context.Context, code string, internal, midterm int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.state.Marks {
		r := &m.state.Marks[i]
		if r.Code == code {
			if internal < 0 || internal > 30 || midterm < 0 || midterm > 20 {
				return ErrInvalid
			}
			r.Internal = internal
			r.Midterm = midterm
			r.Total = internal + midterm
			return nil
		}
	}
	return ErrNotFound
}
func (m *Memory) AddNotice(_ context.Context, n domain.Notice) error {
	if n.Title == "" || n.Type == "" {
		return ErrInvalid
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Notices = append([]domain.Notice{n}, m.state.Notices...)
	return nil
}
func (m *Memory) UpdateFees(_ context.Context, v domain.Fees) error {
	if v.Amount < 0 || v.Due == "" || v.Paid > v.Amount {
		return ErrInvalid
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if v.Paid < m.state.Fees.Paid {
		return ErrInvalid
	}
	m.state.Fees = v
	return nil
}

func (m *Memory) SaveBook(_ context.Context, b domain.Book, create bool) error {
	if b.Title == "" || b.Author == "" || b.Category == "" || b.TotalCopies < 0 {
		return ErrInvalid
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if create {
		for _, x := range m.state.Books {
			if b.ISBN != "" && x.ISBN == b.ISBN {
				return ErrConflict
			}
		}
		b.ID = newUUID()
		b.AvailableCopies = b.TotalCopies
		m.state.Books = append(m.state.Books, b)
		return nil
	}
	for i := range m.state.Books {
		old := &m.state.Books[i]
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
}
func (m *Memory) IssueBook(_ context.Context, v domain.Issue) error {
	due, dateErr := time.Parse("2006-01-02", v.DueAt)
	if v.StudentID == "" || dateErr != nil || due.Before(time.Now().Truncate(24*time.Hour)) {
		return ErrInvalid
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	bookIndex := -1
	for i := range m.state.Books {
		if m.state.Books[i].ID == v.BookID {
			bookIndex = i
			break
		}
	}
	if bookIndex < 0 {
		return ErrNotFound
	}
	if m.state.Books[bookIndex].AvailableCopies < 1 {
		return ErrConflict
	}
	for _, i := range m.state.Issues {
		if i.BookID == v.BookID && i.StudentID == v.StudentID && i.ReturnedAt == "" {
			return ErrConflict
		}
	}
	student, err := studentByNumber(v.StudentID)
	if err != nil {
		return err
	}
	v.StudentName = student.Name
	v.ID = newUUID()
	v.BookTitle = m.state.Books[bookIndex].Title
	v.IssuedAt = time.Now().Format("2006-01-02")
	m.state.Books[bookIndex].AvailableCopies--
	m.state.Issues = append(m.state.Issues, v)
	return nil
}
func (m *Memory) ReturnBook(_ context.Context, id, returnedAt string) error {
	if returnedAt == "" {
		returnedAt = time.Now().Format("2006-01-02")
	}
	returnDate, dateErr := time.Parse("2006-01-02", returnedAt)
	if dateErr != nil {
		return ErrInvalid
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.state.Issues {
		issue := &m.state.Issues[i]
		if issue.ID == id {
			if issue.ReturnedAt != "" {
				return ErrConflict
			}
			issuedDate, err := time.Parse("2006-01-02", issue.IssuedAt)
			if err != nil || returnDate.Before(issuedDate) {
				return ErrInvalid
			}
			issue.ReturnedAt = returnedAt
			for j := range m.state.Books {
				if m.state.Books[j].ID == issue.BookID {
					m.state.Books[j].AvailableCopies++
					return nil
				}
			}
			return ErrNotFound
		}
	}
	return ErrNotFound
}

func roster(s domain.State, teacherID, offeringID string) ([]domain.Student, error) {
	assigned := false
	for _, o := range s.Offerings {
		if o.ID == offeringID && o.TeacherID == teacherID {
			assigned = true
			break
		}
	}
	if !assigned {
		return nil, ErrForbidden
	}
	active := map[string]bool{}
	for _, e := range s.Enrollments {
		if e.CourseOfferingID == offeringID && e.Active {
			active[e.StudentID] = true
		}
	}
	out := make([]domain.Student, 0, len(active))
	for _, student := range s.Students {
		if student.Active && active[student.ID] {
			out = append(out, student)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EnrollmentNumber < out[j].EnrollmentNumber })
	return out, nil
}
func (m *Memory) GetCourseRoster(_ context.Context, teacher, offering string) ([]domain.Student, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return roster(m.state, teacher, offering)
}
func (m *Memory) AttendanceSessions(_ context.Context, teacher, offering string) ([]domain.AttendanceSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, err := roster(m.state, teacher, offering); err != nil {
		return nil, err
	}
	out := make([]domain.AttendanceSession, 0)
	for _, s := range m.state.Sessions {
		if s.CourseOfferingID == offering && s.TeacherID == teacher {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SessionDate == out[j].SessionDate {
			return out[i].StartTime > out[j].StartTime
		}
		return out[i].SessionDate > out[j].SessionDate
	})
	return out, nil
}
func (m *Memory) GetAttendanceSession(_ context.Context, teacher, id string) (domain.AttendanceSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.state.Sessions {
		if s.ID == id {
			if s.TeacherID != teacher {
				return domain.AttendanceSession{}, ErrForbidden
			}
			return s, nil
		}
	}
	return domain.AttendanceSession{}, ErrNotFound
}
func validateAttendance(s domain.State, teacher string, v domain.AttendanceSession) error {
	students, err := roster(s, teacher, v.CourseOfferingID)
	if err != nil {
		return err
	}
	return validateAttendanceEntries(students, v)
}
func validateAttendanceEntries(students []domain.Student, v domain.AttendanceSession) error {
	var err error
	if _, err = time.Parse("2006-01-02", v.SessionDate); err != nil {
		return ErrInvalid
	}
	if _, err = time.Parse("15:04", v.StartTime); err != nil {
		return ErrInvalid
	}
	if len(students) == 0 || len(students) != len(v.Attendance) {
		return ErrInvalid
	}
	expected := map[string]bool{}
	for _, st := range students {
		expected[st.ID] = true
	}
	seen := map[string]bool{}
	for _, a := range v.Attendance {
		if !expected[a.StudentID] || seen[a.StudentID] {
			return ErrInvalid
		}
		switch a.Status {
		case "PRESENT", "ABSENT", "LATE", "EXCUSED":
		default:
			return ErrInvalid
		}
		seen[a.StudentID] = true
	}
	return nil
}
func (m *Memory) SaveAttendanceSession(_ context.Context, teacher string, v domain.AttendanceSession, create bool) (domain.AttendanceSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := validateAttendance(m.state, teacher, v); err != nil {
		return domain.AttendanceSession{}, err
	}
	for i := range m.state.Sessions {
		old := &m.state.Sessions[i]
		if old.CourseOfferingID == v.CourseOfferingID && old.SessionDate == v.SessionDate && old.StartTime == v.StartTime && old.ID != v.ID {
			return domain.AttendanceSession{}, ErrConflict
		}
		if !create && old.ID == v.ID {
			if old.TeacherID != teacher {
				return domain.AttendanceSession{}, ErrForbidden
			}
			v.TeacherID = teacher
			v.CreatedAt = old.CreatedAt
			*old = v
			return v, nil
		}
	}
	if !create {
		return domain.AttendanceSession{}, ErrNotFound
	}
	v.ID = uuidFor(fmt.Sprintf("session:%s:%s:%s", v.CourseOfferingID, v.SessionDate, v.StartTime))
	v.TeacherID = teacher
	v.CreatedAt = time.Now().Format(time.RFC3339)
	m.state.Sessions = append(m.state.Sessions, v)
	return v, nil
}
func overlaps(aStart, aEnd, bStart, bEnd string) bool { return aStart < bEnd && bStart < aEnd }
func roomIsMaintenance(r domain.Room) bool {
	return strings.EqualFold(r.Status, "maintenance") || strings.EqualFold(r.Status, "MAINTENANCE")
}
func availableRooms(s domain.State, teacher, offering, date, start, end string) ([]domain.Room, error) {
	students := []domain.Student{}
	if offering != "" {
		var err error
		students, err = roster(s, teacher, offering)
		if err != nil {
			return nil, err
		}
	} else if teacher != "" {
		return nil, ErrForbidden
	}
	if err := validateInterval(date, start, end); err != nil {
		return nil, err
	}
	blocked := map[string]bool{}
	for _, a := range s.RoomAllocations {
		if a.Status == "ACTIVE" && a.AllocationDate == date && overlaps(start, end, a.StartTime, a.EndTime) {
			blocked[a.RoomID] = true
		}
	}
	out := []domain.Room{}
	for _, r := range s.Rooms {
		if !roomIsMaintenance(r) && r.Capacity >= len(students) && !blocked[r.ID] {
			out = append(out, r)
		}
	}
	return out, nil
}
func roomAllocations(s domain.State, teacher string, management bool, roomID string) []domain.RoomAllocation {
	out := []domain.RoomAllocation{}
	for _, a := range s.RoomAllocations {
		if (!management && a.TeacherID != teacher) || (roomID != "" && a.RoomID != roomID) {
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].AllocationDate == out[j].AllocationDate {
			return out[i].StartTime > out[j].StartTime
		}
		return out[i].AllocationDate > out[j].AllocationDate
	})
	return out
}
func claimRoom(s *domain.State, teacher string, a domain.RoomAllocation) (domain.RoomAllocation, error) {
	students, err := roster(*s, teacher, a.CourseOfferingID)
	if err != nil {
		return domain.RoomAllocation{}, err
	}
	if err = validateInterval(a.AllocationDate, a.StartTime, a.EndTime); err != nil {
		return domain.RoomAllocation{}, err
	}
	ri := -1
	for i := range s.Rooms {
		if s.Rooms[i].ID == a.RoomID {
			ri = i
			break
		}
	}
	if ri < 0 {
		return domain.RoomAllocation{}, ErrNotFound
	}
	r := s.Rooms[ri]
	if roomIsMaintenance(r) {
		return domain.RoomAllocation{}, ErrConflict
	}
	if r.Capacity < len(students) {
		return domain.RoomAllocation{}, ErrInvalid
	}
	var offering *domain.CourseOffering
	for i := range s.Offerings {
		if s.Offerings[i].ID == a.CourseOfferingID {
			offering = &s.Offerings[i]
			break
		}
	}
	if offering == nil {
		return domain.RoomAllocation{}, ErrNotFound
	}
	for _, old := range s.RoomAllocations {
		if old.Status == "ACTIVE" && old.AllocationDate == a.AllocationDate && overlaps(a.StartTime, a.EndTime, old.StartTime, old.EndTime) && (old.RoomID == a.RoomID || old.TeacherID == teacher || old.CourseOfferingID == a.CourseOfferingID) {
			return domain.RoomAllocation{}, ErrConflict
		}
	}
	a.ID = newUUID()
	a.TeacherID = teacher
	if u, e := user(context.Background(), teacher); e == nil {
		a.TeacherName = u.Name
	}
	a.RoomNumber = r.Room
	a.CourseName = offering.Subject
	a.ClassSection = offering.Section
	a.ClassSectionID = offering.ClassSectionID
	a.Status = "ACTIVE"
	a.ClaimedAt = time.Now().Format(time.RFC3339)
	s.RoomAllocations = append(s.RoomAllocations, a)
	s.Rooms[ri].Status = "Occupied"
	return a, nil
}
func releaseRoom(s *domain.State, teacher, id string, management bool) error {
	for i := range s.RoomAllocations {
		a := &s.RoomAllocations[i]
		if a.ID != id {
			continue
		}
		if !management && a.TeacherID != teacher {
			return ErrForbidden
		}
		if a.Status != "ACTIVE" {
			return ErrConflict
		}
		a.Status = "RELEASED"
		a.ReleasedAt = time.Now().Format(time.RFC3339)
		for j := range s.Rooms {
			if s.Rooms[j].ID != a.RoomID || roomIsMaintenance(s.Rooms[j]) {
				continue
			}
			occupied := false
			for _, other := range s.RoomAllocations {
				if other.ID != a.ID && other.RoomID == a.RoomID && other.Status == "ACTIVE" {
					occupied = true
				}
			}
			if !occupied {
				s.Rooms[j].Status = "Available"
			}
		}
		return nil
	}
	return ErrNotFound
}
func saveRoom(s *domain.State, r domain.Room, create bool) error {
	if r.Room == "" || r.Capacity < 1 {
		return ErrInvalid
	}
	r.RoomType = strings.ToUpper(r.RoomType)
	switch r.RoomType {
	case "CLASSROOM", "LABORATORY", "SEMINAR_HALL", "LECTURE_HALL":
	default:
		return ErrInvalid
	}
	switch strings.ToUpper(r.Status) {
	case "", "AVAILABLE":
		r.Status = "Available"
	case "OCCUPIED":
		r.Status = "Occupied"
	case "MAINTENANCE":
		r.Status = "Maintenance"
	default:
		return ErrInvalid
	}
	if create {
		if r.Status == "Occupied" {
			return ErrInvalid
		}
		for _, old := range s.Rooms {
			if old.Room == r.Room {
				return ErrConflict
			}
		}
		r.ID = uuidFor("room:" + r.Room)
		if r.Status == "" {
			r.Status = "Available"
		}
		s.Rooms = append(s.Rooms, r)
		return nil
	}
	for i := range s.Rooms {
		if s.Rooms[i].ID == r.ID {
			if roomIsMaintenance(r) {
				for _, a := range s.RoomAllocations {
					if a.RoomID == r.ID && a.Status == "ACTIVE" {
						return ErrConflict
					}
				}
			}
			if r.Status == "Occupied" {
				active := false
				for _, a := range s.RoomAllocations {
					if a.RoomID == r.ID && a.Status == "ACTIVE" {
						active = true
					}
				}
				if !active {
					return ErrInvalid
				}
			}
			s.Rooms[i] = r
			return nil
		}
	}
	return ErrNotFound
}
func (m *Memory) AvailableRooms(_ context.Context, teacher, offering, date, start, end string) ([]domain.Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return availableRooms(m.state, teacher, offering, date, start, end)
}
func validateInterval(date, start, end string) error {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return ErrInvalid
	}
	a, e1 := time.Parse("15:04", start)
	b, e2 := time.Parse("15:04", end)
	if e1 != nil || e2 != nil || !b.After(a) {
		return ErrInvalid
	}
	return nil
}
func (m *Memory) RoomAllocations(_ context.Context, teacher string, management bool, roomID string) ([]domain.RoomAllocation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return roomAllocations(m.state, teacher, management, roomID), nil
}
func (m *Memory) ClaimRoom(_ context.Context, teacher string, a domain.RoomAllocation) (domain.RoomAllocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return claimRoom(&m.state, teacher, a)
}
func (m *Memory) ReleaseRoom(_ context.Context, teacher, id string, management bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return releaseRoom(&m.state, teacher, id, management)
}
func (m *Memory) SaveRoom(_ context.Context, r domain.Room, create bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return saveRoom(&m.state, r, create)
}
func (*Memory) Close() {}
