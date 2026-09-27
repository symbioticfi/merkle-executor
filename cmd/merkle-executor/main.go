package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/symbioticfi/merkle-executor/internal/config"
	"github.com/symbioticfi/merkle-executor/internal/version"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := root().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func root() *cobra.Command {
	cmd := &cobra.Command{Use: "merkle-executor", Short: "Execute committed Merkle recipes", SilenceUsage: true, SilenceErrors: true}
	var runPath string
	var once bool
	run := &cobra.Command{Use: "run", Short: "Run configured strategies", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return runBot(cmd.Context(), runPath, once, false, cmd.OutOrStdout())
	}}
	run.Flags().StringVar(&runPath, "config", "", "YAML configuration path")
	run.Flags().BoolVar(&once, "once", false, "process one iteration")
	_ = run.MarkFlagRequired("config")
	var planPath string
	plan := &cobra.Command{Use: "plan", Short: "Discover, assemble and simulate without signing or submitting", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return runBot(cmd.Context(), planPath, true, true, cmd.OutOrStdout())
	}}
	plan.Flags().StringVar(&planPath, "config", "", "YAML configuration path")
	_ = plan.MarkFlagRequired("config")
	var checkPath string
	check := &cobra.Command{Use: "check-config", Short: "Validate framework configuration without RPC access or reading keys", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := config.Load(checkPath)
		if err == nil {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "configuration valid")
		}
		return err
	}}
	check.Flags().StringVar(&checkPath, "config", "", "YAML configuration path")
	_ = check.MarkFlagRequired("config")
	v := &cobra.Command{Use: "version", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s (%s, %s)\n", version.Version, version.Commit, version.Date)
		return err
	}}
	cmd.AddCommand(run, plan, check, v)
	cmd.AddCommand(stateCommands()...)
	return cmd
}
