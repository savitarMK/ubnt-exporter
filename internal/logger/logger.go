// Package logger handles formatted logging to console and optional rotating files.
package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ubnt-exporter/internal/config"
	"ubnt-exporter/internal/definitions"
)

// ContextKey defines the context key type for logger correlation.
type ContextKey string

const (
	// TraceIDKey is the context key for tracing and correlation IDs.
	TraceIDKey ContextKey = "trace_id"
)

// WithTraceID returns a copy of parent context with the trace ID attached.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// GetTraceID extracts the trace ID from the context, or returns an empty string if absent.
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(TraceIDKey).(string); ok {
		return v
	}
	return ""
}

// Level represents the severity level of a log entry.
type Level int

// Log severity levels.
const (
	// LevelDebug logs verbose debug messages.
	LevelDebug Level = iota
	// LevelInfo logs standard informational messages.
	LevelInfo
	// LevelWarn logs recoverable warnings.
	LevelWarn
	// LevelError logs critical error events.
	LevelError
)

// ParseLevel converts a string representation of a log level to its typed Level value.
func ParseLevel(lvl string) Level {
	switch strings.ToUpper(strings.TrimSpace(lvl)) {
	case "DEBUG":
		return LevelDebug
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR":
		return LevelError
	default:
		return LevelInfo
	}
}

func parseLevel(lvl string) Level {
	return ParseLevel(lvl)
}

func levelString(lvl Level) string {
	switch lvl {
	case LevelDebug:
		return "DEBUG"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

var facilityMap = map[string]int{
	"kern": 0, "user": 1, "mail": 2, "daemon": 3, "auth": 4, "syslog": 5,
	"lpr": 6, "news": 7, "uucp": 8, "cron": 9, "authpriv": 10, "ftp": 11,
	"ntp": 12, "security": 13, "console": 14, "solaris-cron": 15,
	"local0": 16, "local1": 17, "local2": 18, "local3": 19,
	"local4": 20, "local5": 21, "local6": 22, "local7": 23,
}

func getFacilityCode(name string) int {
	if code, ok := facilityMap[strings.ToLower(strings.TrimSpace(name))]; ok {
		return code
	}
	return 1 // default to 'user'
}

type dampenRecord struct {
	caller     string
	count      int
	suppressed int
	lastEmit   time.Time
}

// Logger is a concurrent-safe multi-format logger.
type Logger struct {
	mu           sync.Mutex
	level        Level
	format       string // "text", "cef", "rfc5424", "syslog", "json"
	facility     int
	hostname     string
	pid          int
	writers      []io.Writer
	fileWriter   *os.File
	filePath     string
	fileMaxSize  int64
	fileBackups  int
	fileCurBytes int64

	dampenEnabled   bool
	dampenThreshold int
	dampenInterval  time.Duration
	dampenMap       map[string]*dampenRecord
}

var defaultLogger *Logger
var once sync.Once

// Get returns the singleton Logger instance.
func Get() *Logger {
	once.Do(func() {
		h, _ := os.Hostname()
		if h == "" {
			h = "localhost"
		}
		defaultLogger = &Logger{
			level:           LevelInfo,
			format:          "text",
			facility:        1,
			hostname:        h,
			pid:             os.Getpid(),
			writers:         []io.Writer{os.Stdout},
			dampenEnabled:   false,
			dampenThreshold: 3,
			dampenInterval:  300 * time.Second,
			dampenMap:       make(map[string]*dampenRecord),
		}
	})
	return defaultLogger
}

// Setup configures the global logger based on system configuration.
func Setup(cfg *config.SystemConfig) {
	l := Get()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.level = parseLevel(cfg.Logging.Level)
	l.facility = getFacilityCode(cfg.Logging.SyslogFacility)

	if cfg.Logging.Formats.CEF {
		l.format = "cef"
	} else if cfg.Logging.Formats.RFC5424 {
		l.format = "rfc5424"
	} else if cfg.Logging.Formats.Syslog {
		l.format = "syslog"
	} else if cfg.Logging.Formats.JSON {
		l.format = "json"
	} else {
		l.format = "text"
	}

	l.dampenEnabled = cfg.Logging.Dampening.Enabled
	l.dampenThreshold = cfg.Logging.Dampening.Threshold
	if l.dampenThreshold <= 0 {
		l.dampenThreshold = 3
	}
	intervalSec := cfg.Logging.Dampening.Interval
	if intervalSec <= 0 {
		intervalSec = 300
	}
	l.dampenInterval = time.Duration(intervalSec) * time.Second
	l.dampenMap = make(map[string]*dampenRecord)

	// Close existing file writer if any
	if l.fileWriter != nil {
		_ = l.fileWriter.Close()
		l.fileWriter = nil
		l.fileCurBytes = 0
	}

	l.writers = nil
	if cfg.Logging.Console {
		l.writers = append(l.writers, os.Stdout)
	}

	if cfg.Logging.File.Enabled && strings.TrimSpace(cfg.Logging.File.Path) != "" {
		filePath := cfg.Logging.File.Path
		if dir := filepath.Dir(filePath); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0750)
		}
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Could not open log file %q: %v\n", filePath, err)
		} else {
			info, statErr := f.Stat()
			if statErr == nil && info != nil {
				l.fileCurBytes = info.Size()
			}
			l.fileWriter = f
			l.filePath = filePath
			l.fileMaxSize = int64(cfg.Logging.File.MaxSizeMB) * 1024 * 1024
			if l.fileMaxSize <= 0 {
				l.fileMaxSize = 5 * 1024 * 1024
			}
			l.fileBackups = cfg.Logging.File.BackupCount
			l.writers = append(l.writers, f)
		}
	}
}

