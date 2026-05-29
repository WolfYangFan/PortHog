package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

type Config struct {
	PortSpec string
	Debug    bool
	LogLevel string
	Duration time.Duration
	JSON     bool
	BindIP   string
	PortFile string
}

var cfg Config

var rootCmd = &cobra.Command{
	Use:   "porthog",
	Short: "Occupy TCP ports by listening and RST-closing connections",
	Long: `PortHog occupies TCP ports on a machine by binding listeners that
immediately RST-close every incoming connection. Useful for reserving
ports during CI/CD, integration tests, or local development.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := setupLogger(cfg.Debug, cfg.LogLevel, cfg.JSON)
		printStartupBanner(cfg.JSON, logger)

		portSpec := cfg.PortSpec

		if portSpec != "" {
			expanded, err := expandAtRefs(portSpec)
			if err != nil {
				logger.Fatal("Failed to expand @ references", "error", err)
			}
			portSpec = expanded
		}

		if cfg.PortFile != "" {
			filePorts, err := loadPortsFromFile(cfg.PortFile)
			if err != nil {
				logger.Fatal("Failed to load port file", "path", cfg.PortFile, "error", err)
			}
			if portSpec != "" {
				portSpec += "," + filePorts
			} else {
				portSpec = filePorts
			}
		}

		if portSpec == "" {
			logger.Fatal("No ports specified. Use -p or --port-file.")
		}

		portList, err := parsePorts(portSpec)
		if err != nil {
			logger.Fatal("Failed to parse ports", "error", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		listeners := make([]*Listener, 0, len(portList))

		for _, port := range portList {
			l, err := startListener(ctx, &wg, port, cfg.BindIP, logger)
			if err != nil {
				logger.Error("Failed to start listener", "port", port, "error", err)
				continue
			}
			listeners = append(listeners, l)
		}

		if len(listeners) == 0 {
			logger.Fatal("No valid ports available")
		}

		if cfg.Duration > 0 {
			logger.Info("Auto-shutdown scheduled", "duration", cfg.Duration)
			time.AfterFunc(cfg.Duration, func() {
				logger.Info("Duration elapsed, initiating shutdown...")
				cancel()
			})
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		select {
		case sig := <-sigCh:
			logger.Info("Received signal - initiating graceful shutdown...", "signal", sig)
			cancel()
		case <-ctx.Done():
		}

		shutdown(ctx, listeners, logger)
		wg.Wait()

		logger.Info("Shutdown completed")
		return nil
	},
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate shell completion script for the specified shell.

To load completions:

  Bash:
    porthog completion bash > /etc/bash_completion.d/porthog

  Zsh:
    porthog completion zsh > /usr/local/share/zsh/site-functions/_porthog

  Fish:
    porthog completion fish > ~/.config/fish/completions/porthog.fish

  PowerShell:
    porthog completion powershell > profile.ps1`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletion(os.Stdout)
		default:
			return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish, powershell)", args[0])
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&cfg.PortSpec, "ports", "p", "", "Port specification (e.g. 8080,9000-9005,@file.txt)")
	rootCmd.Flags().BoolVar(&cfg.Debug, "debug", false, "Enable debug mode")
	rootCmd.Flags().StringVar(&cfg.LogLevel, "level", "", "Set log level (debug, info, warn, error)")
	rootCmd.Flags().DurationVarP(&cfg.Duration, "duration", "d", 0, "Auto-shutdown after duration (e.g. 30s, 5m)")
	rootCmd.Flags().BoolVar(&cfg.JSON, "json", false, "Enable JSON output")
	rootCmd.Flags().StringVar(&cfg.BindIP, "interface", "", "Bind to specific IP address")
	rootCmd.Flags().StringVar(&cfg.PortFile, "port-file", "", "Load port list from file")

	rootCmd.AddCommand(completionCmd)
}
