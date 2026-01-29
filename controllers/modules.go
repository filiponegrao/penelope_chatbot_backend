package controllers

import (
	"net/http"

	dbpkg "penelope/db"
	"penelope/models"

	"github.com/gin-gonic/gin"
)

type PlanModulePayload struct {
	PlanID   int64 `json:"plan_id"`
	ModuleID int64 `json:"module_id"`
}

// GET /api/modules (admin)
func GetModules(c *gin.Context) {
	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}
	var modules []models.Module
	if err := db.Order("id asc").Find(&modules).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	RespondSuccess(c, gin.H{"modules": modules})
}

// GET /api/modules/:id (admin)
func GetModuleByID(c *gin.Context) {
	id, ok := ParamID(c, "id")
	if !ok {
		return
	}
	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}
	var module models.Module
	if err := db.First(&module, id).Error; err != nil {
		RespondError(c, "módulo não encontrado", http.StatusNotFound)
		return
	}
	RespondSuccess(c, gin.H{"module": module})
}

// POST /api/modules (admin)
func CreateModule(c *gin.Context) {
	var module models.Module
	if err := c.Bind(&module); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if module.Key == "" {
		RespondError(c, "key é obrigatório", http.StatusBadRequest)
		return
	}
	if module.Name == "" {
		RespondError(c, "name é obrigatório", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}
	if err := db.Create(&module).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	RespondSuccess(c, gin.H{"module": module})
}

// PUT /api/modules/:id (admin)
func UpdateModule(c *gin.Context) {
	id, ok := ParamID(c, "id")
	if !ok {
		return
	}

	var body models.Module
	if err := c.Bind(&body); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var module models.Module
	if err := db.First(&module, id).Error; err != nil {
		RespondError(c, "módulo não encontrado", http.StatusNotFound)
		return
	}

	if body.Key != "" {
		module.Key = body.Key
	}
	if body.Name != "" {
		module.Name = body.Name
	}
	module.Description = body.Description

	if err := db.Save(&module).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	RespondSuccess(c, gin.H{"module": module})
}

// DELETE /api/modules/:id (admin)
func DeleteModule(c *gin.Context) {
	id, ok := ParamID(c, "id")
	if !ok {
		return
	}
	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}
	if err := db.Delete(&models.Module{}, "id = ?", id).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	RespondSuccess(c, gin.H{"status": "deleted"})
}

func AddModuleToPlan(c *gin.Context) {
	var payload PlanModulePayload
	if err := c.Bind(&payload); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	if payload.PlanID <= 0 || payload.ModuleID <= 0 {
		RespondError(c, "plan_id e module_id são obrigatórios", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	if err := db.First(&models.Plan{}, payload.PlanID).Error; err != nil {
		RespondError(c, "plano não encontrado", http.StatusNotFound)
		return
	}

	if err := db.First(&models.Module{}, payload.ModuleID).Error; err != nil {
		RespondError(c, "módulo não encontrado", http.StatusNotFound)
		return
	}

	var existing models.PlanModule
	if err := db.
		Where("plan_id = ? AND module_id = ?", payload.PlanID, payload.ModuleID).
		First(&existing).Error; err == nil {
		RespondSuccess(c, gin.H{"status": "already_linked"})
		return
	}

	link := models.PlanModule{
		PlanID:   payload.PlanID,
		ModuleID: payload.ModuleID,
	}

	if err := db.Create(&link).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, gin.H{"status": "linked", "link": link})
}

func RemoveModuleFromPlan(c *gin.Context) {
	var payload PlanModulePayload
	if err := c.Bind(&payload); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	if payload.PlanID <= 0 || payload.ModuleID <= 0 {
		RespondError(c, "plan_id e module_id são obrigatórios", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	if err := db.
		Delete(&models.PlanModule{}, "plan_id = ? AND module_id = ?", payload.PlanID, payload.ModuleID).
		Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, gin.H{"status": "unlinked"})
}

// GET /api/modules/user (validated)
func GetModulesForUser(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	planID, err := getUserPlanID(db, user.ID)
	if err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if planID == nil || *planID <= 0 {
		RespondSuccess(c, gin.H{"modules": []models.Module{}})
		return
	}

	var links []models.PlanModule
	if err := db.Where("plan_id = ?", *planID).Find(&links).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	var modules []models.Module
	for _, l := range links {

		var module models.Module
		if err := db.First(&module, l.ModuleID).Error; err != nil {
			RespondError(c, err.Error(), http.StatusBadRequest)
			return
		}
		modules = append(modules, module)
	}
	RespondSuccess(c, gin.H{"modules": modules})
}

// GET /api/plans/:id/modules (admin)
func GetModulesByPlanID(c *gin.Context) {
	planID, ok := ParamID(c, "id")
	if !ok {
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	// garante que o plano existe (mensagem mais clara)
	if err := db.First(&models.Plan{}, planID).Error; err != nil {
		RespondError(c, "plano não encontrado", http.StatusNotFound)
		return
	}

	var links []models.PlanModule
	if err := db.Where("plan_id = ?", planID).Find(&links).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	ids := make([]int64, 0, len(links))
	for _, l := range links {
		ids = append(ids, l.ModuleID)
	}
	if len(ids) == 0 {
		RespondSuccess(c, gin.H{"modules": []models.Module{}})
		return
	}

	var modules []models.Module
	if err := db.Where("id in (?)", ids).Order("id asc").Find(&modules).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, gin.H{"modules": modules})
}
