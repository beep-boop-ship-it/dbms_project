package store

import (
	"college-management/backend/internal/domain"
	"context"
	"testing"
	"time"
)

func TestDemoAuthenticationAndWrites(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	if _, err := m.Authenticate(ctx, domain.Credentials{Username: "2024bcs1021", Password: "demo1234", Role: "student"}); err != nil {
		t.Fatalf("valid student login: %v", err)
	}
	if _, err := m.Authenticate(ctx, domain.Credentials{Username: "2024bcs1021", Password: "wrong", Role: "student"}); err == nil {
		t.Fatal("invalid password was accepted")
	}
	if _, err := m.Authenticate(ctx, domain.Credentials{Username: "admin@northstar.edu", Password: "demo1234", Role: "management"}); err != nil {
		t.Fatalf("valid management login: %v", err)
	}
	if err := m.UpdateAttendance(ctx, "CS-301", 24, 27); err != nil {
		t.Fatalf("attendance update: %v", err)
	}
	s, _ := m.Snapshot(ctx)
	if s.Attendance[0].Attended != 24 || s.Attendance[0].Total != 27 {
		t.Fatalf("attendance not persisted in memory: %+v", s.Attendance[0])
	}
	if err := m.UpdateAttendance(ctx, "CS-301", 28, 27); err != ErrInvalid {
		t.Fatalf("invalid attendance should fail, got %v", err)
	}
	if err := m.UpdateMarks(ctx, "CS-301", 25, 19); err != nil {
		t.Fatalf("marks update: %v", err)
	}
	s, _ = m.Snapshot(ctx)
	if s.Marks[0].Total != 44 {
		t.Fatalf("marks total should recalculate, got %d", s.Marks[0].Total)
	}
}

