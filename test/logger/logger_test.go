package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubnt-exporter/internal/config"
	"ubnt-exporter/internal/logger"
)

func TestLoggerFormats(t *testing.T) {
	l := logger.Get()

	// 1. Text format
	cfgText := config.DefaultSystemConfig()
	cfgText.Logging.Formats.CEF = false
	cfgText.Logging.Formats.Text = true
	logger.Setup(cfgText)

	msgText := l.FormatMessage(logger.LevelInfo, "testDevice", "Test message for text")
	if !strings.Contains(msgText, "INFO") || !strings.Contains(msgText, "testDevice") || !strings.Contains(msgText, "Test message for text") {
		t.Errorf("Unexpected text format: %q", msgText)
	}

	// 2. CEF format
	cfgCEF := config.DefaultSystemConfig()
	cfgCEF.Logging.Formats.CEF = true
	logger.Setup(cfgCEF)

	msgCEF := l.FormatMessage(logger.LevelWarn, "testDevice", "Device | with | pipe and \n newline")
	if !strings.HasPrefix(msgCEF, "CEF:0|MatiasSegurola|UbiquitiExporter|") {
		t.Errorf("CEF format missing header prefix: %q", msgCEF)
	}
	if !strings.Contains(msgCEF, "sproc=testDevice") {
		t.Errorf("CEF format missing sproc component identifier: %q", msgCEF)
	}
	if !strings.Contains(msgCEF, `Device \| with \| pipe and   newline`) {
		t.Errorf("CEF format failed to properly escape pipe or newline: %q", msgCEF)
	}

	// 3. RFC5424 format
	cfg5424 := config.DefaultSystemConfig()
	cfg5424.Logging.Formats.CEF = false
	cfg5424.Logging.Formats.RFC5424 = true
	cfg5424.Logging.SyslogFacility = "local0" // facility 16
	logger.Setup(cfg5424)

	msg5424 := l.FormatMessage(logger.LevelInfo, "dev1", "RFC5424 test")
	// Facility 16 * 8 + severity 6 = 134
	if !strings.HasPrefix(msg5424, "<134>1 ") {
		t.Errorf("RFC5424 format PRI unexpected: %q", msg5424)
	}
	if !strings.Contains(msg5424, "ubnt-exporter") || !strings.Contains(msg5424, "dev1") {
		t.Errorf("RFC5424 format missing tags: %q", msg5424)
	}

	// 4. Syslog (RFC 3164) format
	cfg3164 := config.DefaultSystemConfig()
	cfg3164.Logging.Formats.CEF = false
	cfg3164.Logging.Formats.Syslog = true
	cfg3164.Logging.SyslogFacility = "local0"
	logger.Setup(cfg3164)

	msg3164 := l.FormatMessage(logger.LevelError, "dev1", "RFC3164 test")
	// Facility 16 * 8 + severity 3 = 131
	if !strings.HasPrefix(msg3164, "<131>") {
		t.Errorf("RFC3164 format PRI unexpected: %q", msg3164)
	}
	if !strings.Contains(msg3164, "ubnt-exporter[") {
		t.Errorf("RFC3164 format missing process tag: %q", msg3164)
	}

	// 5. JSON format
	cfgJSON := config.DefaultSystemConfig()
	cfgJSON.Logging.Formats.CEF = false
	cfgJSON.Logging.Formats.JSON = true
	logger.Setup(cfgJSON)

	msgJSON := l.FormatMessage(logger.LevelWarn, "devJSON", "JSON test message")
	var parsedJSON map[string]any
	if err := json.Unmarshal([]byte(msgJSON), &parsedJSON); err != nil {
		t.Fatalf("failed to parse JSON log message: %v (raw: %q)", err, msgJSON)
	}
	if parsedJSON["level"] != "WARN" {
		t.Errorf("expected level WARN, got %v", parsedJSON["level"])
	}
	if parsedJSON["caller"] != "devJSON" {
		t.Errorf("expected caller devJSON, got %v", parsedJSON["caller"])
	}
	if parsedJSON["message"] != "JSON test message" {
		t.Errorf("expected message 'JSON test message', got %v", parsedJSON["message"])
	}
	if parsedJSON["timestamp"] == nil || parsedJSON["timestamp"] == "" {
		t.Errorf("expected non-empty timestamp in JSON log: %v", parsedJSON)
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	cfg := config.DefaultSystemConfig()
	cfg.Logging.Level = "WARN"
	logger.Setup(cfg)

	if logger.GetLevel() != logger.LevelWarn {
		t.Fatalf("expected logger level WARN, got %v", logger.GetLevel())
	}

	if logger.ParseLevel("DEBUG") != logger.LevelDebug {
		t.Errorf("DEBUG parsed incorrectly")
	}
	if logger.ParseLevel("ERROR") != logger.LevelError {
		t.Errorf("ERROR parsed incorrectly")
	}
	if logger.ParseLevel("UNKNOWN") != logger.LevelInfo {
		t.Errorf("UNKNOWN level should default to INFO")
	}
}

func TestLoggerFileWritingAndRotation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "subdir", "test.log")
	_ = os.MkdirAll(filepath.Dir(logFile), 0750)
	_ = os.WriteFile(logFile, []byte("initial\n"), 0600)
	_ = os.Truncate(logFile, 1024*1024-10)

	cfg := config.DefaultSystemConfig()
	cfg.Logging.Level = "DEBUG"
	cfg.Logging.SyslogFacility = "unknown_facility"
	cfg.Logging.Console = false
	cfg.Logging.File.Enabled = true
	cfg.Logging.File.Path = logFile
	cfg.Logging.File.MaxSizeMB = 1
	cfg.Logging.File.BackupCount = 2
	logger.Setup(cfg)

	logger.Debug("tester", "Line 1 triggers rotation")
	logger.Warn("tester", "Line 2 in new rotated file")
	logger.Info("tester", "Line 3 in new rotated file")

	recent := logger.GetRecentLogs(2, "tester")
	if len(recent) != 2 {
		t.Errorf("expected 2 recent logs from ring buffer, got %d", len(recent))
	}

	logger.Close()

	if _, err := os.Stat(logFile + ".1"); err != nil {
		t.Errorf("expected rotated backup file %s.1 to exist: %v", logFile, err)
	}

	// #nosec G304 -- test reads generated test file
	content, err := os.ReadFile(filepath.Clean(logFile))
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if !strings.Contains(string(content), "Line 2 in new rotated file") {
		t.Errorf("expected new log file to contain post-rotation message, got: %s", string(content))
	}
}

