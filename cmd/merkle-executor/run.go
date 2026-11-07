package main

import (
	"context"
	"github.com/symbioticfi/merkle-executor/internal/config"
	"github.com/symbioticfi/merkle-executor/internal/service"
	"io"
	"net/http"
)

func runBot(ctx context.Context, path string, once, planOnly bool, out io.Writer) error {
	return runBotWithHTTP(ctx, path, once, planOnly, out, nil)
}

// The injected transport serves API fixtures through the production composition.
func runBotWithHTTP(ctx context.Context, path string, once, planOnly bool, out io.Writer, client *http.Client) error {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	return service.Run(ctx, cfg, service.Options{Once: once, PlanOnly: planOnly, Output: out, HTTP: client})
}
