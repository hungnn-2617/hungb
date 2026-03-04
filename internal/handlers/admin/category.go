package admin

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	appErrors "sun-booking-tours/internal/errors"
	"sun-booking-tours/internal/messages"
	"sun-booking-tours/internal/middleware"
	"sun-booking-tours/internal/services"

	"github.com/gin-gonic/gin"
)

// CategoryHandler handles admin category CRUD routes.
type CategoryHandler struct {
	service *services.CategoryService
}

func NewCategoryHandler(service *services.CategoryService) *CategoryHandler {
	return &CategoryHandler{service: service}
}

// List renders the category tree page.
func (h *CategoryHandler) List(c *gin.Context) {
	trees, err := h.service.ListCategories(c.Request.Context())
	if err != nil {
		slog.Error(messages.LogAdminCategoryListFailed, "error", err)
		c.HTML(http.StatusInternalServerError, "admin/pages/error.html", gin.H{
			"status":  500,
			"message": messages.ErrInternalServer,
		})
		return
	}

	flashSuccess, flashError := middleware.GetFlash(c)

	c.HTML(http.StatusOK, "admin/pages/categories_list.html", gin.H{
		"title":       messages.TitleAdminCategories,
		"active_menu": "categories",
		"user":        middleware.GetCurrentUser(c),
		"csrf_token":  middleware.CSRFToken(c),

		"flash_success": flashSuccess,
		"flash_error":   flashError,

		"categories": trees,
	})
}

// CreateForm renders the "add category" form.
func (h *CategoryHandler) CreateForm(c *gin.Context) {
	parents, err := h.service.AllFlatCategories(c.Request.Context())
	if err != nil {
		slog.Error(messages.LogAdminCategoryListFailed, "error", err)
	}

	// Filter to only root categories for parent dropdown
	var rootCats []struct {
		ID   uint
		Name string
	}
	for _, p := range parents {
		if p.ParentID == nil {
			rootCats = append(rootCats, struct {
				ID   uint
				Name string
			}{p.ID, p.Name})
		}
	}

	flashSuccess, flashError := middleware.GetFlash(c)

	c.HTML(http.StatusOK, "admin/pages/category_form.html", gin.H{
		"title":       messages.TitleAdminCategoryCreate,
		"active_menu": "categories",
		"user":        middleware.GetCurrentUser(c),
		"csrf_token":  middleware.CSRFToken(c),

		"flash_success": flashSuccess,
		"flash_error":   flashError,

		"parents":  rootCats,
		"is_edit":  false,
		"form_url": "/admin/categories/create",
	})
}

// Create processes the new-category form submission.
func (h *CategoryHandler) Create(c *gin.Context) {
	var form services.CategoryForm
	if err := c.ShouldBind(&form); err != nil {
		middleware.SetFlashError(c, messages.ErrInvalidForm)
		c.Redirect(http.StatusFound, "/admin/categories/create")
		return
	}

	if err := h.service.CreateCategory(c.Request.Context(), &form); err != nil {
		slog.Error(messages.LogAdminCategoryCreateFailed, "error", err)
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) {
			middleware.SetFlashError(c, appErr.Message)
		} else {
			middleware.SetFlashError(c, messages.ErrAdminCategoryCreateFail)
		}
		c.Redirect(http.StatusFound, "/admin/categories/create")
		return
	}

	middleware.SetFlashSuccess(c, messages.MsgAdminCategoryCreated)
	c.Redirect(http.StatusFound, "/admin/categories")
}

// EditForm renders the category edit form.
func (h *CategoryHandler) EditForm(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "admin/pages/error.html", gin.H{
			"status":  400,
			"message": messages.ErrInvalidForm,
		})
		return
	}

	cat, err := h.service.GetCategory(c.Request.Context(), uint(id))
	if err != nil {
		c.HTML(http.StatusNotFound, "admin/pages/error.html", gin.H{
			"status":  404,
			"message": messages.ErrAdminCategoryNotFound,
		})
		return
	}

	parents, _ := h.service.AllFlatCategories(c.Request.Context())
	var rootCats []struct {
		ID   uint
		Name string
	}
	for _, p := range parents {
		if p.ParentID == nil && p.ID != uint(id) {
			rootCats = append(rootCats, struct {
				ID   uint
				Name string
			}{p.ID, p.Name})
		}
	}

	flashSuccess, flashError := middleware.GetFlash(c)

	c.HTML(http.StatusOK, "admin/pages/category_form.html", gin.H{
		"title":       messages.TitleAdminCategoryEdit,
		"active_menu": "categories",
		"user":        middleware.GetCurrentUser(c),
		"csrf_token":  middleware.CSRFToken(c),

		"flash_success": flashSuccess,
		"flash_error":   flashError,

		"parents":  rootCats,
		"is_edit":  true,
		"category": cat,
		"form_url": fmt.Sprintf("/admin/categories/%d/edit", id),
	})
}

// Update processes the category edit form submission.
func (h *CategoryHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "admin/pages/error.html", gin.H{
			"status":  400,
			"message": messages.ErrInvalidForm,
		})
		return
	}

	var form services.CategoryForm
	if err := c.ShouldBind(&form); err != nil {
		middleware.SetFlashError(c, messages.ErrInvalidForm)
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/categories/%d/edit", id))
		return
	}

	if err := h.service.UpdateCategory(c.Request.Context(), uint(id), &form); err != nil {
		slog.Error(messages.LogAdminCategoryUpdateFailed, "error", err)
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) {
			middleware.SetFlashError(c, appErr.Message)
		} else {
			middleware.SetFlashError(c, messages.ErrAdminCategoryUpdateFail)
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/categories/%d/edit", id))
		return
	}

	middleware.SetFlashSuccess(c, messages.MsgAdminCategoryUpdated)
	c.Redirect(http.StatusFound, "/admin/categories")
}

// Delete removes a category.
func (h *CategoryHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "admin/pages/error.html", gin.H{
			"status":  400,
			"message": messages.ErrInvalidForm,
		})
		return
	}

	if err := h.service.DeleteCategory(c.Request.Context(), uint(id)); err != nil {
		slog.Error(messages.LogAdminCategoryDeleteFailed, "error", err)
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) {
			middleware.SetFlashError(c, appErr.Message)
		} else {
			middleware.SetFlashError(c, messages.ErrAdminCategoryDeleteFail)
		}
		c.Redirect(http.StatusFound, "/admin/categories")
		return
	}

	middleware.SetFlashSuccess(c, messages.MsgAdminCategoryDeleted)
	c.Redirect(http.StatusFound, "/admin/categories")
}
