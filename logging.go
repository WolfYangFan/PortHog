package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
)

const (
	envDebugMode    = "PORTHOG_DEBUG"
	defaultLogLevel = log.InfoLevel
)

func setupLogger(cmdDebug bool, cmdLogLevel string, jsonMode bool) *log.Logger {
	logger := log.New(os.Stdout)
	logger.SetReportTimestamp(true)

	switch {
	case cmdLogLevel != "":
		setLevelFromString(logger, cmdLogLevel)
	case cmdDebug || os.Getenv(envDebugMode) != "":
		logger.SetLevel(log.DebugLevel)
	default:
		logger.SetLevel(defaultLogLevel)
	}

	if jsonMode {
		logger.SetFormatter(log.JSONFormatter)
	} else if os.Getenv("NO_COLOR") != "1" {
		logger.SetFormatter(log.TextFormatter)
	} else {
		logger.SetFormatter(log.LogfmtFormatter)
	}

	return logger
}

func setLevelFromString(logger *log.Logger, level string) {
	switch strings.ToLower(level) {
	case "debug":
		logger.SetLevel(log.DebugLevel)
	case "info":
		logger.SetLevel(log.InfoLevel)
	case "warn", "warning":
		logger.SetLevel(log.WarnLevel)
	case "error":
		logger.SetLevel(log.ErrorLevel)
	default:
		logger.Warn("Invalid log level, using default", "input", level)
		logger.SetLevel(defaultLogLevel)
	}
}

func printStartupBanner(jsonMode bool, logger *log.Logger) {
	if jsonMode {
		b, _ := json.Marshal(map[string]any{
			"event":   "started",
			"version": Version,
			"commit":  GitCommit,
			"pid":     os.Getpid(),
		})
		fmt.Println(string(b))
	} else {
		logger.Info("PortHog started", "version", Version, "commit", GitCommit, "pid", os.Getpid())
	}
}
