package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/vedhavyas/sezzle-calculator/backend/internal/httpapi"
)

const shutdownTimeout = 10 * time.Second

func main() {
	addr := flag.String("addr", defaultAddr(), "address to listen on")
	webDir := flag.String("web", "", "directory holding the built frontend; empty serves the API only")
	flag.Parse()

	if err := serve(*addr, *webDir); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// serve runs the API until SIGINT or SIGTERM, then drains in-flight requests for
// at most shutdownTimeout.
func serve(addr, webDir string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	server := &http.Server{
		Handler:           httpapi.New(webDir),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	drained := make(chan struct{})
	go func() {
		defer close(drained)
		<-ctx.Done()
		slog.Info("shutting down", "timeout", shutdownTimeout.String())
		drainCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(drainCtx); err != nil {
			slog.Error("shutdown did not finish cleanly", "error", err)
			_ = server.Close()
		}
	}()

	slog.Info("calculator listening", "addr", listener.Addr().String(), "web", webDir)
	serveErr := server.Serve(listener)
	// Serve returns as soon as Shutdown starts, so wait for the drain. Restoring
	// the default signal handling first means a second signal can still end a
	// drain that hangs.
	stop()
	if errors.Is(serveErr, http.ErrServerClosed) {
		<-drained
		return nil
	}
	return serveErr
}

func defaultAddr() string {
	if port := strings.TrimPrefix(os.Getenv("PORT"), ":"); port != "" {
		return ":" + port
	}
	return ":8080"
}
