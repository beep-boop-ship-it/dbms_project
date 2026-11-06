package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"college-management/backend/internal/domain"
)

// Run with POSTGRES_TEST_DATABASE_URL against a disposable database after
// migrations/001_initial.sql, migrations/002_attendance_rooms.sql and
// seeds/demo.sql have been applied.
func TestPostgresRepositoryAgainstSeededDatabase(t *testing.T) {
	url := os.Getenv("POSTGRES_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("POSTGRES_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p, err := NewPostgres(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	var originalState []byte
	if err = p.pool.QueryRow(ctx, "SELECT payload FROM campus_state WHERE id=1").Scan(&originalState); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = p.pool.Exec(context.Background(), "UPDATE campus_state SET payload=$1::jsonb,updated_at=now() WHERE id=1", originalState)
	})

	for _, account := range []domain.Credentials{
		{Username: "admin@northstar.edu", Password: "demo1234", Role: "management"},
		{Username: "nisha@northstar.edu", Password: "demo1234", Role: "teacher"},
		{Username: "2024BCS1021", Password: "demo1234", Role: "student"},
	} {
		if _, err = p.Authenticate(ctx, account); err != nil {
			t.Fatalf("seeded %s authentication: %v", account.Role, err)
		}
	}
	teacher, err := p.Authenticate(ctx, domain.Credentials{Username: "nisha@northstar.edu", Password: "demo1234", Role: "teacher"})
	if err != nil {
		t.Fatal(err)
	}
	offerings, err := p.ListCourseOfferings(ctx, teacher.ID, false)
	if err != nil || len(offerings) != 4 {
		t.Fatalf("course offerings: count=%d err=%v", len(offerings), err)
	}
	roster, err := p.GetCourseRoster(ctx, teacher.ID, offerings[0].ID)
	if err != nil || len(roster) != 6 {
		t.Fatalf("complete roster: count=%d err=%v", len(roster), err)
	}
	if _, err = p.AttendanceSessions(ctx, teacher.ID, offerings[0].ID); err != nil {
		t.Fatal(err)
	}

	date := "2999-12-29"
	entries := make([]domain.AttendanceEntry, 0, len(roster))
	for i, student := range roster {
		status := "PRESENT"
		if i == 1 {
			status = "LATE"
		}
		entries = append(entries, domain.AttendanceEntry{StudentID: student.ID, Status: status})
	}
	session, err := p.SaveAttendanceSession(ctx, teacher.ID, domain.AttendanceSession{CourseOfferingID: offerings[0].ID, SessionDate: date, StartTime: "20:00", Attendance: entries}, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = p.pool.Exec(context.Background(), "DELETE FROM attendance_sessions WHERE id::text=$1", session.ID)
	})
	fetched, err := p.GetAttendanceSession(ctx, teacher.ID, session.ID)
	if err != nil || len(fetched.Attendance) != len(roster) {
		t.Fatalf("attendance fetch: count=%d err=%v", len(fetched.Attendance), err)
	}
	fetched.Attendance[0].Status = "ABSENT"
	if _, err = p.SaveAttendanceSession(ctx, teacher.ID, fetched, false); err != nil {
		t.Fatalf("attendance update: %v", err)
	}

	rooms, err := p.ListRooms(ctx)
	if err != nil || len(rooms) < 20 {
		t.Fatalf("rooms: count=%d err=%v", len(rooms), err)
	}
	available, err := p.AvailableRooms(ctx, teacher.ID, offerings[0].ID, date, "20:00", "21:00")
	if err != nil || len(available) == 0 {
		t.Fatalf("available rooms: count=%d err=%v", len(available), err)
	}
	allocation, err := p.ClaimRoom(ctx, teacher.ID, domain.RoomAllocation{RoomID: available[0].ID, CourseOfferingID: offerings[0].ID, AllocationDate: date, StartTime: "20:00", EndTime: "21:00"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = p.pool.Exec(context.Background(), "DELETE FROM room_allocations WHERE id::text=$1", allocation.ID)
	})
	if err = p.ReleaseRoom(ctx, teacher.ID, allocation.ID, false); err != nil {
		t.Fatal(err)
	}
	allocations, err := p.RoomAllocations(ctx, teacher.ID, false, available[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findAllocation(allocations, allocation.ID, "RELEASED"); !ok {
		t.Fatalf("released allocation missing from history: %+v", allocations)
	}

	roomNumber := fmt.Sprintf("TEST-%d", time.Now().UnixNano())
	if err = p.SaveRoom(ctx, domain.Room{Room: roomNumber, Block: "T", Floor: "4 Floor", Capacity: 12, Status: "Available", RoomType: "CLASSROOM"}, true); err != nil {
		t.Fatal(err)
	}
	var roomID string
	if err = p.pool.QueryRow(ctx, "SELECT id::text FROM rooms WHERE room_number=$1", roomNumber).Scan(&roomID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = p.pool.Exec(context.Background(), "DELETE FROM rooms WHERE id::text=$1", roomID) })
	if err = p.SaveRoom(ctx, domain.Room{ID: roomID, Room: roomNumber, Block: "T", Floor: "5 Floor", Capacity: 14, Status: "Available", RoomType: "LABORATORY"}, false); err != nil {
		t.Fatalf("room update: %v", err)
	}

	// Exercise the JSONB-backed legacy modules and restore the seed snapshot at cleanup.
	if err = p.UpdateAttendance(ctx, "CS-301", 24, 26); err != nil {
		t.Fatal(err)
	}
	if err = p.UpdateMarks(ctx, "CS-301", 25, 19); err != nil {
		t.Fatal(err)
	}
	if err = p.AddNotice(ctx, domain.Notice{Title: "Integration test", Type: "Test"}); err != nil {
		t.Fatal(err)
	}
	if err = p.UpdateFees(ctx, domain.Fees{Amount: 42500, Paid: 0, Due: "2999-12-31", Term: "Integration test"}); err != nil {
		t.Fatal(err)
	}
	if err = p.SaveBook(ctx, domain.Book{Title: "Integration Book", Author: "Test Author", Category: "Testing", TotalCopies: 1}, true); err != nil {
		t.Fatal(err)
	}
	state, err := p.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var bookID string
	for _, book := range state.Books {
		if book.Title == "Integration Book" {
			bookID = book.ID
		}
	}
	if bookID == "" {
		t.Fatal("JSONB-backed book was not persisted")
	}
	if err = p.IssueBook(ctx, domain.Issue{BookID: bookID, StudentID: "2024BCS1021", DueAt: "2999-12-31"}); err != nil {
		t.Fatal(err)
	}
	state, err = p.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range state.Issues {
		if issue.BookID == bookID {
			if err = p.ReturnBook(ctx, issue.ID, "2999-12-30"); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err = json.Marshal(state); err != nil {
		t.Fatal(err)
	}
}

func findAllocation(rows []domain.RoomAllocation, id, status string) (domain.RoomAllocation, bool) {
	for _, row := range rows {
		if row.ID == id && row.Status == status {
			return row, true
		}
	}
	return domain.RoomAllocation{}, false
}
