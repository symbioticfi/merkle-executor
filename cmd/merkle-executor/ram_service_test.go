package main

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/symbioticfi/merkle-executor/internal/config"
	"github.com/symbioticfi/merkle-executor/internal/service"
	runtime "github.com/symbioticfi/merkle-executor/sdk/runtime"
)

func runServiceOnce(ctx context.Context, cfg *config.Config, client *http.Client) ([]runtime.Workflow, error) {
	var snapshot []runtime.Workflow
	err := service.Run(ctx, cfg, service.Options{Once: true, Output: io.Discard, HTTP: client, OnSnapshot: func(rows []runtime.Workflow) { snapshot = rows }})
	return snapshot, err
}

// One service process retains external identity between observations, without files.
func startRAMService(t *testing.T, ctx context.Context, cfg *config.Config, client *http.Client) func() []runtime.Workflow {
	t.Helper()
	running, cancel := context.WithCancel(ctx)
	snapshots := make(chan []runtime.Workflow)
	done := make(chan error, 1)
	cfgCopy := *cfg
	cfgCopy.Runtime.PollInterval = time.Millisecond
	go func() {
		done <- service.Run(running, &cfgCopy, service.Options{Output: io.Discard, HTTP: client, OnSnapshot: func(rows []runtime.Workflow) {
			select {
			case snapshots <- rows:
			case <-running.Done():
			}
		}})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(10 * time.Second):
			t.Error("RAM service did not drain")
		}
	})
	return func() []runtime.Workflow {
		t.Helper()
		select {
		case rows := <-snapshots:
			return rows
		case err := <-done:
			t.Fatalf("RAM service stopped: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		return nil
	}
}

func workflowNamed(t *testing.T, rows []runtime.Workflow, name string) runtime.Workflow {
	t.Helper()
	for _, row := range rows {
		if row.Name == name {
			return row
		}
	}
	t.Fatalf("RAM workflow %s missing", name)
	return runtime.Workflow{}
}
