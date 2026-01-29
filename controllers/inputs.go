package controllers

import (
	"net/http"
	"strconv"

	dbpkg "penelope/db"
	"penelope/models"

	"github.com/gin-gonic/gin"
)

// GET /api/inputs (admin)
func GetInputs(c *gin.Context) {
	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}
	var inputs []models.Input
	if err := db.Order("id asc").Find(&inputs).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	RespondSuccess(c, gin.H{"inputs": inputs})
}

// GET /api/inputs/:id (admin)
func GetInputByID(c *gin.Context) {
	id, ok := ParamID(c, "id")
	if !ok {
		return
	}
	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}
	var input models.Input
	if err := db.First(&input, id).Error; err != nil {
		RespondError(c, "input não encontrado", http.StatusNotFound)
		return
	}
	RespondSuccess(c, gin.H{"input": input})
}

// POST /api/inputs (admin)
func CreateInput(c *gin.Context) {
	var input models.Input
	if err := c.Bind(&input); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if input.Key == "" {
		RespondError(c, "key é obrigatório", http.StatusBadRequest)
		return
	}
	if input.Type == "" {
		RespondError(c, "type é obrigatório", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}
	if err := db.Create(&input).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	RespondSuccess(c, gin.H{"input": input})
}

// PUT /api/inputs/:id (admin)
func UpdateInput(c *gin.Context) {
	id, ok := ParamID(c, "id")
	if !ok {
		return
	}

	var body models.Input
	if err := c.Bind(&body); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var input models.Input
	if err := db.First(&input, id).Error; err != nil {
		RespondError(c, "input não encontrado", http.StatusNotFound)
		return
	}

	if body.Key != "" {
		input.Key = body.Key
	}
	if body.Type != "" {
		input.Type = body.Type
	}
	input.Description = body.Description

	if err := db.Save(&input).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	RespondSuccess(c, gin.H{"input": input})
}

// DELETE /api/inputs/:id (admin)
func DeleteInput(c *gin.Context) {
	id, ok := ParamID(c, "id")
	if !ok {
		return
	}
	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}
	if err := db.Delete(&models.Input{}, "id = ?", id).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	RespondSuccess(c, gin.H{"status": "deleted"})
}

type ModuleInputPayload struct {
	ModuleID int64 `json:"module_id"`
	InputID  int64 `json:"input_id"`
}

// POST /api/module-inputs (admin)
func AddInputToModule(c *gin.Context) {
	var payload ModuleInputPayload
	if err := c.Bind(&payload); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if payload.ModuleID <= 0 || payload.InputID <= 0 {
		RespondError(c, "module_id e input_id são obrigatórios", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	if err := db.First(&models.Module{}, payload.ModuleID).Error; err != nil {
		RespondError(c, "módulo não encontrado", http.StatusNotFound)
		return
	}
	if err := db.First(&models.Input{}, payload.InputID).Error; err != nil {
		RespondError(c, "input não encontrado", http.StatusNotFound)
		return
	}

	var existing models.ModuleInput
	if err := db.Where("module_id = ? AND input_id = ?", payload.ModuleID, payload.InputID).
		First(&existing).Error; err == nil {
		RespondSuccess(c, gin.H{"status": "already_linked"})
		return
	}

	link := models.ModuleInput{ModuleID: payload.ModuleID, InputID: payload.InputID}
	if err := db.Create(&link).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, gin.H{"status": "linked", "link": link})
}

// DELETE /api/module-inputs (admin)
func RemoveInputFromModule(c *gin.Context) {
	var payload ModuleInputPayload
	if err := c.Bind(&payload); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if payload.ModuleID <= 0 || payload.InputID <= 0 {
		RespondError(c, "module_id e input_id são obrigatórios", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	if err := db.Delete(&models.ModuleInput{}, "module_id = ? AND input_id = ?", payload.ModuleID, payload.InputID).
		Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, gin.H{"status": "unlinked"})
}
func GetInputsForUser(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	moduleIDStr := c.Query("module_id")
	if moduleIDStr == "" {
		RespondError(c, "module_id é obrigatório", http.StatusBadRequest)
		return
	}
	moduleID, err := strconv.ParseInt(moduleIDStr, 10, 64)
	if err != nil || moduleID <= 0 {
		RespondError(c, "module_id inválido", http.StatusBadRequest)
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
		RespondSuccess(c, gin.H{"inputs": []models.Input{}})
		return
	}

	// Confirma que o módulo está habilitado no plano do usuário
	var pm models.PlanModule
	if err := db.Where("plan_id = ? AND module_id = ?", *planID, moduleID).First(&pm).Error; err != nil {
		RespondSuccess(c, gin.H{"inputs": []models.Input{}})
		return
	}

	// Busca todos os inputs do módulo
	var links []models.ModuleInput
	if err := db.Where("module_id = ?", moduleID).Find(&links).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	ids := make([]int64, 0, len(links))
	for _, l := range links {
		ids = append(ids, l.InputID)
	}
	if len(ids) == 0 {
		RespondSuccess(c, gin.H{"inputs": []models.Input{}})
		return
	}

	var inputs []models.Input
	if err := db.Where("id in (?)", ids).Order("id asc").Find(&inputs).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, gin.H{"inputs": inputs})
}
