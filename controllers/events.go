package controllers

import (
	"net/http"

	dbpkg "penelope/db"
	"penelope/models"

	"github.com/gin-gonic/gin"
)

// GET /api/events (admin)
func GetEvents(c *gin.Context) {
	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var events []models.Event
	if err := db.Order("id desc").Limit(200).Find(&events).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, gin.H{"events": events})
}

// GET /api/events/:id (admin)
func GetEventByID(c *gin.Context) {
	id, ok := ParamID(c, "id")
	if !ok {
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var event models.Event
	if err := db.First(&event, id).Error; err != nil {
		RespondError(c, "event não encontrado", http.StatusNotFound)
		return
	}

	RespondSuccess(c, gin.H{"event": event})
}
