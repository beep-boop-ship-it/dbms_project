package transport

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"college-management/backend/internal/domain"
	"college-management/backend/internal/store"
)

type Server struct {
	repo           store.Repository
	secret, origin string
}

func New(repo store.Repository, secret, origin string) http.Handler {
	return (&Server{repo, secret, origin}).routes()
}
func (s *Server) routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { jsonOut(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		if _, e := s.repo.Snapshot(r.Context()); e != nil {
			jsonOut(w, 503, map[string]string{"status": "not_ready"})
			return
		}
		jsonOut(w, 200, map[string]string{"status": "ready"})
	})
	m.HandleFunc("POST /api/v1/auth/login", s.login)
	m.Handle("GET /api/v1/auth/me", s.auth(http.HandlerFunc(s.me)))
	m.Handle("GET /api/v1/students/me/attendance", s.authRole(http.HandlerFunc(s.attendance), "student", "teacher", "management"))
	m.Handle("GET /api/v1/students/me/marks", s.authRole(http.HandlerFunc(s.marks), "student", "teacher", "management"))
	m.Handle("GET /api/v1/rooms", s.auth(http.HandlerFunc(s.rooms)))
	m.Handle("GET /api/v1/notices", s.auth(http.HandlerFunc(s.notices)))
	m.Handle("GET /api/v1/students/me/fees", s.authRole(http.HandlerFunc(s.fees), "student", "management"))
	m.Handle("GET /api/v1/faculty/me/courses", s.authRole(http.HandlerFunc(s.courses), "teacher", "management"))
	m.Handle("GET /api/v1/courses/{id}/students", s.authRole(http.HandlerFunc(s.courseStudents), "teacher"))
	m.Handle("POST /api/v1/attendance/sessions", s.authRole(http.HandlerFunc(s.createAttendanceSession), "teacher"))
	m.Handle("GET /api/v1/attendance/sessions", s.authRole(http.HandlerFunc(s.listAttendanceSessions), "teacher"))
	m.Handle("GET /api/v1/attendance/sessions/{id}", s.authRole(http.HandlerFunc(s.getAttendanceSession), "teacher"))
	m.Handle("PUT /api/v1/attendance/sessions/{id}", s.authRole(http.HandlerFunc(s.updateAttendanceSession), "teacher"))
	m.Handle("POST /api/v1/attendance", s.authRole(http.HandlerFunc(s.updateAttendance), "teacher"))
	m.Handle("POST /api/v1/marks", s.authRole(http.HandlerFunc(s.updateMarks), "teacher"))
	m.Handle("POST /api/v1/notices", s.authRole(http.HandlerFunc(s.addNotice), "teacher", "management"))
	m.Handle("PUT /api/v1/fees/current", s.authRole(http.HandlerFunc(s.updateFees), "management"))
	m.Handle("GET /api/v1/management/library/books", s.authRole(http.HandlerFunc(s.books), "management"))
	m.Handle("POST /api/v1/management/library/books", s.authRole(http.HandlerFunc(s.createBook), "management"))
	m.Handle("PUT /api/v1/management/library/books/{id}", s.authRole(http.HandlerFunc(s.updateBook), "management"))
	m.Handle("GET /api/v1/management/library/issues", s.authRole(http.HandlerFunc(s.issues), "management"))
	m.Handle("POST /api/v1/management/library/issues", s.authRole(http.HandlerFunc(s.issueBook), "management"))
	m.Handle("POST /api/v1/management/library/issues/{id}/return", s.authRole(http.HandlerFunc(s.returnBook), "management"))
	m.Handle("GET /api/v1/students/me/library/issues", s.authRole(http.HandlerFunc(s.myIssues), "student"))
	m.Handle("GET /api/v1/rooms/available", s.authRole(http.HandlerFunc(s.availableRooms), "teacher", "management"))
	m.Handle("GET /api/v1/rooms/allocations", s.authRole(http.HandlerFunc(s.listRoomAllocations), "teacher", "management"))
	m.Handle("GET /api/v1/rooms/allocations/current", s.authRole(http.HandlerFunc(s.currentRoomAllocations), "teacher"))
	m.Handle("POST /api/v1/rooms/allocations", s.authRole(http.HandlerFunc(s.claimRoom), "teacher"))
	m.Handle("POST /api/v1/rooms/allocations/{id}/release", s.authRole(http.HandlerFunc(s.releaseRoom), "teacher"))
	m.Handle("GET /api/v1/management/rooms", s.authRole(http.HandlerFunc(s.managementRooms), "management"))
	m.Handle("POST /api/v1/management/rooms", s.authRole(http.HandlerFunc(s.createRoom), "management"))
	m.Handle("PUT /api/v1/management/rooms/{id}", s.authRole(http.HandlerFunc(s.updateRoom), "management"))
	m.Handle("GET /api/v1/management/rooms/{id}/allocations", s.authRole(http.HandlerFunc(s.roomHistory), "management"))
	return s.cors(m)
}
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (s.origin == "*" || origin == s.origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type claims struct {
	ID, Role string
	Exp      int64
}

func (s *Server) token(c claims) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s|%s|%d", c.ID, c.Role, c.Exp)))
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (s *Server) parse(token string) (claims, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return claims{}, false
	}
	raw, e := base64.RawURLEncoding.DecodeString(parts[0])
	if e != nil {
		return claims{}, false
	}
	sig, e := base64.RawURLEncoding.DecodeString(parts[1])
	if e != nil {
		return claims{}, false
	}
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return claims{}, false
	}
	v := strings.Split(string(raw), "|")
	if len(v) != 3 {
		return claims{}, false
	}
	var exp int64
	if _, e = fmt.Sscan(v[2], &exp); e != nil || time.Now().Unix() > exp {
		return claims{}, false
	}
	return claims{v[0], v[1], exp}, true
}

