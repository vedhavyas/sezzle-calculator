package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestRunDrainsInFlightRequestOnCancel(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		_, _ = io.WriteString(w, "done")
	})}

	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() { finished <- run(ctx, cancel, listener, server) }()

	response := make(chan string, 1)
	go func() {
		resp, err := http.Get("http://" + listener.Addr().String() + "/")
		if err != nil {
			response <- "error: " + err.Error()
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		response <- string(body)
	}()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the request never reached the handler")
	}

	cancel()

	// The drain must wait for the handler rather than closing the connection:
	// run may not return until the request below is released.
	select {
	case err := <-finished:
		t.Fatalf("run returned before the in-flight request finished: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)

	select {
	case body := <-response:
		if body != "done" {
			t.Fatalf("body = %q, want the in-flight response to finish", body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the in-flight request was not drained")
	}

	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("run returned %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not return after the drain")
	}
}
