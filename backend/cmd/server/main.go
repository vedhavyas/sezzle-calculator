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

// serve listens, starts the server and drains it when a signal arrives. It is
// split from main so the process wiring is one short path and the drain can be
// tested without a signal.
func serve(addr, webDir string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("calculator listening", "addr", listener.Addr().String(), "web", webDir)
	return run(ctx, stop, listener, newServer(webDir))
}

func newServer(webDir string) *http.Server {
	return &http.Server{
		Handler:           httpapi.New(webDir),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// run serves until ctx is canceled, then drains in-flight requests for at most
// shutdownTimeout. stop restores the default signal handling after Serve
// returns, so a second signal can still end a drain that hangs.
func run(ctx context.Context, stop context.CancelFunc, listener net.Listener, server *http.Server) error {
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

	serveErr := server.Serve(listener)
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
