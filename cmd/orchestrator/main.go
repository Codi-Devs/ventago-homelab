package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Codi-Devs/ventago-homelab/internal/config"
	"github.com/Codi-Devs/ventago-homelab/internal/orchestrator"
)

func main() {
	configPath := flag.String("config", "", "path to config JSON (or CONFIG_PATH)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := orchestrator.New(cfg).Start(ctx); err != nil {
		log.Fatal(err)
	}
}