func TestLoggerDynamicLevelUpdate(t *testing.T) {
	_ = logger.WithTraceID(nil, "nil-ctx-trace") //nolint:staticcheck // testing nil context guard
	logger.SetLevel("DEBUG")
	if logger.GetLevel() != logger.LevelDebug {
		t.Errorf("expected LevelDebug, got %v", logger.GetLevel())
	}
	logger.SetLevel("ERROR")
	if logger.GetLevel() != logger.LevelError {
		t.Errorf("expected LevelError, got %v", logger.GetLevel())
	}
	logger.SetLevel("INFO")
	if logger.GetLevel() != logger.LevelInfo {
		t.Errorf("expected LevelInfo, got %v", logger.GetLevel())
	}
}

func TestLoggerContextCorrelation(t *testing.T) {
	l := logger.Get()
	ctx := logger.WithTraceID(context.Background(), "trace-xyz-123")

	if logger.GetTraceID(ctx) != "trace-xyz-123" {
		t.Fatalf("expected trace ID 'trace-xyz-123', got %q", logger.GetTraceID(ctx))
	}
	if logger.GetTraceID(nil) != "" { //nolint:staticcheck // intentionally testing nil context safety
		t.Errorf("expected empty trace ID for nil context")
	}

	// 1. JSON format with trace ID
	cfgJSON := config.DefaultSystemConfig()
	cfgJSON.Logging.Formats.CEF = false
	cfgJSON.Logging.Formats.JSON = true
	logger.Setup(cfgJSON)

	msgJSON := l.FormatMessageContext(ctx, logger.LevelInfo, "devTrace", "Message with trace")
	var parsedJSON map[string]any
	if err := json.Unmarshal([]byte(msgJSON), &parsedJSON); err != nil {
		t.Fatalf("failed to parse JSON log with trace: %v", err)
	}
	if parsedJSON["trace_id"] != "trace-xyz-123" {
		t.Errorf("expected trace_id 'trace-xyz-123', got %v", parsedJSON["trace_id"])
	}

	// 2. Text format with trace ID
	cfgText := config.DefaultSystemConfig()
	cfgText.Logging.Formats.CEF = false
	cfgText.Logging.Formats.Text = true
	logger.Setup(cfgText)

	msgText := l.FormatMessageContext(ctx, logger.LevelInfo, "devTrace", "Message with trace")
	if !strings.Contains(msgText, "devTrace [trace-xyz-123]") {
		t.Errorf("expected text format to contain 'devTrace [trace-xyz-123]', got: %q", msgText)
	}

	// 3. CEF format with trace ID
	cfgCEF := config.DefaultSystemConfig()
	cfgCEF.Logging.Formats.CEF = true
	logger.Setup(cfgCEF)

	msgCEF := l.FormatMessageContext(ctx, logger.LevelInfo, "devTrace", "Message with trace")
	if !strings.Contains(msgCEF, "cs1=trace-xyz-123 cs1Label=TraceID") {
		t.Errorf("expected CEF format to contain trace extension, got: %q", msgCEF)
	}

	// 4. RFC 5424 format with trace ID
	cfg5424 := config.DefaultSystemConfig()
	cfg5424.Logging.Formats.CEF = false
	cfg5424.Logging.Formats.RFC5424 = true
	logger.Setup(cfg5424)

	msg5424 := l.FormatMessageContext(ctx, logger.LevelInfo, "devTrace", "Message with trace")
	if !strings.Contains(msg5424, `[trace@4711 id="trace-xyz-123"]`) {
		t.Errorf("expected RFC5424 format to contain trace structured data, got: %q", msg5424)
	}

	// 5. Context log functions
	logger.DebugContext(ctx, "devTrace", "Debug trace message")
	logger.InfoContext(ctx, "devTrace", "Info trace message")
	logger.WarnContext(ctx, "devTrace", "Warn trace message")
	logger.ErrorContext(ctx, "devTrace", "Error trace message")
}

