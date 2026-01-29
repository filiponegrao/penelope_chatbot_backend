package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func ParamID(c *gin.Context, name string) (int64, bool) {
	v := c.Param(name)
	if v == "" {
		RespondError(c, name+" é obrigatório", http.StatusBadRequest)
		return 0, false
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		RespondError(c, name+" inválido", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}
