// Package main is the entrypoint for the Starter Go API server.
//
// @title           Starter Go API
// @version         1.0
// @description     Minimal, production-ready Go API starter built with Fiber, GORM, JWT, and S3-compatible object storage.
// @termsOfService  http://swagger.io/terms/
// @contact.name    API Support
// @host            localhost:4000
// @BasePath        /
// @schemes         http https
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     Type "Bearer {token}" to authenticate.
package main

import (
	"log"

	"github.com/ganiramadhan/starter-go/internal/app"
	"github.com/ganiramadhan/starter-go/internal/config"
)

var version = "dev"

func main() {
	cfg := config.Load()

	log.Printf("app: starting starter-go (version=%s, env=%s)", version, cfg.App.Env)

	a, err := app.New(cfg)
	if err != nil {
		log.Fatalf("app: bootstrap failed: %v", err)
	}
	if err := a.Run(); err != nil {
		log.Fatalf("app: %v", err)
	}
}