func TestLoggerDampeningSuppression(t *testing.T) {
	cfg := config.DefaultSystemConfig()
	cfg.Logging.Level = "INFO"
	cfg.Logging.Formats.CEF = false
	cfg.Logging.Formats.Text = true
	cfg.Logging.Dampening.Enabled = true
	cfg.Logging.Dampening.Threshold = 2
	cfg.Logging.Dampening.Interval = 1
	logger.Setup(cfg)

	var buf bytes.Buffer
	logger.SetOutput(&buf)
	defer logger.SetOutput(os.Stdout)

	// Override with fast interval for testing
	logger.SetDampening(true, 2, 200*time.Millisecond)

	// Emit 4 identical messages
	for i := 0; i < 4; i++ {
		logger.Error("devStorm", "Repeated failure connection")
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected exactly 2 logged lines before suppression, got %d: %q", len(lines), buf.String())
	}

	// Sleep past interval
	time.Sleep(250 * time.Millisecond)
	buf.Reset()

	// Emit 5th identical message (interval expired)
	logger.Error("devStorm", "Repeated failure connection")
	postLines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(postLines) != 1 {
		t.Fatalf("expected 1 logged line with summary after interval, got %d: %q", len(postLines), buf.String())
	}
	if !strings.Contains(postLines[0], "(suppressed 2 identical messages)") {
		t.Errorf("expected summary note in log line, got: %q", postLines[0])
	}

	// Emit one more message within new threshold window
	buf.Reset()
	logger.Error("devStorm", "Repeated failure connection")
	if len(strings.Split(strings.TrimSpace(buf.String()), "\n")) != 1 {
		t.Errorf("expected 2nd message in window to be allowed, got: %q", buf.String())
	}

	// Test ResetDampening
	logger.ResetDampening("devStorm")
	buf.Reset()
	logger.Error("devStorm", "Repeated failure connection")
	logger.Error("devStorm", "Repeated failure connection")
	resetLines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(resetLines) != 2 {
		t.Errorf("expected 2 messages allowed after ResetDampening, got %d: %q", len(resetLines), buf.String())
	}

	// Test distinct caller is not suppressed by devStorm
	buf.Reset()
	logger.Error("otherDev", "Repeated failure connection")
	if !strings.Contains(buf.String(), "otherDev") {
		t.Errorf("expected otherDev to be logged without interference, got: %q", buf.String())
	}

	// Turn off dampening
	logger.SetDampening(false, 2, 200*time.Millisecond)
}

func TestLoggerFatal(t *testing.T) {
	var exitCode int
	logger.SetExitFunc(func(code int) {
		exitCode = code
	})
	defer logger.ResetExitFunc()

	var buf bytes.Buffer
	logger.SetOutput(&buf)
	defer logger.SetOutput(os.Stdout)

	logger.Fatal("testMain", "Fatal failure testing")
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 from Fatal, got %d", exitCode)
	}
	if !strings.Contains(buf.String(), "Fatal failure testing") {
		t.Errorf("expected log buffer to contain fatal message, got: %s", buf.String())
	}

	exitCode = 0
	buf.Reset()
	ctx := logger.WithTraceID(context.Background(), "fatal-trace-123")
	logger.FatalContext(ctx, "testMain", "Fatal context failure testing")
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 from FatalContext, got %d", exitCode)
	}
	if !strings.Contains(buf.String(), "Fatal context failure testing") {
		t.Errorf("expected log buffer to contain fatal context message, got: %s", buf.String())
	}
}

func TestLogger_EdgeCases(_ *testing.T) {
	// SetDampening with zero/negative values defaults to 3 and 300s
	logger.SetDampening(true, -1, -1)
	// ResetDampening with empty string purges all records
	logger.ResetDampening("")
	// SetOutput(nil) clears writers
	l := logger.Get()
	l.SetOutput(nil)
	logger.Info("caller", "ignored output")
	l.SetOutput(os.Stdout)
}
