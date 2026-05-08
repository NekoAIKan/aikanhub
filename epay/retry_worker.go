package main

import (
	"context"
	"log"
	"time"
)

const defaultMaxCallbackAttempts = 20

func RunRetryWorker(ctx context.Context, gateway *Gateway, interval time.Duration, maxAttempts int) {
	if interval <= 0 {
		interval = defaultCallbackRetryInterval
	}
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxCallbackAttempts
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := gateway.RetryPendingCallbacks(ctx, maxAttempts); err != nil {
				log.Printf("retry callbacks failed: %v", err)
			}
		}
	}
}