// SetDampening dynamically configures repetitive log throttling parameters.
func (l *Logger) SetDampening(enabled bool, threshold int, interval time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.dampenEnabled = enabled
	l.dampenThreshold = threshold
	if l.dampenThreshold <= 0 {
		l.dampenThreshold = 3
	}
	if interval <= 0 {
		interval = 300 * time.Second
	}
	l.dampenInterval = interval
	l.dampenMap = make(map[string]*dampenRecord)
}

// SetDampening dynamically configures repetitive log throttling parameters on the default logger.
func SetDampening(enabled bool, threshold int, interval time.Duration) {
	Get().SetDampening(enabled, threshold, interval)
}

// ResetDampening clears repetitive message dampening records for a specific caller name.
// If name is empty, all active dampening records are purged.
func (l *Logger) ResetDampening(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.dampenMap == nil {
		l.dampenMap = make(map[string]*dampenRecord)
		return
	}
	if name == "" {
		l.dampenMap = make(map[string]*dampenRecord)
		return
	}
	for k, rec := range l.dampenMap {
		if rec.caller == name {
			delete(l.dampenMap, k)
		}
	}
}

// ResetDampening clears repetitive message dampening records for a specific caller name on the default logger.
func ResetDampening(name string) {
	Get().ResetDampening(name)
}

// SetOutput overrides logger destinations with a specific writer (useful for unit testing).
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if w == nil {
		l.writers = nil
	} else {
		l.writers = []io.Writer{w}
	}
}

// SetOutput overrides default logger destinations with a specific writer.
func SetOutput(w io.Writer) {
	Get().SetOutput(w)
}

// SetLevel dynamically updates the logger severity threshold at runtime.
func SetLevel(lvl string) {
	l := Get()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = parseLevel(lvl)
}

// GetLevel returns the current log severity level.
func GetLevel() Level {
	l := Get()
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}

// Close closes any open file writers and flushes logging resources.
func Close() {
	l := Get()
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.fileWriter != nil {
		_ = l.fileWriter.Close()
		l.fileWriter = nil
	}
}

