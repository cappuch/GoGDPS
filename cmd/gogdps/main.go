package main

import (
	"flag"
	"log"

	"gogdps/internal/app"
	"gogdps/internal/config"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("startup: %v", err)
	}
	defer application.Close()

	if err := application.Run(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
