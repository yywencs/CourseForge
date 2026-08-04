//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	authinfra "github.com/yywencs/courseforge/internal/identity/infrastructure/mysql"
	infraquery "github.com/yywencs/courseforge/internal/identity/infrastructure/query"
)

func TestAuthenticationRepositoryLoadsStudentAndCurrentRound(t *testing.T) {
	const (
		studentID   = uint64(9_200_000_000_001)
		accountID   = uint64(9_200_000_000_005)
		studentNo   = "integration-login-001"
		termID      = uint64(9_200_000_000_002)
		roundID     = uint64(9_200_000_000_003)
		roundCode   = "integration-login-round"
		password    = "integration-bcrypt-hash"
		studentName = "集成测试学生"
	)
	now := time.Now().Truncate(time.Millisecond)
	database := integrationCourseForgeDB
	if err := database.Exec(
		`INSERT INTO user_account
			(id, password_hash, state)
		 VALUES (?, ?, 'enabled')
		 ON DUPLICATE KEY UPDATE
			password_hash = VALUES(password_hash),
			state = 'enabled'`,
		accountID,
		password,
	).Error; err != nil {
		t.Fatalf("seed user account: %v", err)
	}
	if err := database.Exec(
		`INSERT INTO student
			(id, account_id, student_no, student_name, major_id, grade_year, state)
		 VALUES (?, ?, ?, ?, ?, ?, 'active')
		 ON DUPLICATE KEY UPDATE
			account_id = VALUES(account_id),
			student_no = VALUES(student_no),
			student_name = VALUES(student_name),
			state = 'active'`,
		studentID,
		accountID,
		studentNo,
		studentName,
		uint64(9_200_000_000_004),
		now.Year(),
	).Error; err != nil {
		t.Fatalf("seed student account: %v", err)
	}
	if err := database.Exec(
		`INSERT INTO selection_round
			(id, term_id, round_code, round_name, start_time, end_time, state)
		 VALUES (?, ?, ?, '集成测试登录轮次', ?, ?, 'open')
		 ON DUPLICATE KEY UPDATE
			start_time = VALUES(start_time),
			end_time = VALUES(end_time),
			state = 'open'`,
		roundID,
		termID,
		roundCode,
		now.Add(-time.Minute),
		now.Add(time.Hour),
	).Error; err != nil {
		t.Fatalf("seed selection round: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Exec("DELETE FROM selection_round WHERE id = ?", roundID).Error
		_ = database.Exec("DELETE FROM student WHERE id = ?", studentID).Error
		_ = database.Exec("DELETE FROM user_account WHERE id = ?", accountID).Error
	})

	accountRepository := authinfra.NewAccountRepository(database)
	account, err := accountRepository.FindStudentByNumber(context.Background(), studentNo)
	if err != nil {
		t.Fatalf("FindStudentByNumber() error = %v", err)
	}
	if account.ID != studentID ||
		account.AccountID != accountID ||
		account.StudentName != studentName ||
		account.PasswordHash != password {
		t.Fatalf("account = %#v", account)
	}
	selectionContext, err := infraquery.NewSelectionContextQuery(database).
		FindCurrentSelectionContext(context.Background())
	if err != nil {
		t.Fatalf("FindCurrentSelectionContext() error = %v", err)
	}
	if selectionContext == nil ||
		selectionContext.TermID != termID ||
		selectionContext.RoundID != roundID {
		t.Fatalf("selection context = %#v", selectionContext)
	}
}

func TestAuthenticationRepositoryLoadsAdministrator(t *testing.T) {
	const (
		administratorID = uint64(9_300_000_000_001)
		accountID       = uint64(9_300_000_000_002)
		username        = "integration-admin-001"
		password        = "integration-admin-bcrypt-hash"
	)
	database := integrationCourseForgeDB
	if err := database.Exec(
		`INSERT INTO user_account
			(id, password_hash, state)
		 VALUES (?, ?, 'enabled')
		 ON DUPLICATE KEY UPDATE
			password_hash = VALUES(password_hash),
			state = 'enabled'`,
		accountID,
		password,
	).Error; err != nil {
		t.Fatalf("seed administrator user account: %v", err)
	}
	if err := database.Exec(
		`INSERT INTO administrator (id, account_id, username)
		 VALUES (?, ?, ?)
		 ON DUPLICATE KEY UPDATE
			account_id = VALUES(account_id),
			username = VALUES(username)`,
		administratorID,
		accountID,
		username,
	).Error; err != nil {
		t.Fatalf("seed administrator: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Exec("DELETE FROM administrator WHERE id = ?", administratorID).Error
		_ = database.Exec("DELETE FROM user_account WHERE id = ?", accountID).Error
	})

	repository := authinfra.NewAccountRepository(database)
	account, err := repository.FindAdministratorByUsername(context.Background(), username)
	if err != nil {
		t.Fatalf("FindAdministratorByUsername() error = %v", err)
	}
	if account.ID != administratorID ||
		account.AccountID != accountID ||
		account.Username != username ||
		account.PasswordHash != password {
		t.Fatalf("account = %#v", account)
	}

	accountByID, err := repository.FindAdministratorByID(context.Background(), administratorID)
	if err != nil {
		t.Fatalf("FindAdministratorByID() error = %v", err)
	}
	if accountByID.Username != username {
		t.Fatalf("account by ID = %#v", accountByID)
	}
}
