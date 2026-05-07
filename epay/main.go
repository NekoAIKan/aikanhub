package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}
	store, err := NewFileOrderStore(cfg.OrderStorePath)
	if err != nil {
		log.Fatalf("open order store: %v", err)
	}
	gateway := NewGateway(cfg, store, log.Default())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go RunRetryWorker(ctx, gateway, cfg.CallbackRetryInterval, defaultMaxCallbackAttempts)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           gateway.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("EPay-Alipay gateway listening on %s", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}
