package controllers

import (
	dbpkg "penelope/db"
	"penelope/models"
	"penelope/tools"

	"github.com/gin-gonic/gin"
)

func CheckUserExists(c *gin.Context, email string) (bool, error, *models.User) {
	db := dbpkg.DBInstance(c)
	if db == nil {
		return false, nil, nil
	}

	var user models.User
	if err := db.Where("email = ?", email).First(&user).Error; err != nil {
		return false, nil, nil
	}
	return true, nil, &user
}

func CreateUser(c *gin.Context) {
	db := dbpkg.DBInstance(c)
	user := models.User{}
	if err := c.Bind(&user); err != nil {
		RespondError(c, err.Error(), 400)
		return
	}

	missing := user.MissingFields()
	if missing != "" {
		RespondError(c, "Faltando campo "+missing, 400)
		return
	}

	if !tools.ValidateEmail(user.Email) {
		RespondError(c, "E-mail inválido!", 400)
		return
	}

	exists, err, _ := CheckUserExists(c, user.Email)
	if err != nil {
		RespondError(c, err.Error(), 400)
		return
	} else if exists {
		RespondError(c, "Usuário já existe", 400)
		return
	}

	if user.Password != "" {
		passwordEncode := tools.EncryptTextSHA512(user.Password)
		passwordEncode = user.Email + ":" + passwordEncode
		passwordEncode = tools.EncryptTextSHA512(passwordEncode)
		user.Password = passwordEncode
	}

	user.Admin = false
	user.Type = models.USER_TYPE_NORMAL
	user.Status = models.USER_STATUS_AVAILABLE

	headerVersion := c.Request.Header.Get("Application-Version")
	if headerVersion == "v1" {
		user.Status = models.USER_STATUS_PENDING
	}

	tx := db.Begin()
	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		RespondError(c, err.Error(), 400)
		return
	}

	if headerVersion == "v1" {
		code := tools.RandomString(6)
		_, err := CreateInvite(c, tx, code, user, "")
		if err != nil {
			tx.Rollback()
			RespondError(c, err.Error(), 400)
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		RespondError(c, err.Error(), 400)
		return
	}

	user.Password = ""
	RespondSuccess(c, user)
}
