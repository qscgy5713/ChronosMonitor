package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"chronosmonitor/internal/broker"
	"chronosmonitor/internal/config"
	"chronosmonitor/internal/db"
	"chronosmonitor/internal/handlers"
	"chronosmonitor/internal/notifier"
	"chronosmonitor/internal/router"
	"chronosmonitor/internal/store"
	"chronosmonitor/internal/sweeper"
	"chronosmonitor/internal/webui"
)

func main() {
	cfg := config.Load()

	conn, err := db.Open(cfg)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer conn.Close()

	taskStore := store.NewTaskStore(conn)
	hub := broker.New()
	taskHandler := handlers.NewTaskHandler(taskStore, hub)
	streamHandler := handlers.NewStreamHandler(hub)
	r := router.New(taskHandler, streamHandler, webui.Assets())

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ttlSweeper := sweeper.New(taskStore, hub, cfg.TTLSweepInterval)
	go ttlSweeper.Run(ctx)

	if cfg.AlertWebhookURL != "" {
		n := notifier.NewWebhookNotifier(cfg.AlertWebhookURL, notifier.Format(cfg.AlertWebhookFormat))
		dispatcher := notifier.NewDispatcher(hub, n, cfg.AlertRateLimitPerMinute, time.Minute)
		go dispatcher.Run(ctx)
		log.Printf("alerting enabled: %s webhook, rate limit %d/min", cfg.AlertWebhookFormat, cfg.AlertRateLimitPerMinute)
	}

	go func() {
		log.Printf("ChronosMonitor server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("server stopped")
}
