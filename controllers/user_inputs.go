package controllers

import (
	"net/http"

	dbpkg "penelope/db"
	"penelope/models"
	"penelope/tools"

	"github.com/gin-gonic/gin"
)

type UserInputRequest struct {
	InputID int64  `json:"input_id" form:"input_id"`
	Content string `json:"content" form:"content"`
}

// GET /api/user-inputs (validated)
func GetUserInputs(c *gin.Context) {
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

	var items []models.UserInput
	if err := db.Where("user_id = ?", user.ID).Order("id asc").Find(&items).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	RespondSuccess(c, gin.H{"user_inputs": items})
}

// GET /api/user-inputs/:id (validated)
func GetUserInputByID(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, ok := ParamID(c, "id")
	if !ok {
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var item models.UserInput
	if err := db.First(&item, id).Error; err != nil {
		RespondError(c, "user_input não encontrado", http.StatusNotFound)
		return
	}
	if item.UserID != user.ID {
		RespondError(c, "forbidden", http.StatusForbidden)
		return
	}

	RespondSuccess(c, gin.H{"user_input": item})
}

// POST /api/user-inputs (validated)
// Cria um user_input para o usuário autenticado.
// Se o usuário estiver em um plano, valida se o input está habilitado no plano.
func CreateUserInput(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req UserInputRequest
	if err := c.Bind(&req); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if req.InputID <= 0 {
		RespondError(c, "input_id é obrigatório", http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		RespondError(c, "content é obrigatório", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	// Verifica se o input existe
	if err := db.First(&models.Input{}, req.InputID).Error; err != nil {
		RespondError(c, "input não encontrado", http.StatusNotFound)
		return
	}

	// Se o usuário tiver plano, valida se esse input é permitido no plano
	planID, err := getUserPlanID(db, user.ID)
	if err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if planID != nil && *planID > 0 {
		// join: module_inputs -> plan_modules
		tmp := models.ModuleInput{}
		q := db.Table("module_inputs").
			Select("module_inputs.*").
			Joins("join plan_modules on plan_modules.module_id = module_inputs.module_id").
			Where("plan_modules.plan_id = ? AND module_inputs.input_id = ?", *planID, req.InputID)

		if err := q.First(&tmp).Error; err != nil {
			RespondError(c, "input não habilitado no seu plano", http.StatusForbidden)
			return
		}
	}

	// Garante unicidade por (user_id, input_id)
	var existing models.UserInput
	if err := db.Where("user_id = ? AND input_id = ?", user.ID, req.InputID).First(&existing).Error; err == nil {
		RespondError(c, "já existe um user_input para este input", http.StatusConflict)
		return
	}

	embedding, err := tools.EmbedText(c.Request.Context(), req.Content)
	if err != nil {
		RespondError(c, "falha ao gerar embedding: "+err.Error(), http.StatusBadRequest)
		return
	}

	item := models.UserInput{
		UserID:    user.ID,
		InputID:   req.InputID,
		Content:   req.Content,
		Embedding: embedding,
	}

	if err := db.Create(&item).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, gin.H{"user_input": item})
}

// PUT /api/user-inputs/:id (validated)
func UpdateUserInput(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, ok := ParamID(c, "id")
	if !ok {
		return
	}

	var req UserInputRequest
	if err := c.Bind(&req); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		RespondError(c, "content é obrigatório", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var item models.UserInput
	if err := db.First(&item, id).Error; err != nil {
		RespondError(c, "user_input não encontrado", http.StatusNotFound)
		return
	}
	if item.UserID != user.ID {
		RespondError(c, "forbidden", http.StatusForbidden)
		return
	}

	embedding, err := tools.EmbedText(c.Request.Context(), req.Content)
	if err != nil {
		RespondError(c, "falha ao gerar embedding: "+err.Error(), http.StatusBadRequest)
		return
	}

	item.Content = req.Content
	item.Embedding = embedding

	if err := db.Save(&item).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, gin.H{"user_input": item})
}

// DELETE /api/user-inputs/:id (validated)
func DeleteUserInput(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, ok := ParamID(c, "id")
	if !ok {
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var item models.UserInput
	if err := db.First(&item, id).Error; err != nil {
		RespondError(c, "user_input não encontrado", http.StatusNotFound)
		return
	}
	if item.UserID != user.ID {
		RespondError(c, "forbidden", http.StatusForbidden)
		return
	}

	if err := db.Delete(&models.UserInput{}, "id = ?", id).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, gin.H{"status": "deleted"})
}
