package services

import (
	"context"
	"fmt"
	"log/slog"
	"math"

	"sun-booking-tours/internal/constants"
	appErrors "sun-booking-tours/internal/errors"
	"sun-booking-tours/internal/messages"
	"sun-booking-tours/internal/models"
	"sun-booking-tours/internal/repository"
)

// UserListResult holds a page of users together with pagination metadata.
type UserListResult struct {
	Users      []models.User
	Total      int64
	Page       int
	Limit      int
	TotalPages int
}

// AdminUserService handles admin-side user management logic.
type AdminUserService struct {
	userRepo repository.UserRepo
}

func NewAdminUserService(userRepo repository.UserRepo) *AdminUserService {
	return &AdminUserService{userRepo: userRepo}
}

// ListUsers returns a paginated, filtered list of users.
func (s *AdminUserService) ListUsers(ctx context.Context, f repository.UserFilter) (*UserListResult, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit <= 0 {
		f.Limit = 10
	}

	total, err := s.userRepo.CountWithFilters(ctx, f)
	if err != nil {
		slog.ErrorContext(ctx, messages.LogAdminUserListFailed, "error", err)
		return nil, appErrors.ErrInternalServerError
	}

	users, err := s.userRepo.FindWithFilters(ctx, f)
	if err != nil {
		slog.ErrorContext(ctx, messages.LogAdminUserListFailed, "error", err)
		return nil, appErrors.ErrInternalServerError
	}

	totalPages := int(math.Ceil(float64(total) / float64(f.Limit)))

	return &UserListResult{
		Users:      users,
		Total:      total,
		Page:       f.Page,
		Limit:      f.Limit,
		TotalPages: totalPages,
	}, nil
}

// GetUserDetail returns a user with their bookings and reviews preloaded.
func (s *AdminUserService) GetUserDetail(ctx context.Context, id uint) (*models.User, error) {
	user, err := s.userRepo.FindByIDWithRelations(ctx, id)
	if err != nil {
		slog.ErrorContext(ctx, messages.LogAdminUserDetailFailed, "user_id", id, "error", err)
		return nil, appErrors.ErrUserNotFound
	}
	return user, nil
}

// UpdateUserStatus changes the target user's status.
// Rules: admin cannot change their own status, cannot change another admin's status.
func (s *AdminUserService) UpdateUserStatus(ctx context.Context, adminID, targetID uint, newStatus string) error {
	if !isValidStatus(newStatus) {
		return fmt.Errorf("%w", appErrors.ErrInvalidInput)
	}

	if adminID == targetID {
		return appErrors.NewAppError(appErrors.ErrForbidden.Status, messages.ErrCannotChangeOwnStatus)
	}

	target, err := s.userRepo.FindByID(ctx, targetID)
	if err != nil {
		slog.ErrorContext(ctx, messages.LogAdminUserStatusFailed, "target_id", targetID, "error", err)
		return appErrors.ErrUserNotFound
	}

	if target.Role == constants.RoleAdmin {
		return appErrors.NewAppError(appErrors.ErrForbidden.Status, messages.ErrCannotChangeAdminStatus)
	}

	target.Status = newStatus
	if err := s.userRepo.Update(ctx, target); err != nil {
		slog.ErrorContext(ctx, messages.LogAdminUserStatusFailed, "target_id", targetID, "error", err)
		return appErrors.ErrInternalServerError
	}

	return nil
}

func isValidStatus(s string) bool {
	return s == constants.StatusActive || s == constants.StatusInactive || s == constants.StatusBanned
}
