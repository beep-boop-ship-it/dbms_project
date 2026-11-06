package transport

import (
	"bytes"
	"college-management/backend/internal/domain"
	"college-management/backend/internal/store"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthenticationAndAuthorization(t *testing.T) {
	handler := New(store.NewMemory(), "test-secret", "")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/me/fees", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("unauthenticated request status=%d, want 401", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"nisha@northstar.edu","password":"demo1234","role":"teacher"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("login status=%d", rec.Code)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil || out.Token == "" {
		t.Fatalf("login response missing token: %v", err)
	}
	req = httptest.NewRequest(http.MethodPut, "/api/v1/fees/current", bytes.NewBufferString(`{"amount":100,"due":"2026-12-01"}`))
	req.Header.Set("Authorization", "Bearer "+out.Token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("teacher fee write status=%d, want 403", rec.Code)
	}
}

func TestAttendanceAPIReturnsAndSavesWholeRoster(t *testing.T) {
	h := New(store.NewMemory(), "test-secret", "")
	teacherToken := loginToken(t, h, "nisha@northstar.edu", "teacher")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/faculty/me/courses", nil)
	req.Header.Set("Authorization", "Bearer "+teacherToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("courses status=%d %s", rec.Code, rec.Body.String())
	}
	var courses struct {
		Items []domain.CourseOffering `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&courses); err != nil || len(courses.Items) == 0 {
		t.Fatalf("no assigned offerings: %v", err)
	}
	offering := courses.Items[0]
	req = httptest.NewRequest(http.MethodGet, "/api/v1/courses/"+offering.ID+"/students", nil)
	req.Header.Set("Authorization", "Bearer "+teacherToken)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var roster struct {
		Items []domain.Student `json:"items"`
		Total int              `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&roster); err != nil || rec.Code != 200 || roster.Total != len(roster.Items) || roster.Total != 6 {
		t.Fatalf("incomplete roster status=%d total=%d items=%d err=%v", rec.Code, roster.Total, len(roster.Items), err)
	}
	entries := make([]domain.AttendanceEntry, 0, len(roster.Items))
	for _, student := range roster.Items {
		entries = append(entries, domain.AttendanceEntry{StudentID: student.ID, Status: "PRESENT"})
	}
	payload, _ := json.Marshal(domain.AttendanceSession{CourseOfferingID: offering.ID, SessionDate: time.Now().Format("2006-01-02"), StartTime: "15:15", Attendance: entries})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/attendance/sessions", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+teacherToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("session submit status=%d body=%s", rec.Code, rec.Body.String())
	}
	var session domain.AttendanceSession
	if err := json.NewDecoder(rec.Body).Decode(&session); err != nil || len(session.Attendance) != roster.Total {
		t.Fatalf("session response incomplete: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/attendance/sessions/"+session.ID, nil)
	req.Header.Set("Authorization", "Bearer "+teacherToken)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("session review status=%d", rec.Code)
	}
	managementToken := loginToken(t, h, "admin@northstar.edu", "management")
	req = httptest.NewRequest(http.MethodGet, "/api/v1/attendance/sessions/"+session.ID, nil)
	req.Header.Set("Authorization", "Bearer "+managementToken)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("unauthorized attendance review status=%d", rec.Code)
	}
}

func loginToken(t *testing.T, h http.Handler, username, role string) string {
	t.Helper()
	body := fmt.Sprintf(`{"username":%q,"password":"demo1234","role":%q}`, username, role)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("login %s status=%d body=%s", role, rec.Code, rec.Body.String())
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out.Token
}
