package repository

import (
	"context"
	"fmt"

	"sun-booking-tours/internal/models"

	"gorm.io/gorm"
)

// CategoryRepo is the data-access contract for category records.
type CategoryRepo interface {
	FindAll(ctx context.Context) ([]models.Category, error)
	FindAllParents(ctx context.Context) ([]models.Category, error)
	FindByID(ctx context.Context, id uint) (*models.Category, error)
	FindBySlug(ctx context.Context, slug string) (*models.Category, error)
	ExistsBySlug(ctx context.Context, slug string) (bool, error)
	ExistsBySlugExcluding(ctx context.Context, slug string, excludeID uint) (bool, error)
	Create(ctx context.Context, cat *models.Category) error
	Update(ctx context.Context, cat *models.Category) error
	Delete(ctx context.Context, id uint) error
	HasTours(ctx context.Context, id uint) (bool, error)
	HasChildren(ctx context.Context, id uint) (bool, error)
	GetDescendantIDs(ctx context.Context, parentID uint) ([]uint, error)
}

type categoryRepository struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) CategoryRepo {
	return &categoryRepository{db: db}
}

// FindAll returns all categories with their children preloaded, ordered by name.
func (r *categoryRepository) FindAll(ctx context.Context) ([]models.Category, error) {
	var cats []models.Category
	if err := r.db.WithContext(ctx).
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			return db.Order("name ASC")
		}).
		Preload("Parent").
		Order("name ASC").
		Find(&cats).Error; err != nil {
		return nil, fmt.Errorf("find all categories: %w", err)
	}
	return cats, nil
}

// FindAllParents returns only root-level categories (parent_id IS NULL) with children.
func (r *categoryRepository) FindAllParents(ctx context.Context) ([]models.Category, error) {
	var cats []models.Category
	if err := r.db.WithContext(ctx).
		Where("parent_id IS NULL").
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			return db.Order("name ASC")
		}).
		Order("name ASC").
		Find(&cats).Error; err != nil {
		return nil, fmt.Errorf("find parent categories: %w", err)
	}
	return cats, nil
}

// FindByID returns a category by primary key with parent and children preloaded.
func (r *categoryRepository) FindByID(ctx context.Context, id uint) (*models.Category, error) {
	var cat models.Category
	if err := r.db.WithContext(ctx).
		Preload("Parent").
		Preload("Children").
		First(&cat, id).Error; err != nil {
		return nil, fmt.Errorf("find category by id: %w", err)
	}
	return &cat, nil
}

// FindBySlug returns a category by its unique slug.
func (r *categoryRepository) FindBySlug(ctx context.Context, slug string) (*models.Category, error) {
	var cat models.Category
	if err := r.db.WithContext(ctx).
		Where("slug = ?", slug).
		First(&cat).Error; err != nil {
		return nil, fmt.Errorf("find category by slug: %w", err)
	}
	return &cat, nil
}

// ExistsBySlug checks if a category with the given slug exists.
func (r *categoryRepository) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Category{}).
		Where("slug = ?", slug).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check slug exists: %w", err)
	}
	return count > 0, nil
}

// ExistsBySlugExcluding checks if a slug exists for a category other than excludeID.
func (r *categoryRepository) ExistsBySlugExcluding(ctx context.Context, slug string, excludeID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Category{}).
		Where("slug = ? AND id != ?", slug, excludeID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check slug exists excluding: %w", err)
	}
	return count > 0, nil
}

// Create inserts a new category.
func (r *categoryRepository) Create(ctx context.Context, cat *models.Category) error {
	if err := r.db.WithContext(ctx).Create(cat).Error; err != nil {
		return fmt.Errorf("create category: %w", err)
	}
	return nil
}

// Update saves all changed fields on the given category.
func (r *categoryRepository) Update(ctx context.Context, cat *models.Category) error {
	if err := r.db.WithContext(ctx).Save(cat).Error; err != nil {
		return fmt.Errorf("update category: %w", err)
	}
	return nil
}

// Delete removes a category by ID (soft delete via GORM DeletedAt).
func (r *categoryRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&models.Category{}, id).Error; err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	return nil
}

// HasTours returns true if any tour is associated with this category via tour_categories.
func (r *categoryRepository) HasTours(ctx context.Context, id uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Table("tour_categories").
		Where("category_id = ?", id).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check category has tours: %w", err)
	}
	return count > 0, nil
}

// HasChildren returns true if any child category references this one as parent.
func (r *categoryRepository) HasChildren(ctx context.Context, id uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Category{}).
		Where("parent_id = ?", id).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check category has children: %w", err)
	}
	return count > 0, nil
}

// GetDescendantIDs returns all descendant category IDs (children, grandchildren, etc.)
// using a simple recursive approach suitable for shallow trees.
func (r *categoryRepository) GetDescendantIDs(ctx context.Context, parentID uint) ([]uint, error) {
	var ids []uint
	if err := r.collectDescendants(ctx, parentID, &ids); err != nil {
		return nil, fmt.Errorf("get descendant ids: %w", err)
	}
	return ids, nil
}

func (r *categoryRepository) collectDescendants(ctx context.Context, parentID uint, ids *[]uint) error {
	var childIDs []uint
	if err := r.db.WithContext(ctx).Model(&models.Category{}).
		Where("parent_id = ?", parentID).
		Pluck("id", &childIDs).Error; err != nil {
		return err
	}
	for _, cid := range childIDs {
		*ids = append(*ids, cid)
		if err := r.collectDescendants(ctx, cid, ids); err != nil {
			return err
		}
	}
	return nil
}
