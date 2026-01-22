package config

import (
	"encoding/json"
	"log"
	"os"
)

type Configuration struct {
	ApiPort string `json:"api_port"`
	LogPath string `json:"log_path"`

	Database string `json:"database"` // "sqlite3" ou "postgres"
	DbHost   string `json:"db_host"`
	DbPort   string `json:"db_port"`
	DbUser   string `json:"db_user"`
	DbName   string `json:"db_name"`
	DbPass   string `json:"db_pass"`

	Security struct {
		JwtSecret           string `json:"jwt_secret"`
		ActivationCodeLen   int    `json:"activation_code_len"`
		RefreshCodeLen      int    `json:"refresh_code_len"`
		RefreshCodeMaxValid int    `json:"refresh_code_max_valid_days"`
	} `json:"security"`
}

func Get(path string) Configuration {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	var c Configuration
	if err := json.Unmarshal(b, &c); err != nil {
		log.Fatal(err)
	}

	// defaults (pra evitar nil/zero chato)
	if c.ApiPort == "" {
		c.ApiPort = "8080"
	}
	if c.LogPath == "" {
		c.LogPath = "logs/server.log"
	}
	if c.Database == "" {
		c.Database = "sqlite3"
	}
	if c.Security.ActivationCodeLen <= 0 {
		c.Security.ActivationCodeLen = 6
	}
	if c.Security.RefreshCodeLen <= 0 {
		c.Security.RefreshCodeLen = 32
	}
	if c.Security.RefreshCodeMaxValid <= 0 {
		c.Security.RefreshCodeMaxValid = 30
	}
	if c.Security.JwtSecret == "" {
		c.Security.JwtSecret = "CHANGE_ME"
	}

	return c
}