// FormatMessageContext formats a log message according to the active configuration format,
// including correlation trace IDs from the context if present.
func (l *Logger) FormatMessageContext(ctx context.Context, lvl Level, name, msg string) string {
	now := time.Now()
	lvlStr := levelString(lvl)
	traceID := GetTraceID(ctx)

	switch l.format {
	case "cef":
		severity := 3
		switch lvl {
		case LevelDebug:
			severity = 0
		case LevelInfo:
			severity = 3
		case LevelWarn:
			severity = 6
		case LevelError:
			severity = 8
		}
		cleanMsg := strings.ReplaceAll(msg, "\\", "\\\\")
		cleanMsg = strings.ReplaceAll(cleanMsg, "|", `\|`)
		cleanMsg = strings.ReplaceAll(cleanMsg, "\n", " ")
		extra := ""
		if traceID != "" {
			cleanTrace := strings.ReplaceAll(traceID, "|", `\|`)
			extra = fmt.Sprintf(" cs1=%s cs1Label=TraceID", cleanTrace)
		}
		return fmt.Sprintf("CEF:0|MatiasSegurola|UbiquitiExporter|%s|100|%s|%d|shost=%s msg=%s%s\n",
			definitions.VERSION, lvlStr, severity, l.hostname, cleanMsg, extra)

	case "rfc5424":
		sevCode := 6
		switch lvl {
		case LevelDebug:
			sevCode = 7
		case LevelInfo:
			sevCode = 6
		case LevelWarn:
			sevCode = 4
		case LevelError:
			sevCode = 3
		}
		pri := (l.facility * 8) + sevCode
		timestamp := now.Format(time.RFC3339)
		msgID := name
		if msgID == "" {
			msgID = "-"
		}
		sd := "-"
		if traceID != "" {
			sd = fmt.Sprintf("[trace@4711 id=%q]", traceID)
		}
		return fmt.Sprintf("<%d>1 %s %s ubnt-exporter %d %s %s %s\n",
			pri, timestamp, l.hostname, l.pid, msgID, sd, msg)

	case "syslog":
		sevCode := 6
		switch lvl {
		case LevelDebug:
			sevCode = 7
		case LevelInfo:
			sevCode = 6
		case LevelWarn:
			sevCode = 4
		case LevelError:
			sevCode = 3
		}
		pri := (l.facility * 8) + sevCode
		timestamp := now.Format("Jan 02 15:04:05")
		if traceID != "" {
			return fmt.Sprintf("<%d>%s %s ubnt-exporter[%d]: [%s] %s\n",
				pri, timestamp, l.hostname, l.pid, traceID, msg)
		}
		return fmt.Sprintf("<%d>%s %s ubnt-exporter[%d]: %s\n",
			pri, timestamp, l.hostname, l.pid, msg)

	case "json":
		entry := struct {
			Timestamp string `json:"timestamp"`
			Level     string `json:"level"`
			Caller    string `json:"caller"`
			TraceID   string `json:"trace_id,omitempty"`
			Message   string `json:"message"`
			Host      string `json:"host"`
			PID       int    `json:"pid"`
		}{
			Timestamp: now.Format(time.RFC3339),
			Level:     lvlStr,
			Caller:    name,
			TraceID:   traceID,
			Message:   msg,
			Host:      l.hostname,
			PID:       l.pid,
		}
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Sprintf("{\"error\":\"failed to marshal log: %s\"}\n", err.Error())
		}
		return string(data) + "\n"

	default: // "text"
		callerStr := name
		if traceID != "" {
			callerStr = fmt.Sprintf("%s [%s]", name, traceID)
		}
		return fmt.Sprintf("%s | %-5s | %s | %s\n",
			now.Format("2006-01-02 15:04:05"), lvlStr, callerStr, msg)
	}
}

// FormatMessage formats a log message according to the active configuration format.
func (l *Logger) FormatMessage(lvl Level, name, msg string) string {
	return l.FormatMessageContext(context.Background(), lvl, name, msg)
}

func (l *Logger) formatMessage(lvl Level, name, msg string) string {
	return l.FormatMessage(lvl, name, msg)
}

// LogContext outputs a message at the specified severity level, correlating context if present.
func (l *Logger) LogContext(ctx context.Context, lvl Level, name, msg string) {
	if lvl < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	outMsg := msg
	if l.dampenEnabled {
		if l.dampenMap == nil {
			l.dampenMap = make(map[string]*dampenRecord)
		}
		now := time.Now()
		key := fmt.Sprintf("%d:%s:%s", lvl, name, msg)
		rec, exists := l.dampenMap[key]
		if !exists {
			l.dampenMap[key] = &dampenRecord{
				caller:     name,
				count:      1,
				suppressed: 0,
				lastEmit:   now,
			}
		} else {
			if now.Sub(rec.lastEmit) >= l.dampenInterval {
				if rec.suppressed > 0 {
					outMsg = fmt.Sprintf("%s (suppressed %d identical messages)", msg, rec.suppressed)
				}
				rec.count = 1
				rec.suppressed = 0
				rec.lastEmit = now
			} else {
				rec.count++
				if rec.count > l.dampenThreshold {
					rec.suppressed++
					return
				}
			}
		}
	}

	formatted := l.FormatMessageContext(ctx, lvl, name, outMsg)
	b := []byte(formatted)

	for _, w := range l.writers {
		_, _ = w.Write(b)
	}

	// Check file rotation if file writer is active
	if l.fileWriter != nil && l.fileMaxSize > 0 {
		l.fileCurBytes += int64(len(b))
		if l.fileCurBytes >= l.fileMaxSize {
			l.rotateFile()
		}
	}
}

