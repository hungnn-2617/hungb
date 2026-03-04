package repository

import (
	"context"
	"fmt"

	"sun-booking-tours/internal/models"

	"gorm.io/gorm"
)

// UserFilter holds query parameters for listing users.
type UserFilter struct {
	Keyword string
	Role    string
	Status  string
	SortBy  string // "created_at" or "email"
	Order   string // "asc" or "desc"
	Page    int
	Limit   int
}

// UserRepo is the data-access contract for user records.
// Depending on an interface (rather than the concrete struct) keeps services
// and handlers decoupled from the GORM implementation and makes unit-testing
// straightforward via simple fakes or mocks.
type UserRepo interface {
	FindByID(ctx context.Context, id uint) (*models.User, error)
	FindByIDWithRelations(ctx context.Context, id uint) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	FindWithFilters(ctx context.Context, filter UserFilter) ([]models.User, error)
	CountWithFilters(ctx context.Context, filter UserFilter) (int64, error)
	Create(ctx context.Context, user *models.User) error
	Update(ctx context.Context, user *models.User) error
}

// userRepository is the GORM-backed implementation of UserRepo.
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository returns a UserRepo backed by the given *gorm.DB.
func NewUserRepository(db *gorm.DB) UserRepo {
	return &userRepository{db: db}
}

// FindByID returns a user by primary key. Returns gorm.ErrRecordNotFound when missing.
func (r *userRepository) FindByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return &user, nil
}

// FindByEmail looks up a user by their exact (already-normalised) email.
// Emails are stored lower-cased on write, so a plain equality check hits the index.
// Returns gorm.ErrRecordNotFound when missing.
func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).
		Where("email = ?", email).
		First(&user).Error; err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return &user, nil
}

// ExistsByEmail returns true when a user with that email already exists in the DB.
// Assumes the caller has already lower-cased the email (normalised on write).
func (r *userRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.User{}).
		Where("email = ?", email).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check email exists: %w", err)
	}
	return count > 0, nil
}

// Create inserts a new user; the model's ID field is populated by GORM on return.
func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// Update saves all changed fields on the given user record.
func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	if err := r.db.WithContext(ctx).Save(user).Error; err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// applyUserFilters returns a *gorm.DB with Where clauses from the filter.
func (r *userRepository) applyUserFilters(ctx context.Context, f UserFilter) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&models.User{})

	if f.Role != "" {
		q = q.Where("role = ?", f.Role)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Keyword != "" {
		kw := "%" + f.Keyword + "%"
		q = q.Where("email ILIKE ? OR full_name ILIKE ?", kw, kw)
	}
	return q
}

// FindWithFilters returns a page of users matching the given filters.
func (r *userRepository) FindWithFilters(ctx context.Context, f UserFilter) ([]models.User, error) {
	q := r.applyUserFilters(ctx, f)

	// Sorting — whitelist allowed columns
	sortCol := "created_at"
	if f.SortBy == "email" {
		sortCol = "email"
	}
	order := "DESC"
	if f.Order == "asc" {
		order = "ASC"
	}
	q = q.Order(fmt.Sprintf("%s %s", sortCol, order))

	// Pagination
	if f.Limit <= 0 {
		f.Limit = 10
	}
	offset := 0
	if f.Page > 1 {
		offset = (f.Page - 1) * f.Limit
	}

	var users []models.User
	if err := q.Offset(offset).Limit(f.Limit).Find(&users).Error; err != nil {
		return nil, fmt.Errorf("find users with filters: %w", err)
	}
	return users, nil
}

// CountWithFilters returns the total number of users matching filters.
func (r *userRepository) CountWithFilters(ctx context.Context, f UserFilter) (int64, error) {
	var count int64
	if err := r.applyUserFilters(ctx, f).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count users with filters: %w", err)
	}
	return count, nil
}

// FindByIDWithRelations returns a user by ID with recent bookings (with Tour) and reviews preloaded.
func (r *userRepository) FindByIDWithRelations(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).
		Preload("Bookings", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(10).Preload("Tour")
		}).
		Preload("Reviews", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(10)
		}).
		First(&user, id).Error; err != nil {
		return nil, fmt.Errorf("find user by id with relations: %w", err)
	}
	return &user, nil
}
