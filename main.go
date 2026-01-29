package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"penelope/config"
	"penelope/db"
	"penelope/router"
	"penelope/workers"
)

func main() {
	cfg := config.Get(getenv("CONFIG_PATH", "config.json"))
	port := getenv("PORT", cfg.ApiPort)
	if os.Getenv("JWT_SECRET") == "" && cfg.Security.JwtSecret != "" {
		_ = os.Setenv("JWT_SECRET", cfg.Security.JwtSecret)
	}

	// DB
	db.SetConfigurations(cfg)
	database, err := db.Connect()
	if err != nil {
		log.Fatalf("db connect error: %v", err)
	}
	defer database.Close()

	// Workers
	workers.StartEventProcessor(database)

	// Gin
	r := gin.New()
	r.Use(db.SetDBtoContext(database))

	// API routes
	router.Initialize(r, cfg)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("Penelope listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
