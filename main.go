package main

import (
	"context"
	"errors"
	"fluxqueue/internal/app"
	"fluxqueue/internal/config"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	cfg, err := config.LoadConfig([]string{"./configs", "."})
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}
	app, err := app.NewApp(cfg)
	if err != nil {
		log.Fatalf("new app: %v", err)
	}

	go func() {
		if err := app.Start(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				// expected on graceful shutdown
				log.Printf("server closed")
				return
			}
			// real error
			log.Fatalf("app start failed: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	<-sig
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	log.Println("shutdown complete")
}
