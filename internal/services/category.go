package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appErrors "sun-booking-tours/internal/errors"
	"sun-booking-tours/internal/models"
	"sun-booking-tours/internal/repository"
	"sun-booking-tours/internal/utils"

	"gorm.io/gorm"
)

// CategoryForm is the input DTO for creating / updating a category.
type CategoryForm struct {
	Name        string `form:"name" binding:"required,max=255"`
	Description string `form:"description"`
	ParentID    uint   `form:"parent_id"`
}

// CategoryService handles category business logic.
type CategoryService struct {
	repo repository.CategoryRepo
}

func NewCategoryService(repo repository.CategoryRepo) *CategoryService {
	return &CategoryService{repo: repo}
}

// CategoryTree represents a root category with its children for display.
type CategoryTree struct {
	models.Category
	Children []models.Category
}

// ListCategories returns root categories with children preloaded (tree structure).
func (s *CategoryService) ListCategories(ctx context.Context) ([]CategoryTree, error) {
	all, err := s.repo.FindAllParents(ctx)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}

	trees := make([]CategoryTree, 0, len(all))
	for _, cat := range all {
		trees = append(trees, CategoryTree{
			Category: cat,
			Children: cat.Children,
		})
	}
	return trees, nil
}

// AllFlatCategories returns all categories (flat list) for dropdowns.
func (s *CategoryService) AllFlatCategories(ctx context.Context) ([]models.Category, error) {
	cats, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("all flat categories: %w", err)
	}
	return cats, nil
}

// GetCategory returns a single category by ID.
func (s *CategoryService) GetCategory(ctx context.Context, id uint) (*models.Category, error) {
	cat, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErrors.ErrCategoryNotFound
		}
		return nil, fmt.Errorf("get category: %w", err)
	}
	return cat, nil
}

// CreateCategory validates and persists a new category.
func (s *CategoryService) CreateCategory(ctx context.Context, form *CategoryForm) error {
	name := strings.TrimSpace(form.Name)
	if name == "" {
		return appErrors.NewAppError(400, "Tên danh mục là bắt buộc.")
	}

	slug := utils.Slugify(name)

	exists, err := s.repo.ExistsBySlug(ctx, slug)
	if err != nil {
		return fmt.Errorf("create category check slug: %w", err)
	}
	if exists {
		return appErrors.NewAppError(409, "Danh mục với tên tương tự đã tồn tại.")
	}

	// Validate parent exists if set
	var parentID *uint
	if form.ParentID > 0 {
		parent, pErr := s.repo.FindByID(ctx, form.ParentID)
		if pErr != nil {
			return appErrors.NewAppError(400, "Danh mục cha không tồn tại.")
		}
		// Only allow 2-level hierarchy (parent must be root)
		if parent.ParentID != nil {
			return appErrors.NewAppError(400, "Chỉ hỗ trợ danh mục con cấp 2 (danh mục cha phải là cấp gốc).")
		}
		parentID = &form.ParentID
	}

	cat := models.Category{
		Name:        name,
		Slug:        slug,
		Description: strings.TrimSpace(form.Description),
		ParentID:    parentID,
	}

	if err := s.repo.Create(ctx, &cat); err != nil {
		return fmt.Errorf("create category: %w", err)
	}
	return nil
}

// UpdateCategory validates and updates an existing category.
func (s *CategoryService) UpdateCategory(ctx context.Context, id uint, form *CategoryForm) error {
	cat, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appErrors.ErrCategoryNotFound
		}
		return fmt.Errorf("update category find: %w", err)
	}

	name := strings.TrimSpace(form.Name)
	if name == "" {
		return appErrors.NewAppError(400, "Tên danh mục là bắt buộc.")
	}

	slug := utils.Slugify(name)

	slugExists, err := s.repo.ExistsBySlugExcluding(ctx, slug, id)
	if err != nil {
		return fmt.Errorf("update category check slug: %w", err)
	}
	if slugExists {
		return appErrors.NewAppError(409, "Danh mục với tên tương tự đã tồn tại.")
	}

	// Validate parent
	var parentID *uint
	if form.ParentID > 0 {
		if form.ParentID == id {
			return appErrors.NewAppError(400, "Danh mục không thể là cha của chính nó.")
		}

		// Check that the new parent is not a descendant (prevent cycles)
		descendants, dErr := s.repo.GetDescendantIDs(ctx, id)
		if dErr != nil {
			return fmt.Errorf("update category get descendants: %w", dErr)
		}
		for _, did := range descendants {
			if did == form.ParentID {
				return appErrors.NewAppError(400, "Không thể chọn danh mục con làm danh mục cha.")
			}
		}

		parent, pErr := s.repo.FindByID(ctx, form.ParentID)
		if pErr != nil {
			return appErrors.NewAppError(400, "Danh mục cha không tồn tại.")
		}
		if parent.ParentID != nil {
			return appErrors.NewAppError(400, "Chỉ hỗ trợ danh mục con cấp 2 (danh mục cha phải là cấp gốc).")
		}
		parentID = &form.ParentID
	}

	cat.Name = name
	cat.Slug = slug
	cat.Description = strings.TrimSpace(form.Description)
	cat.ParentID = parentID

	if err := s.repo.Update(ctx, cat); err != nil {
		return fmt.Errorf("update category: %w", err)
	}
	return nil
}

// DeleteCategory removes a category if it has no children and no associated tours.
func (s *CategoryService) DeleteCategory(ctx context.Context, id uint) error {
	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appErrors.ErrCategoryNotFound
		}
		return fmt.Errorf("delete category find: %w", err)
	}

	hasChildren, err := s.repo.HasChildren(ctx, id)
	if err != nil {
		return fmt.Errorf("delete category check children: %w", err)
	}
	if hasChildren {
		return appErrors.NewAppError(400, "Không thể xóa danh mục có danh mục con. Hãy xóa danh mục con trước.")
	}

	hasTours, err := s.repo.HasTours(ctx, id)
	if err != nil {
		return fmt.Errorf("delete category check tours: %w", err)
	}
	if hasTours {
		return appErrors.ErrCategoryHasTours
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	return nil
}