func (l *Logger) log(lvl Level, name, msg string) {
	l.LogContext(context.Background(), lvl, name, msg)
}

func (l *Logger) rotateFile() {
	if l.fileWriter != nil {
		_ = l.fileWriter.Close()
	}

	for i := l.fileBackups - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", l.filePath, i)
		newPath := fmt.Sprintf("%s.%d", l.filePath, i+1)
		_ = os.Rename(oldPath, newPath)
	}
	if l.fileBackups > 0 {
		_ = os.Rename(l.filePath, fmt.Sprintf("%s.1", l.filePath))
	}

	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0640)
	if err == nil {
		l.fileWriter = f
		l.fileCurBytes = 0
		// Update writer list
		for idx, w := range l.writers {
			if _, ok := w.(*os.File); ok && w != os.Stdout {
				l.writers[idx] = f
			}
		}
	}
}

// DebugContext logs a message at LevelDebug with correlation context.
func DebugContext(ctx context.Context, name, format string, args ...any) {
	Get().LogContext(ctx, LevelDebug, name, fmt.Sprintf(format, args...))
}

// InfoContext logs a message at LevelInfo with correlation context.
func InfoContext(ctx context.Context, name, format string, args ...any) {
	Get().LogContext(ctx, LevelInfo, name, fmt.Sprintf(format, args...))
}

// WarnContext logs a message at LevelWarn with correlation context.
func WarnContext(ctx context.Context, name, format string, args ...any) {
	Get().LogContext(ctx, LevelWarn, name, fmt.Sprintf(format, args...))
}

// ErrorContext logs a message at LevelError with correlation context.
func ErrorContext(ctx context.Context, name, format string, args ...any) {
	Get().LogContext(ctx, LevelError, name, fmt.Sprintf(format, args...))
}

// Debug logs a message at LevelDebug.
func Debug(name, format string, args ...any) {
	DebugContext(context.Background(), name, format, args...)
}

// Info logs a message at LevelInfo.
func Info(name, format string, args ...any) {
	InfoContext(context.Background(), name, format, args...)
}

// Warn logs a message at LevelWarn.
func Warn(name, format string, args ...any) {
	WarnContext(context.Background(), name, format, args...)
}

// Error logs a message at LevelError.
func Error(name, format string, args ...any) {
	ErrorContext(context.Background(), name, format, args...)
}

var (
	exitMu   sync.Mutex
	exitFunc = os.Exit
)

// SetExitFunc overrides the process termination function (primarily for unit testing).
func SetExitFunc(fn func(int)) {
	exitMu.Lock()
	defer exitMu.Unlock()
	exitFunc = fn
}

// ResetExitFunc restores the standard os.Exit termination function.
func ResetExitFunc() {
	exitMu.Lock()
	defer exitMu.Unlock()
	exitFunc = os.Exit
}

func getExitFunc() func(int) {
	exitMu.Lock()
	defer exitMu.Unlock()
	if exitFunc == nil {
		return os.Exit
	}
	return exitFunc
}

// FatalContext logs a critical message at LevelError with correlation context, flushes outputs, and terminates.
func FatalContext(ctx context.Context, name, format string, args ...any) {
	Get().LogContext(ctx, LevelError, name, fmt.Sprintf(format, args...))
	Close()
	getExitFunc()(1)
}

// Fatal logs a critical message at LevelError, flushes outputs, and terminates the application.
func Fatal(name, format string, args ...any) {
	FatalContext(context.Background(), name, format, args...)
}
