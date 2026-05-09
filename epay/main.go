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

const (
	defaultReadHeaderTimeout = 5 * time.Second
	defaultReadTimeout       = 10 * time.Second
	defaultWriteTimeout      = 15 * time.Second
	defaultIdleTimeout       = 60 * time.Second
)

func main() {
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}
	store, err := NewOrderStore(cfg)
	if err != nil {
		log.Fatalf("open order store: %v", err)
	}
	if closer, ok := store.(interface{ Close() error }); ok {
		defer func() {
			if err := closer.Close(); err != nil {
				log.Printf("close order store failed: %v", err)
			}
		}()
	}
	gateway, err := NewGateway(cfg, store, log.Default())
	if err != nil {
		log.Fatalf("create gateway: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go RunRetryWorker(ctx, gateway, cfg.CallbackRetryInterval, defaultMaxCallbackAttempts)

	server := newHTTPServer(cfg, gateway.Routes())
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

func newHTTPServer(cfg Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
		MaxHeaderBytes:    16 * 1024,
	}
}
