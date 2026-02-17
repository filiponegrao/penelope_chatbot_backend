package controllers

import (
	"net/http"
	"strings"

	dbpkg "penelope/db"
	"penelope/models"

	"github.com/gin-gonic/gin"
)

// UpdateCurrentUser updates the logged user ("me").
// Route: PUT /api/user
//
// Forbidden fields: id, email, password, admin, created_at, updated_at.
// All other fields are allowed to be updated.
func UpdateCurrentUser(c *gin.Context) {
	logged, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	// Bind to a generic map so we can ignore forbidden keys safely.
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	// Remove forbidden fields (case-insensitive).
	forbidden := map[string]struct{}{
		"id":         {},
		"email":      {},
		"password":   {},
		"admin":      {},
		"created_at": {},
		"updated_at": {},
	}
	for k := range payload {
		if _, isForbidden := forbidden[strings.ToLower(k)]; isForbidden {
			delete(payload, k)
		}
	}

	// If nothing left to update, just return current user.
	if len(payload) == 0 {
		u := logged
		u.Password = ""
		RespondSuccess(c, u)
		return
	}

	// Apply updates.
	if err := db.Model(&models.User{}).
		Where("id = ?", logged.ID).
		Updates(payload).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	// Fetch fresh user.
	var updated models.User
	if err := db.Where("id = ?", logged.ID).First(&updated).Error; err != nil {
		RespondError(c, err.Error(), http.StatusInternalServerError)
		return
	}
	updated.Password = ""
	RespondSuccess(c, updated)
}
