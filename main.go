package main

import (
	"fluxqueue/internal/config"
	"fmt"
	"log"
)

func main() {
	cfg, err := config.LoadConfig([]string{"./configs", "."})
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	fmt.Println(*cfg)
}
