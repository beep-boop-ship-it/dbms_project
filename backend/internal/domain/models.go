package domain

type User struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	StudentID  string `json:"studentId,omitempty"`
	Department string `json:"department,omitempty"`
	Program    string `json:"program,omitempty"`
	Semester   string `json:"semester,omitempty"`
}
type Attendance struct {
	Subject  string `json:"subject"`
	Code     string `json:"code"`
	Attended int    `json:"attended"`
	Total    int    `json:"total"`
}
type Mark struct {
	Subject  string `json:"subject"`
	Code     string `json:"code"`
	Internal int    `json:"internal"`
	Midterm  int    `json:"midterm"`
	Total    int    `json:"total"`
	Max      int    `json:"max"`
}
type Room struct {
	ID       string `json:"id"`
	Room     string `json:"room"`
	Block    string `json:"block"`
	Floor    string `json:"floor"`
	Capacity int    `json:"capacity"`
	Status   string `json:"status"`
	Use      string `json:"use"`
	RoomType string `json:"room_type"`
}
type Notice struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Date  string `json:"date"`
	Type  string `json:"type"`
}
type Fees struct {
	Amount int    `json:"amount"`
	Due    string `json:"due"`
	Term   string `json:"term"`
	Paid   int    `json:"paid"`
}
type Course struct {
	Code     string `json:"code"`
	Subject  string `json:"subject"`
	Section  string `json:"section"`
	Schedule string `json:"schedule"`
	Room     string `json:"room"`
	Enrolled int    `json:"enrolled"`
}
type Student struct {
	ID               string `json:"id"`
	EnrollmentNumber string `json:"enrollmentNumber"`
	Name             string `json:"name"`
	Active           bool   `json:"active"`
}
type Enrollment struct {
	StudentID        string `json:"studentId"`
	CourseOfferingID string `json:"courseOfferingId"`
	Active           bool   `json:"active"`
}
type CourseOffering struct {
	ID             string `json:"id"`
	Code           string `json:"code"`
	Subject        string `json:"subject"`
	TeacherID      string `json:"teacherId"`
	ClassSectionID string `json:"classSectionId"`
	Section        string `json:"section"`
	Schedule       string `json:"schedule"`
	Room           string `json:"room"`
	Capacity       int    `json:"capacity"`
	Enrolled       int    `json:"enrolled"`
}
type AttendanceEntry struct {
	StudentID string `json:"student_id"`
	Status    string `json:"status"`
	Remarks   string `json:"remarks,omitempty"`
}
type AttendanceSession struct {
	ID               string            `json:"id"`
	CourseOfferingID string            `json:"course_offering_id"`
	TeacherID        string            `json:"teacher_id"`
	SessionDate      string            `json:"session_date"`
	StartTime        string            `json:"start_time"`
	CreatedAt        string            `json:"created_at"`
	Attendance       []AttendanceEntry `json:"attendance"`
}
type Book struct {
	ID              string `json:"id"`
	ISBN            string `json:"isbn"`
	Title           string `json:"title"`
	Author          string `json:"author"`
	Category        string `json:"category"`
	TotalCopies     int    `json:"totalCopies"`
	AvailableCopies int    `json:"availableCopies"`
}
type Issue struct {
	ID          string `json:"id"`
	BookID      string `json:"bookId"`
	BookTitle   string `json:"bookTitle"`
	StudentID   string `json:"studentId"`
	StudentName string `json:"studentName"`
	IssuedAt    string `json:"issuedAt"`
	DueAt       string `json:"dueAt"`
	ReturnedAt  string `json:"returnedAt,omitempty"`
}
type State struct {
	Attendance      []Attendance        `json:"attendance"`
	Marks           []Mark              `json:"marks"`
	Rooms           []Room              `json:"rooms"`
	Notices         []Notice            `json:"notices"`
	Fees            Fees                `json:"fees"`
	Courses         []Course            `json:"courses"`
	Books           []Book              `json:"books"`
	Issues          []Issue             `json:"issues"`
	Offerings       []CourseOffering    `json:"courseOfferings"`
	Students        []Student           `json:"students"`
	Enrollments     []Enrollment        `json:"enrollments"`
	Sessions        []AttendanceSession `json:"attendanceSessions"`
	RoomAllocations []RoomAllocation    `json:"roomAllocations"`
}
type RoomAllocation struct {
	ID               string `json:"id"`
	RoomID           string `json:"room_id"`
	RoomNumber       string `json:"room_number"`
	CourseOfferingID string `json:"course_offering_id"`
	CourseName       string `json:"course_name"`
	TeacherID        string `json:"teacher_id"`
	TeacherName      string `json:"teacher_name"`
	ClassSectionID   string `json:"class_section_id"`
	ClassSection     string `json:"class_section"`
	AllocationDate   string `json:"allocation_date"`
	StartTime        string `json:"start_time"`
	EndTime          string `json:"end_time"`
	Status           string `json:"status"`
	ClaimedAt        string `json:"claimed_at"`
	ReleasedAt       string `json:"released_at,omitempty"`
}
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}
