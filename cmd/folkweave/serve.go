package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/airencracken/folkweave/internal/folkweave"
)

func serveReadOnlyDemo(ctx context.Context, out io.Writer, listen string, config folkweave.Config) (err error) {
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	if config.BaseURL == "" {
		config.BaseURL = "http://" + listener.Addr().String()
	}
	app, cleanup, err := folkweave.NewReadOnlyDemo(ctx, config)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, cleanup()) }()
	return serveHTTP(ctx, out, listener, config.BaseURL, app, "Read-only demo with fictional posts and photos.", nil)
}

func serveHTTP(ctx context.Context, out io.Writer, listener net.Listener, base string, handler http.Handler, message string, flush func(context.Context) error) error {
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 5 * time.Minute, WriteTimeout: 5 * time.Minute, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 32 << 10}
	finished := make(chan error, 1)
	go func() { finished <- server.Serve(listener) }()
	fmt.Fprintln(out, "Folkweave is listening at", base)
	if message != "" {
		fmt.Fprintln(out, message)
	}
	flushCounts := func(ctx context.Context) {
		if flush != nil {
			if err := flush(ctx); err != nil {
				slog.Warn("save request counts", "error", err)
			}
		}
	}
	select {
	case err := <-finished:
		flushCounts(context.Background())
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		err := server.Shutdown(shutdown)
		if err != nil {
			server.Close()
		}
		flushCounts(shutdown)
		return err
	}
}
