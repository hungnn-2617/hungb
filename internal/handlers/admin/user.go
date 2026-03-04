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
	"sun-booking-tours/internal/repository"
	"sun-booking-tours/internal/services"

	"github.com/gin-gonic/gin"
)

// UserHandler handles admin user management routes.
type UserHandler struct {
	service *services.AdminUserService
}

func NewUserHandler(service *services.AdminUserService) *UserHandler {
	return &UserHandler{service: service}
}

// List renders the paginated, filterable user list.
func (h *UserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	filter := repository.UserFilter{
		Keyword: c.Query("keyword"),
		Role:    c.Query("role"),
		Status:  c.Query("status"),
		SortBy:  c.DefaultQuery("sort", "created_at"),
		Order:   c.DefaultQuery("order", "desc"),
		Page:    page,
		Limit:   10,
	}

	result, err := h.service.ListUsers(c.Request.Context(), filter)
	if err != nil {
		slog.Error(messages.LogAdminUserListFailed, "error", err)
		c.HTML(http.StatusInternalServerError, "admin/pages/error.html", gin.H{
			"status":  500,
			"message": messages.ErrInternalServer,
		})
		return
	}

	flashSuccess, flashError := middleware.GetFlash(c)

	c.HTML(http.StatusOK, "admin/pages/users_list.html", gin.H{
		"title":       messages.TitleAdminUsers,
		"active_menu": "users",
		"user":        middleware.GetCurrentUser(c),
		"csrf_token":  middleware.CSRFToken(c),

		"flash_success": flashSuccess,
		"flash_error":   flashError,

		"users":       result.Users,
		"total":       result.Total,
		"page":        result.Page,
		"limit":       result.Limit,
		"total_pages": result.TotalPages,

		"filter_keyword": filter.Keyword,
		"filter_role":    filter.Role,
		"filter_status":  filter.Status,
		"sort":           filter.SortBy,
		"order":          filter.Order,
	})
}

// Detail renders a single user's detail page with bookings and reviews.
func (h *UserHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "admin/pages/error.html", gin.H{
			"status":  400,
			"message": messages.ErrInvalidForm,
		})
		return
	}

	targetUser, err := h.service.GetUserDetail(c.Request.Context(), uint(id))
	if err != nil {
		c.HTML(http.StatusNotFound, "admin/pages/error.html", gin.H{
			"status":  404,
			"message": messages.ErrAdminUserNotFound,
		})
		return
	}

	flashSuccess, flashError := middleware.GetFlash(c)

	c.HTML(http.StatusOK, "admin/pages/user_detail.html", gin.H{
		"title":       fmt.Sprintf("%s — %s", messages.TitleAdminUserDetail, targetUser.FullName),
		"active_menu": "users",
		"user":        middleware.GetCurrentUser(c),
		"csrf_token":  middleware.CSRFToken(c),

		"flash_success": flashSuccess,
		"flash_error":   flashError,

		"target_user": targetUser,
	})
}

// UpdateStatus changes a user's status (active/inactive/banned).
func (h *UserHandler) UpdateStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "admin/pages/error.html", gin.H{
			"status":  400,
			"message": messages.ErrInvalidForm,
		})
		return
	}

	newStatus := c.PostForm("status")
	currentUser := middleware.GetCurrentUser(c)

	err = h.service.UpdateUserStatus(c.Request.Context(), currentUser.ID, uint(id), newStatus)
	if err != nil {
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) {
			middleware.SetFlashError(c, appErr.Message)
		} else {
			middleware.SetFlashError(c, messages.ErrAdminUserStatusFail)
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/users/%d", id))
		return
	}

	middleware.SetFlashSuccess(c, messages.MsgAdminUserStatusUpdated)
	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/users/%d", id))
}
