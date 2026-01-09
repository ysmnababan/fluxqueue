package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	api "fluxqueue/internal/api/http"
	"fluxqueue/internal/config"
	"fluxqueue/internal/logging"
	"fluxqueue/internal/processor"

	"github.com/rs/zerolog"
)

func main() {
	cfg, err := config.LoadConfig([]string{"./configs", "."})
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}
	log.Println("CONFIG: ", *cfg)
	logger := logging.InitLogger(cfg.Server.Env,
		zerolog.InfoLevel,
		cfg.Server.ServiceName,
		cfg.Server.Version)
	mode := flag.String("mode", "worker", "api or worker")
	flag.Parse()
	switch *mode {
	case "api":
		log.Println("START A NEW HTTP SERVER")
		e := api.InitServer()
		api.RegisterRoute(e, *cfg)
		err := e.Start(fmt.Sprintf(":%d", cfg.Server.HTTPPort))
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := e.Shutdown(ctx); err != nil {
			log.Fatalf("shutdown error: %v", err)
		}
	case "worker":
		log.Println("START A NEW TASK PROCESSOR")
		worker, err := processor.NewProcessor(cfg)
		if err != nil {
			logger.Fatal().Err(err).Msg("error starting processor")
		}
		worker.Start(context.Background())

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err = worker.Shutdown(ctx)
		if err != nil {
			logger.Fatal().Err(err).Msg("shutdown error")
		}
	}
	log.Println("shutdown complete")
}
