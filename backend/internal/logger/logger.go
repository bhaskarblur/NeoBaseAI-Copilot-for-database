package logger

import (
	"fmt"
	"neobase-ai/config"
	"os"
	"strings"
	"sync"
	"time"

	logcastle "github.com/bhaskarblur/go-logcastle"
)

var (
	initOnce  sync.Once
	closeOnce sync.Once
)

// InitLogger initializes go-logcastle for centralized log aggregation.
// It intercepts logs from ANY library (stdlib, Logrus, Zap, MongoDB, Redis, etc.).
// For development: Text format with colors (better readability)
// For production: JSON format (structured logs for aggregators)
func InitLogger() error {
	var err error
	initOnce.Do(func() {
		// Determine format based on environment
		isDevelopment := strings.EqualFold(config.Env.Environment, "development") || strings.EqualFold(config.Env.Environment, "develop")
		logFormat := logcastle.JSON
		useColors := false

		if isDevelopment {
			logFormat = logcastle.Text
			useColors = true
		}

		logConfig := logcastle.Config{
			// Format: Text with colors for development, JSON for production
			Format: logFormat,

			// Info level: captures Info/Warn/Error/Fatal (filters Debug)
			Level: logcastle.LevelInfo,

			// Write to stdout (container platforms capture this automatically)
			Output: os.Stdout,

			// Performance tuning for production workloads
			BufferSize:    10000,                  // Buffer up to 10k logs
			FlushInterval: 100 * time.Millisecond, // Flush every 100ms

			// Global fields added to EVERY log (including third-party libs)
			EnrichFields: map[string]interface{}{
				"env": config.Env.Environment,
			},

			// Human-readable timestamps
			TimestampFormat:    logcastle.TimestampFormatDateTime,
			IncludeLoggerField: true,

			// Format-specific options
			ColorOutput:   useColors, // ANSI colors only for Text format in development
			FlattenFields: true,      // Root-level fields for Grafana/Loki label extraction
		}

		err = logcastle.Init(logConfig)
		if err != nil {
			// Can't use log.Printf here - interception not ready
			fmt.Fprintf(os.Stderr, "❌ Failed to initialize log aggregation: %v\n", err)
			return
		}

		// Wait for stdout/stderr interception to be fully active
		logcastle.WaitReady()

		// This log will be captured and formatted as JSON
		// (will have log_parse_error field since it's plain text - that's OK!)
		fmt.Fprintf(os.Stdout, "✅ Log aggregation (go-logcastle) initialized\n")
	})
	return err
}

// CloseLogger gracefully shuts down log aggregation and flushes buffers.
// Call this in defer or signal handlers before exit.
func CloseLogger() error {
	var err error
	closeOnce.Do(func() {
		err = logcastle.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Error closing log aggregation: %v\n", err)
		}
	})
	return err
}
