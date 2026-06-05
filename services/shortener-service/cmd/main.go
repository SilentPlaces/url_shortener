package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpadapter "github.com/arminaray/url_shortener/services/shortener-service/internal/adapters/http"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/infrastructure/config"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/infrastructure/di"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	container, err := di.NewContainer(cfg)
	if err != nil {
		log.Fatalf("failed to build container: %v", err)
	}
	defer func() {
		if err := container.Close(); err != nil {
			log.Printf("failed to close dependencies: %v", err)
		}
	}()

	handler := httpadapter.NewHandler(container.UseCase)
	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           handler.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		log.Printf("shortener-service listening on :%s", cfg.HTTPPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server failed: %v", err)
		}
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	<-stopCh

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	<-done
}
