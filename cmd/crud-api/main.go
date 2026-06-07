// Command crud-api runs the Tasks REST service and emits the application log
// stream consumed by the Vector pipeline.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vakharwalad23/logpipe/internal/crud"
	"github.com/vakharwalad23/logpipe/internal/logging"
)

func main() {
	logger, closer, err := logging.NewAppLogger()
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	defer closer.Close()

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	svc := crud.New(logger)
	srv := &http.Server{
		Addr:    addr,
		Handler: svc.Routes(),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("crud-api started", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err.Error())
			stop()
		}
	}()

	<-ctx.Done()
	stop()
	logger.Info("crud-api shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err.Error())
	}
	logger.Info("crud-api stopped")
}
