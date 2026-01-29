package controllers

import (
	"net/http"

	dbpkg "penelope/db"
	"penelope/models"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
)

type PurchasePlanRequest struct {
	PlanID int64 `json:"plan_id" form:"plan_id"`
}

// GET /api/plans
func GetPlans(c *gin.Context) {
	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var plans []models.Plan
	if err := db.Order("id asc").Find(&plans).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, gin.H{"plans": plans})
}

// GET /api/plans/:id
func GetPlanByID(c *gin.Context) {
	id, ok := ParamID(c, "id")
	if !ok {
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var plan models.Plan
	if err := db.First(&plan, id).Error; err != nil {
		RespondError(c, "plano não encontrado", http.StatusNotFound)
		return
	}

	RespondSuccess(c, gin.H{"plan": plan})
}

// POST /api/plans (admin)
func CreatePlan(c *gin.Context) {
	var plan models.Plan
	if err := c.Bind(&plan); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if plan.Name == "" {
		RespondError(c, "name é obrigatório", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	if err := db.Create(&plan).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, gin.H{"plan": plan})
}

// PUT /api/plans/:id (admin)
func UpdatePlan(c *gin.Context) {
	id, ok := ParamID(c, "id")
	if !ok {
		return
	}

	var body models.Plan
	if err := c.Bind(&body); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var plan models.Plan
	if err := db.First(&plan, id).Error; err != nil {
		RespondError(c, "plano não encontrado", http.StatusNotFound)
		return
	}

	if body.Name != "" {
		plan.Name = body.Name
	}
	plan.Description = body.Description
	if body.PriceCents >= 0 {
		plan.PriceCents = body.PriceCents
	}
	if body.MonthlyMessageLimit >= 0 {
		plan.MonthlyMessageLimit = body.MonthlyMessageLimit
	}
	if body.Currency != "" {
		plan.Currency = body.Currency
	}
	if body.Interval != "" {
		plan.Interval = body.Interval
	}
	plan.IsActive = body.IsActive

	if err := db.Save(&plan).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, gin.H{"plan": plan})
}

// DELETE /api/plans/:id (admin)
func DeletePlan(c *gin.Context) {
	id, ok := ParamID(c, "id")
	if !ok {
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	if err := db.Delete(&models.Plan{}, "id = ?", id).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	RespondSuccess(c, gin.H{"status": "deleted"})
}

// POST /api/plans/purchase (validated)
func PurchasePlan(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req PurchasePlanRequest
	if err := c.Bind(&req); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if req.PlanID <= 0 {
		RespondError(c, "plan_id é obrigatório", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var plan models.Plan
	if err := db.First(&plan, req.PlanID).Error; err != nil {
		RespondError(c, "plano não encontrado", http.StatusNotFound)
		return
	}

	var link models.UserPlan
	err := db.Where("user_id = ?", user.ID).First(&link).Error
	if err != nil {
		tx := db.Begin()

		if gorm.IsRecordNotFoundError(err) {
			// não tinha vínculo -> cria
			link = models.UserPlan{UserID: user.ID, PlanID: req.PlanID}
			if err := tx.Create(&link).Error; err != nil {
				tx.Rollback()
				RespondError(c, err.Error(), http.StatusBadRequest)
				return
			}
		} else {
			tx.Rollback()
			RespondError(c, err.Error(), http.StatusBadRequest)
			return
		}
		if err := tx.Commit().Error; err != nil {
			tx.Rollback()
			RespondError(c, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		RespondError(c, "Usuario ja vinculado a um plano", http.StatusBadRequest)
		return
	}

	RespondSuccess(c, true)
}

// POST /api/plans/cancel (validated)
// Cancela a assinatura de um plano específico do usuário autenticado.
// Body: { "plan_id": 123 }
// Retorna apenas true.
func CancelPlan(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req PurchasePlanRequest // reaproveita { plan_id }
	if err := c.Bind(&req); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if req.PlanID <= 0 {
		RespondError(c, "plan_id é obrigatório", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	// valida que o plano existe
	if err := db.First(&models.Plan{}, req.PlanID).Error; err != nil {
		RespondError(c, "plano não encontrado", http.StatusNotFound)
		return
	}

	// valida que o plano existe
	if err := db.First(&models.UserPlan{}, "user_id = ? AND plan_id = ?", user.ID, req.PlanID).Error; err != nil {
		RespondError(c, "usuário não possui este plano", http.StatusNotFound)
		return
	}

	// Deleta SOMENTE o vínculo daquele plano para aquele usuário.
	// Assim você fica pronto para permitir múltiplos planos no futuro.
	if err := db.Delete(&models.UserPlan{}, "user_id = ? AND plan_id = ?", user.ID, req.PlanID).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, true)
}

func GetUserPlans(c *gin.Context) {
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

	planLinks, err := getUserPlans(db, user.ID)
	if err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if planLinks == nil || len(planLinks) == 0 {
		RespondSuccess(c, gin.H{"plan": nil})
		return
	}

	var plans []models.Plan
	for _, link := range planLinks {
		var plan models.Plan
		if err := db.First(&plan, link.PlanID).Error; err != nil {
			// vínculo inconsistente -> tratamos como sem plano
			RespondSuccess(c, gin.H{"plan": nil})
			return
		}
		plans = append(plans, plan)
	}

	RespondSuccess(c, plans)
}
