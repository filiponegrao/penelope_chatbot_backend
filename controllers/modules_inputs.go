package controllers

import (
	"net/http"
	"strconv"

	dbpkg "penelope/db"
	"penelope/models"

	"github.com/gin-gonic/gin"
)

// GET /api/modules/input (admin)
// Filtros opcionais:
// ?module_id=1
// ?key=triage
func GetModulesInput(c *gin.Context) {
	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	// optional filters
	var (
		moduleID    int64
		hasModuleID bool
		key         string
	)

	// filtro por id
	if v := c.Query("module_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			RespondError(c, "module_id inválido", http.StatusBadRequest)
			return
		}
		moduleID = id
		hasModuleID = true
	}

	// filtro por key
	key = c.Query("key")

	// Fetch modules
	var modules []models.Module
	q := db.Order("id asc")

	if key != "" {
		q = q.Where("key = ?", key)
	} else if hasModuleID {
		q = q.Where("id = ?", moduleID)
	}

	if err := q.Find(&modules).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	if len(modules) == 0 {
		RespondSuccess(c, []any{})
		return
	}

	// Fetch inputs linked to modules
	type row struct {
		ModuleID int64 `gorm:"column:module_id"`
		models.Input
	}

	var rows []row

	join := db.Table("module_inputs").
		Select(`
			module_inputs.module_id,
			inputs.id,
			inputs.key,
			inputs.type,
			inputs.description,
			inputs.created_at,
			inputs.updated_at
		`).
		Joins("join inputs on inputs.id = module_inputs.input_id").
		Order("module_inputs.module_id asc, inputs.id asc")

	if key != "" {
		join = join.Joins("join modules on modules.id = module_inputs.module_id").
			Where("modules.key = ?", key)
	} else if hasModuleID {
		join = join.Where("module_inputs.module_id = ?", moduleID)
	}

	if err := join.Scan(&rows).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	inputsByModule := map[int64][]models.Input{}
	for _, r := range rows {
		inputsByModule[r.ModuleID] = append(inputsByModule[r.ModuleID], r.Input)
	}

	type ModuleWithInputs struct {
		models.Module
		Inputs []models.Input `json:"inputs"`
	}

	out := make([]ModuleWithInputs, 0, len(modules))

	for _, m := range modules {
		out = append(out, ModuleWithInputs{
			Module: m,
			Inputs: inputsByModule[m.ID],
		})
	}

	RespondSuccess(c, out)
}
