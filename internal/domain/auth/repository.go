package auth

import "context"

type Repository interface {
	FindStudentByNumber(ctx context.Context, studentNo string) (*StudentAccount, error)
	FindStudentByID(ctx context.Context, studentID uint64) (*StudentAccount, error)
	FindCurrentSelectionContext(ctx context.Context) (*SelectionContext, error)
}
