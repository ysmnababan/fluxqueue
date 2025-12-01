package main

import (
	"fluxqueue/api/http"
	"fluxqueue/internal/config"
	"fmt"
	"log"

	"github.com/labstack/echo/v4"
)

func main() {
	cfg, err := config.LoadConfig([]string{"./configs", "."})
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	fmt.Println(*cfg)
	e := echo.New()
	http.InitServer(e)

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", cfg.Server.HTTPPort)))
}