type contextKey string

const userKey contextKey = "campusUser"

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		c, ok := s.parse(h)
		if !ok {
			jsonOut(w, 401, map[string]string{"error": "A valid session is required."})
			return
		}
		u, e := s.repo.GetUser(r.Context(), c.ID)
		if e != nil || u.Role != c.Role {
			jsonOut(w, 401, map[string]string{"error": "A valid session is required."})
			return
		}
		next.ServeHTTP(w, contextWithUser(r, u))
	})
}
func contextWithUser(r *http.Request, u domain.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userKey, u))
}
func (s *Server) authRole(next http.Handler, roles ...string) http.Handler {
	return s.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := r.Context().Value(userKey).(domain.User)
		for _, role := range roles {
			if u.Role == role {
				next.ServeHTTP(w, r)
				return
			}
		}
		jsonOut(w, 403, map[string]string{"error": "Your role cannot access this resource."})
	}))
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var c domain.Credentials
	if !decode(w, r, &c) {
		return
	}
	u, e := s.repo.Authenticate(r.Context(), c)
	if e != nil {
		jsonOut(w, 401, map[string]string{"error": "Invalid username, password, or role."})
		return
	}
	jsonOut(w, 200, map[string]any{"token": s.token(claims{u.ID, u.Role, time.Now().Add(12 * time.Hour).Unix()}), "user": u})
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, 200, r.Context().Value(userKey))
}
func (s *Server) state(w http.ResponseWriter, r *http.Request) (domain.State, bool) {
	v, e := s.repo.Snapshot(r.Context())
	if e != nil {
		jsonOut(w, 500, map[string]string{"error": "Unable to load campus records."})
		return domain.State{}, false
	}
	return v, true
}
func (s *Server) attendance(w http.ResponseWriter, r *http.Request) {
	v, ok := s.state(w, r)
	if ok {
		jsonOut(w, 200, map[string]any{"items": v.Attendance})
	}
}
func (s *Server) marks(w http.ResponseWriter, r *http.Request) {
	v, ok := s.state(w, r)
	if ok {
		jsonOut(w, 200, map[string]any{"items": v.Marks})
	}
}
func (s *Server) rooms(w http.ResponseWriter, r *http.Request) {
	rows, err := s.repo.ListRooms(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	jsonOut(w, 200, map[string]any{"items": rows})
}
func (s *Server) notices(w http.ResponseWriter, r *http.Request) {
	v, ok := s.state(w, r)
	if ok {
		jsonOut(w, 200, map[string]any{"items": v.Notices})
	}
}
func (s *Server) fees(w http.ResponseWriter, r *http.Request) {
	v, ok := s.state(w, r)
	if ok {
		if v.Fees.Due == "" {
			jsonOut(w, http.StatusOK, nil)
			return
		}
		jsonOut(w, 200, v.Fees)
	}
}
func (s *Server) courses(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(domain.User)
	courses, err := s.repo.ListCourseOfferings(r.Context(), u.ID, u.Role == "management")
	if err != nil {
		writeError(w, err)
		return
	}
	jsonOut(w, 200, map[string]any{"items": courses})
}
func (s *Server) updateAttendance(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Code     string `json:"code"`
		Attended int    `json:"attended"`
		Total    int    `json:"total"`
	}
	if !decode(w, r, &v) {
		return
	}
	if e := s.repo.UpdateAttendance(r.Context(), v.Code, v.Attended, v.Total); e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 200, map[string]string{"status": "updated"})
}
func (s *Server) courseStudents(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(domain.User)
	rows, e := s.repo.GetCourseRoster(r.Context(), u.ID, r.PathValue("id"))
	if e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 200, map[string]any{"items": rows, "total": len(rows)})
}
func (s *Server) createAttendanceSession(w http.ResponseWriter, r *http.Request) {
	var v domain.AttendanceSession
	if !decode(w, r, &v) {
		return
	}
	u := r.Context().Value(userKey).(domain.User)
	saved, e := s.repo.SaveAttendanceSession(r.Context(), u.ID, v, true)
	if e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 201, saved)
}
func (s *Server) listAttendanceSessions(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(domain.User)
	offering := r.URL.Query().Get("course_offering_id")
	if offering == "" {
		jsonOut(w, 400, map[string]string{"error": "course_offering_id is required."})
		return
	}
	rows, e := s.repo.AttendanceSessions(r.Context(), u.ID, offering)
	if e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 200, map[string]any{"items": rows})
}
func (s *Server) getAttendanceSession(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(domain.User)
	v, e := s.repo.GetAttendanceSession(r.Context(), u.ID, r.PathValue("id"))
	if e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 200, v)
}
func (s *Server) updateAttendanceSession(w http.ResponseWriter, r *http.Request) {
	var v domain.AttendanceSession
	if !decode(w, r, &v) {
		return
	}
	u := r.Context().Value(userKey).(domain.User)
	v.ID = r.PathValue("id")
	saved, e := s.repo.SaveAttendanceSession(r.Context(), u.ID, v, false)
	if e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 200, saved)
}
func (s *Server) updateMarks(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Code     string `json:"code"`
		Internal int    `json:"internal"`
		Midterm  int    `json:"midterm"`
	}
	if !decode(w, r, &v) {
		return
	}
	if e := s.repo.UpdateMarks(r.Context(), v.Code, v.Internal, v.Midterm); e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 200, map[string]string{"status": "updated"})
}
func (s *Server) addNotice(w http.ResponseWriter, r *http.Request) {
	var n domain.Notice
	if !decode(w, r, &n) {
		return
	}
	n.ID = fmt.Sprintf("notice-%d", time.Now().UnixNano())
	n.Date = "Just now"
	if e := s.repo.AddNotice(r.Context(), n); e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 201, n)
}
func (s *Server) updateFees(w http.ResponseWriter, r *http.Request) {
	var f domain.Fees
	if !decode(w, r, &f) {
		return
	}
	if e := s.repo.UpdateFees(r.Context(), f); e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 200, f)
}
func (s *Server) books(w http.ResponseWriter, r *http.Request) {
	v, ok := s.state(w, r)
	if ok {
		jsonOut(w, 200, map[string]any{"items": v.Books})
	}
}
func (s *Server) issues(w http.ResponseWriter, r *http.Request) {
	v, ok := s.state(w, r)
	if ok {
		jsonOut(w, 200, map[string]any{"items": v.Issues})
	}
}
func (s *Server) myIssues(w http.ResponseWriter, r *http.Request) {
	v, ok := s.state(w, r)
	if !ok {
		return
	}
	u := r.Context().Value(userKey).(domain.User)
	rows := make([]domain.Issue, 0)
	for _, x := range v.Issues {
		if x.StudentID == u.StudentID {
			rows = append(rows, x)
		}
	}
	jsonOut(w, 200, map[string]any{"items": rows})
}
func (s *Server) createBook(w http.ResponseWriter, r *http.Request) {
	var b domain.Book
	if !decode(w, r, &b) {
		return
	}
	if e := s.repo.SaveBook(r.Context(), b, true); e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 201, map[string]string{"status": "created"})
}
func (s *Server) updateBook(w http.ResponseWriter, r *http.Request) {
	var b domain.Book
	if !decode(w, r, &b) {
		return
	}
	b.ID = r.PathValue("id")
	if e := s.repo.SaveBook(r.Context(), b, false); e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 200, map[string]string{"status": "updated"})
}
func (s *Server) issueBook(w http.ResponseWriter, r *http.Request) {
	var v domain.Issue
	if !decode(w, r, &v) {
		return
	}
	student, err := s.repo.GetStudentByNumber(r.Context(), v.StudentID)
	if err != nil {
		writeError(w, err)
		return
	}
	v.StudentName = student.Name
	if e := s.repo.IssueBook(r.Context(), v); e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 201, map[string]string{"status": "issued"})
}
func (s *Server) returnBook(w http.ResponseWriter, r *http.Request) {
	var v struct {
		ReturnedAt string `json:"returnedAt"`
	}
	if r.Body != nil && r.ContentLength != 0 && !decode(w, r, &v) {
		return
	}
	if e := s.repo.ReturnBook(r.Context(), r.PathValue("id"), v.ReturnedAt); e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 200, map[string]string{"status": "returned"})
}
func (s *Server) availableRooms(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(domain.User)
	q := r.URL.Query()
	teacher, offering := u.ID, q.Get("course_offering_id")
	if u.Role == "management" {
		teacher, offering = "", ""
	}
	rows, e := s.repo.AvailableRooms(r.Context(), teacher, offering, q.Get("allocation_date"), q.Get("start_time"), q.Get("end_time"))
	if e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 200, map[string]any{"items": rows, "total": len(rows)})
}
func (s *Server) listRoomAllocations(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(domain.User)
	rows, e := s.repo.RoomAllocations(r.Context(), u.ID, u.Role == "management", r.URL.Query().Get("room_id"))
	if e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 200, map[string]any{"items": rows})
}
func (s *Server) currentRoomAllocations(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(domain.User)
	rows, e := s.repo.RoomAllocations(r.Context(), u.ID, false, "")
	if e != nil {
		writeError(w, e)
		return
	}
	active := []domain.RoomAllocation{}
	for _, a := range rows {
		if a.Status == "ACTIVE" {
			active = append(active, a)
		}
	}
	jsonOut(w, 200, map[string]any{"items": active})
}
func (s *Server) claimRoom(w http.ResponseWriter, r *http.Request) {
	var a domain.RoomAllocation
	if !decode(w, r, &a) {
		return
	}
	u := r.Context().Value(userKey).(domain.User)
	saved, e := s.repo.ClaimRoom(r.Context(), u.ID, a)
	if e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 201, saved)
}
func (s *Server) releaseRoom(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(domain.User)
	if e := s.repo.ReleaseRoom(r.Context(), u.ID, r.PathValue("id"), false); e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 200, map[string]string{"status": "RELEASED"})
}
func (s *Server) managementRooms(w http.ResponseWriter, r *http.Request) {
	rows, err := s.repo.ListRooms(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	jsonOut(w, 200, map[string]any{"items": rows})
}
func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	var room domain.Room
	if !decode(w, r, &room) {
		return
	}
	if e := s.repo.SaveRoom(r.Context(), room, true); e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 201, map[string]string{"status": "created"})
}
func (s *Server) updateRoom(w http.ResponseWriter, r *http.Request) {
	var room domain.Room
	if !decode(w, r, &room) {
		return
	}
	room.ID = r.PathValue("id")
	if e := s.repo.SaveRoom(r.Context(), room, false); e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 200, map[string]string{"status": "updated"})
}
func (s *Server) roomHistory(w http.ResponseWriter, r *http.Request) {
	rows, e := s.repo.RoomAllocations(r.Context(), "", true, r.PathValue("id"))
	if e != nil {
		writeError(w, e)
		return
	}
	jsonOut(w, 200, map[string]any{"items": rows})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	defer r.Body.Close()
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		jsonOut(w, 400, map[string]string{"error": "Invalid JSON request."})
		return false
	}
	return true
}
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, e error) {
	status := 400
	switch {
	case errors.Is(e, store.ErrNotFound):
		status = 404
	case errors.Is(e, store.ErrForbidden):
		status = 403
	case errors.Is(e, store.ErrConflict):
		status = 409
	case !errors.Is(e, store.ErrInvalid):
		status = 500
	}
	jsonOut(w, status, map[string]string{"error": e.Error()})
}
