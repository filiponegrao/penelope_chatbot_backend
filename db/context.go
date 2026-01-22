package db

import (
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
)

const dbKey = "db"

// Use este middleware no setup do gin
func SetDBtoContext(database *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(dbKey, database)
		c.Next()
	}
}

func DBInstance(c *gin.Context) *gorm.DB {
	v, ok := c.Get(dbKey)
	if !ok {
		return nil
	}
	db, _ := v.(*gorm.DB)
	return db
}
