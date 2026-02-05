package controllers

import (
	"net/http"
	"strings"
	"time"

	dbpkg "penelope/db"
	"penelope/models"
	"penelope/tools"

	"github.com/jinzhu/gorm"
	"github.com/gin-gonic/gin"
)

type upsertWhatsAppConfigReq struct {
	PhoneNumberID string `json:"phone_number_id"`
	AccessToken   string `json:"access_token"`
	ApiVersion    string `json:"api_version"`
}

// PUT /api/whatsapp/config (validated)
// Upsert the tenant WhatsApp credentials.
// Returns only true.
func UpsertWhatsAppConfig(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req upsertWhatsAppConfigReq
	if err := c.Bind(&req); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	req.PhoneNumberID = strings.TrimSpace(req.PhoneNumberID)
	req.AccessToken = strings.TrimSpace(req.AccessToken)
	req.ApiVersion = strings.TrimSpace(req.ApiVersion)
	if req.ApiVersion == "" {
		req.ApiVersion = "v24.0"
	}

	if req.PhoneNumberID == "" {
		RespondError(c, "phone_number_id é obrigatório", http.StatusBadRequest)
		return
	}
	if req.AccessToken == "" {
		RespondError(c, "access_token é obrigatório", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var wa models.WhatsAppConfig
	err := db.Where("user_id = ?", user.ID).First(&wa).Error
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			wa = models.WhatsAppConfig{
				UserID:        user.ID,
				PhoneNumberID: req.PhoneNumberID,
				AccessToken:   req.AccessToken,
				ApiVersion:    req.ApiVersion,
				Status:        models.WHATSAPP_STATUS_PENDING,
			}
			if err := db.Create(&wa).Error; err != nil {
				RespondError(c, err.Error(), http.StatusBadRequest)
				return
			}
			RespondSuccess(c, true)
			return
		}
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	// Update existing config and reset status to pending if phone id changed.
	status := wa.Status
	if strings.TrimSpace(wa.PhoneNumberID) != req.PhoneNumberID {
		status = models.WHATSAPP_STATUS_PENDING
	}

	if err := db.Model(&models.WhatsAppConfig{}).
		Where("id = ?", wa.ID).
		Updates(map[string]any{
			"phone_number_id": req.PhoneNumberID,
			"access_token":    req.AccessToken,
			"api_version":      req.ApiVersion,
			"status":           status,
		}).Error; err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, true)
}

type requestCodeReq struct {
	CodeMethod string `json:"code_method"` // SMS | VOICE
	Language   string `json:"language"`    // pt_BR
}

// POST /api/whatsapp/request-code (validated)
// Requests a verification code to the business phone number.
// Returns only true.
func WhatsAppRequestCode(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req requestCodeReq
	_ = c.Bind(&req) // optional body

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var wa models.WhatsAppConfig
	if err := db.Where("user_id = ?", user.ID).First(&wa).Error; err != nil {
		RespondError(c, "whatsapp config não encontrada", http.StatusNotFound)
		return
	}

	ctx := c.Request.Context()
	client := tools.WhatsAppClient{AccessToken: wa.AccessToken, ApiVersion: wa.ApiVersion, PhoneNumberID: wa.PhoneNumberID}
	if err := client.RequestCode(ctx, strings.TrimSpace(req.CodeMethod), strings.TrimSpace(req.Language)); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	RespondSuccess(c, true)
}

type registerReq struct {
	Pin string `json:"pin"`
}

// POST /api/whatsapp/register (validated)
// Registers the business phone number in Cloud API using the PIN.
// Returns only true.
func WhatsAppRegister(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req registerReq
	if err := c.Bind(&req); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	pin := strings.TrimSpace(req.Pin)
	if pin == "" {
		RespondError(c, "pin é obrigatório", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var wa models.WhatsAppConfig
	if err := db.Where("user_id = ?", user.ID).First(&wa).Error; err != nil {
		RespondError(c, "whatsapp config não encontrada", http.StatusNotFound)
		return
	}

	ctx := c.Request.Context()
	client := tools.WhatsAppClient{AccessToken: wa.AccessToken, ApiVersion: wa.ApiVersion, PhoneNumberID: wa.PhoneNumberID}
	if err := client.Register(ctx, pin); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	// mark as registered
	now := time.Now()
	_ = db.Model(&models.WhatsAppConfig{}).Where("id = ?", wa.ID).Updates(map[string]any{
		"status":      models.WHATSAPP_STATUS_REGISTERED,
		"updated_at":  &now,
	}).Error

	RespondSuccess(c, true)
}
