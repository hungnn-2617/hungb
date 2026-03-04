package middleware

import (
	"sun-booking-tours/internal/repository"

	"github.com/gin-gonic/gin"
)

// LoadCategories injects the category tree into every public template context
// so the navbar can display a categories dropdown without each handler
// needing to fetch categories individually.
func LoadCategories(catRepo repository.CategoryRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		cats, err := catRepo.FindAllParents(c.Request.Context())
		if err == nil {
			c.Set("nav_categories", cats)
		}
		c.Next()
	}
}

// GetNavCategories retrieves the category list set by LoadCategories.
func GetNavCategories(c *gin.Context) any {
	v, exists := c.Get("nav_categories")
	if !exists {
		return nil
	}
	return v
}
