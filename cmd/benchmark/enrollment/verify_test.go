package main

import (
	"strings"
	"testing"
)

func TestSelectionVerificationSnapshotAcceptsConvergedState(t *testing.T) {
	snapshot := selectionVerificationSnapshot{
		Capacity:             500,
		ClassSelected:        500,
		Applications:         500,
		SelectedApplications: 500,
		DistinctStudents:     500,
		DistinctRequests:     500,
		Enrollments:          500,
		EnrollmentStudents:   500,
		OutboxEvents:         500,
		OutboxPublished:      500,
		Notifications:        500,
		RedisSeats:           0,
	}

	if err := snapshot.fatalViolation(500); err != nil {
		t.Fatalf("fatalViolation() error = %v", err)
	}
	if err := snapshot.validate(500); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
}

func TestSelectionVerificationSnapshotWaitsForNotificationConvergence(t *testing.T) {
	snapshot := selectionVerificationSnapshot{
		Capacity:             500,
		ClassSelected:        500,
		Applications:         500,
		SelectedApplications: 500,
		DistinctStudents:     500,
		DistinctRequests:     500,
		Enrollments:          500,
		EnrollmentStudents:   500,
		OutboxEvents:         500,
		OutboxPublished:      490,
		OutboxUnpublished:    10,
		Notifications:        480,
		RedisSeats:           0,
	}
	if err := snapshot.validate(500); err == nil {
		t.Fatal("validate() error = nil, want notification state-not-converged error")
	}
}

func TestSelectionVerificationSnapshotRejectsOverselling(t *testing.T) {
	snapshot := selectionVerificationSnapshot{
		Capacity:             500,
		ClassSelected:        501,
		Applications:         501,
		SelectedApplications: 501,
		DistinctStudents:     501,
		DistinctRequests:     501,
		Enrollments:          501,
		EnrollmentStudents:   501,
		RedisSeats:           -1,
	}

	err := snapshot.fatalViolation(501)
	if err == nil || !strings.Contains(err.Error(), "超卖") {
		t.Fatalf("fatalViolation() error = %v, want overselling error", err)
	}
}

func TestSelectionVerificationSnapshotWaitsForAsyncPersistence(t *testing.T) {
	snapshot := selectionVerificationSnapshot{
		Capacity:             500,
		ClassSelected:        480,
		Applications:         480,
		SelectedApplications: 480,
		DistinctStudents:     480,
		DistinctRequests:     480,
		Enrollments:          480,
		EnrollmentStudents:   480,
		RedisSeats:           0,
		RedisPending:         20,
	}

	if err := snapshot.fatalViolation(500); err != nil {
		t.Fatalf("fatalViolation() error = %v, want pending state", err)
	}
	if err := snapshot.validate(500); err == nil {
		t.Fatal("validate() error = nil, want state-not-converged error")
	}
}

func TestSelectionVerificationSnapshotRejectsDuplicateEnrollment(t *testing.T) {
	snapshot := selectionVerificationSnapshot{
		Capacity:             500,
		ClassSelected:        2,
		Applications:         2,
		SelectedApplications: 2,
		DistinctStudents:     2,
		DistinctRequests:     2,
		Enrollments:          2,
		EnrollmentStudents:   1,
		RedisSeats:           498,
	}

	err := snapshot.fatalViolation(2)
	if err == nil || !strings.Contains(err.Error(), "重复正式选课") {
		t.Fatalf("fatalViolation() error = %v, want duplicate error", err)
	}
}

func TestWaitlistVerificationSnapshotRequiresUniqueWaitingEntries(t *testing.T) {
	snapshot := waitlistVerificationSnapshot{
		Entries:          100,
		WaitingEntries:   100,
		DistinctStudents: 100,
		DistinctRequests: 100,
	}
	if err := snapshot.validate(100); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
	snapshot.DistinctStudents = 99
	if err := snapshot.validate(100); err == nil {
		t.Fatal("validate() error = nil, want duplicate waitlist error")
	}
}
