package db

import (
	"log"
	"os"
	"path/filepath"

	"penelope/config"
	"penelope/models"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var conf config.Configuration

func SetConfigurations(configuration config.Configuration) {
	conf = configuration
}

// Connect abre conexão com DB (sqlite3 por padrão) e faz automigrate básico.
// Para habilitar automigrate em ambientes de dev, exporte AUTOMIGRATE=1.
func Connect() (*gorm.DB, error) {
	database := conf.Database
	if database == "" {
		database = "sqlite3"
	}

	var (
		db  *gorm.DB
		err error
	)

	if database == "postgres" || database == "postgresql" {
		log.Println("Utilizando conexão com o postgresql...")
		path := "host=" + conf.DbHost + " port=" + conf.DbPort
		path += " user=" + conf.DbUser + " dbname=" + conf.DbName
		path += " password=" + conf.DbPass
		db, err = gorm.Open("postgres", path)
	} else {
		log.Println("Utilizando conexão com o sqlite3...")
		dir := filepath.Dir("db/database.db")
		db, err = gorm.Open("sqlite3", dir+"/database.db")
	}

	if err != nil {
		log.Println("Got error when connect database, the error is: " + err.Error())
		return nil, err
	}

	// Log em dev
	db.LogMode(true)

	if getenv("AUTOMIGRATE", "0") == "1" {
		db.AutoMigrate(
			&models.User{},
			&models.Invite{},
			&models.RefreshToken{},
			&models.PasswordReset{},
			&models.Plan{},
			&models.Module{},
			&models.PlanModule{},
			&models.Input{},
			&models.ModuleInput{},
			&models.UserInput{},
			&models.Event{},
			&models.UserPlan{},
			&models.WhatsAppConfig{},
		)
	}

	return db, nil
}

func getenv(k, def string) string {
	// helper interno para não importar os/os em vários lugares
	// (main.go já tem um getenv, mas aqui evitamos dependência circular)
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