func TestAttendanceSessionCoversAllActiveStudentsAndUpdatesAtomically(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	state, _ := m.Snapshot(ctx)
	offering := state.Offerings[0]
	roster, err := m.GetCourseRoster(ctx, "u-teacher", offering.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(roster) != 6 {
		t.Fatalf("roster count=%d, want all 6 seeded active enrollments", len(roster))
	}
	entries := make([]domain.AttendanceEntry, 0, len(roster))
	for i, s := range roster {
		status := "PRESENT"
		if i == 1 {
			status = "LATE"
		}
		if i == 2 {
			status = "ABSENT"
		}
		if i == 3 {
			status = "EXCUSED"
		}
		entries = append(entries, domain.AttendanceEntry{StudentID: s.ID, Status: status})
	}
	session := domain.AttendanceSession{CourseOfferingID: offering.ID, SessionDate: time.Now().Format("2006-01-02"), StartTime: "14:00", Attendance: entries}
	saved, err := m.SaveAttendanceSession(ctx, "u-teacher", session, true)
	if err != nil {
		t.Fatalf("save full class: %v", err)
	}
	if len(saved.Attendance) != len(roster) {
		t.Fatal("saved session omitted enrolled students")
	}
	if _, err = m.SaveAttendanceSession(ctx, "u-other", session, true); err != ErrForbidden {
		t.Fatalf("unassigned teacher should be rejected: %v", err)
	}
	if _, err = m.SaveAttendanceSession(ctx, "u-teacher", session, true); err != ErrConflict {
		t.Fatalf("duplicate session should be rejected: %v", err)
	}
	if _, err = m.GetAttendanceSession(ctx, "u-other", saved.ID); err != ErrForbidden {
		t.Fatalf("another teacher should not review this session: %v", err)
	}
	saved.Attendance[0].Status = "ABSENT"
	if _, err = m.SaveAttendanceSession(ctx, "u-teacher", saved, false); err != nil {
		t.Fatalf("update session: %v", err)
	}
	incomplete := saved
	incomplete.ID = ""
	incomplete.Attendance = incomplete.Attendance[:len(incomplete.Attendance)-1]
	if _, err = m.SaveAttendanceSession(ctx, "u-teacher", incomplete, true); err != ErrInvalid {
		t.Fatalf("partial submission must be rejected: %v", err)
	}
}

func TestRoomClaimOverlapMaintenanceAndOwnership(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	state, _ := m.Snapshot(ctx)
	offering := state.Offerings[0]
	secondary := state.Offerings[1]
	secondary.ID = uuidFor("offering:other-teacher")
	secondary.TeacherID = "u-other"
	m.mu.Lock()
	m.state.Offerings = append(m.state.Offerings, secondary)
	for _, student := range state.Students {
		m.state.Enrollments = append(m.state.Enrollments, domain.Enrollment{StudentID: student.ID, CourseOfferingID: secondary.ID, Active: true})
	}
	m.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	available, err := m.AvailableRooms(ctx, "u-teacher", offering.ID, today, "09:30", "10:30")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range available {
		if r.Room == "B204" {
			t.Fatal("overlapping active allocation was shown as available")
		}
	}
	var chosen domain.Room
	for _, r := range available {
		if r.Status == "Available" {
			chosen = r
			break
		}
	}
	if chosen.ID == "" {
		t.Fatal("no available demo room")
	}
	a, err := m.ClaimRoom(ctx, "u-teacher", domain.RoomAllocation{RoomID: chosen.ID, CourseOfferingID: offering.ID, AllocationDate: today, StartTime: "12:00", EndTime: "13:00"})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	chosen.Status = "Maintenance"
	if err = m.SaveRoom(ctx, chosen, false); err != ErrConflict {
		t.Fatalf("a room with an active allocation cannot enter maintenance: %v", err)
	}
	if _, err = m.ClaimRoom(ctx, "u-teacher", domain.RoomAllocation{RoomID: chosen.ID, CourseOfferingID: offering.ID, AllocationDate: today, StartTime: "12:30", EndTime: "13:30"}); err != ErrConflict {
		t.Fatalf("overlap must be rejected: %v", err)
	}
	if _, err = m.ClaimRoom(ctx, "u-other", domain.RoomAllocation{RoomID: chosen.ID, CourseOfferingID: secondary.ID, AllocationDate: today, StartTime: "12:30", EndTime: "13:30"}); err != ErrConflict {
		t.Fatalf("second teacher cannot take overlapping room: %v", err)
	}
	if err = m.ReleaseRoom(ctx, "u-other", a.ID, false); err != ErrForbidden {
		t.Fatalf("another teacher cannot release: %v", err)
	}
	if err = m.ReleaseRoom(ctx, "u-teacher", a.ID, false); err != nil {
		t.Fatalf("owner release: %v", err)
	}
	rows, err := m.RoomAllocations(ctx, "u-teacher", false, chosen.ID)
	if err != nil || len(rows) != 1 || rows[0].Status != "RELEASED" {
		t.Fatalf("released history not retained: %+v, %v", rows, err)
	}
	maintenance := ""
	for _, r := range state.Rooms {
		if r.Status == "Maintenance" {
			maintenance = r.ID
			break
		}
	}
	if maintenance != "" {
		if _, err = m.ClaimRoom(ctx, "u-teacher", domain.RoomAllocation{RoomID: maintenance, CourseOfferingID: offering.ID, AllocationDate: today, StartTime: "16:00", EndTime: "17:00"}); err != ErrConflict {
			t.Fatalf("maintenance room should not be claimable: %v", err)
		}
	}
}

func TestLibraryInventoryLifecycle(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	if err := m.IssueBook(ctx, domain.Issue{BookID: "book-002", StudentID: "2024BCS1021", DueAt: "2026-12-31"}); err != nil {
		t.Fatalf("issue book: %v", err)
	}
	if err := m.IssueBook(ctx, domain.Issue{BookID: "book-002", StudentID: "2024BCS1021", DueAt: "2026-12-31"}); err != ErrConflict {
		t.Fatalf("duplicate active issue should fail, got %v", err)
	}
	s, _ := m.Snapshot(ctx)
	if s.Books[1].AvailableCopies != 3 {
		t.Fatalf("available copies after issue=%d", s.Books[1].AvailableCopies)
	}
	issue := s.Issues[len(s.Issues)-1]
	if err := m.ReturnBook(ctx, issue.ID, "2026-11-06"); err != nil {
		t.Fatalf("return issued book: %v", err)
	}
	if err := m.ReturnBook(ctx, issue.ID, "2026-11-06"); err != ErrConflict {
		t.Fatalf("duplicate return should fail, got %v", err)
	}
	s, _ = m.Snapshot(ctx)
	if s.Books[1].AvailableCopies != 4 {
		t.Fatalf("available copies after return=%d", s.Books[1].AvailableCopies)
	}
}
