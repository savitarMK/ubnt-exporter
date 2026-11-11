package logger_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"ubnt-exporter/internal/config"
	"ubnt-exporter/internal/logger"
)

func setupFormat(t *testing.T, mutate func(*config.FormatsConfig)) *logger.Logger {
	t.Helper()
	cfg := config.DefaultSystemConfig()
	cfg.Logging.Formats = config.FormatsConfig{}
	mutate(&cfg.Logging.Formats)
	logger.Setup(cfg)
	t.Cleanup(func() { logger.Setup(config.DefaultSystemConfig()) })
	return logger.Get()
}

// Regression: CEF extension values must escape '\' and '=' (not '|'), and encode newlines,
// so SIEM parsers cannot be confused into reading forged key=value pairs.
func TestCEFExtensionEscaping(t *testing.T) {
	l := setupFormat(t, func(f *config.FormatsConfig) { f.CEF = true })
	ctx := logger.WithTraceID(context.Background(), "tr=1")
	out := l.FormatMessageContext(ctx, logger.LevelError, "AP=1", `user=admin act=x \ path|ok`+"\n2nd")

	for _, want := range []string{
		`sproc=AP\=1`,
		`msg=user\=admin act\=x \\ path|ok\n2nd`,
		`cs1=tr\=1 cs1Label=TraceID`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in CEF output: %q", want, out)
		}
	}
	if strings.Count(out, "\n") != 1 {
		t.Errorf("CEF record must be a single line: %q", out)
	}
}

// Regression: RFC 5424 MSGID must be 1-32 printable ASCII chars and SD values must be escaped.
func TestRFC5424MsgIDAndSD(t *testing.T) {
	l := setupFormat(t, func(f *config.FormatsConfig) { f.RFC5424 = true })
	cases := map[string]string{
		"AP Sector 01":            "AP_Sector_01",
		"":                        "-",
		"Torre-Ñandú":             "Torre-__and__",
		strings.Repeat("x", 40):   strings.Repeat("x", 32),
		"plain-device_01.example": "plain-device_01.example",
	}
	for name, wantID := range cases {
		out := l.FormatMessage(logger.LevelInfo, name, "hello\nworld")
		fields := strings.SplitN(out, " ", 8) // PRI+VER TS HOST APP PROCID MSGID SD MSG
		if len(fields) < 8 || fields[5] != wantID {
			t.Errorf("name %q: expected MSGID %q, got record %q", name, wantID, out)
		}
		if strings.Count(out, "\n") != 1 {
			t.Errorf("RFC 5424 record must be a single line: %q", out)
		}
	}

	ctx := logger.WithTraceID(context.Background(), `a"b]c\d`)
	out := l.FormatMessageContext(ctx, logger.LevelInfo, "dev", "m")
	if !strings.Contains(out, `[trace@4711 id="a\"b\]c\\d"]`) {
		t.Errorf("expected escaped SD-PARAM value, got %q", out)
	}
}

// Regression: in line-oriented formats a message containing newlines must not forge extra records.
func TestLineFormatsResistNewlineInjection(t *testing.T) {
	formats := map[string]func(*config.FormatsConfig){
		"text":   func(f *config.FormatsConfig) { f.Text = true },
		"syslog": func(f *config.FormatsConfig) { f.Syslog = true },
	}
	for name, mutate := range formats {
		t.Run(name, func(t *testing.T) {
			l := setupFormat(t, mutate)
			out := l.FormatMessage(logger.LevelInfo, "dev", "ok\n2026-01-01 00:00:00 | ERROR | main | forged\r\nmore")
			if strings.Count(out, "\n") != 1 || strings.Contains(out, "\r") {
				t.Errorf("%s record must be a single line: %q", name, out)
			}
		})
	}
}

// Regression: the dampening table must stay bounded when messages carry variable content.
func TestDampeningTableIsBounded(t *testing.T) {
	cfg := config.DefaultSystemConfig()
	cfg.Logging.Dampening.Enabled = true
	logger.Setup(cfg)
	t.Cleanup(func() { logger.Setup(config.DefaultSystemConfig()) })
	logger.SetOutput(io.Discard)
	logger.SetDampening(true, 3, time.Hour)

	for i := 0; i < 5000; i++ {
		logger.Info("main", "Scrape cycle finished in %d ms", i)
	}
	if n := logger.Get().DampenEntries(); n > 1024 {
		t.Errorf("expected at most 1024 dampening entries, got %d", n)
	}

	// Dampening must keep working for a repeated message after evictions.
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	for i := 0; i < 10; i++ {
		logger.Warn("dev", "storm")
	}
	if got := strings.Count(buf.String(), "storm"); got != 3 {
		t.Errorf("expected 3 emitted 'storm' messages (threshold), got %d", got)
	}
}

type countingStringer struct{ n *int }

func (c countingStringer) String() string { *c.n++; return "x" }

// Disabled levels must return before formatting their arguments.
func TestDisabledLevelSkipsFormatting(t *testing.T) {
	cfg := config.DefaultSystemConfig()
	cfg.Logging.Level = "ERROR"
	logger.Setup(cfg)
	t.Cleanup(func() { logger.Setup(config.DefaultSystemConfig()) })
	logger.SetOutput(io.Discard)

	calls := 0
	arg := countingStringer{n: &calls}
	logger.Debug("dev", "%v", arg)
	logger.Info("dev", "%v", arg)
	logger.Warn("dev", "%v", arg)
	if calls != 0 {
		t.Errorf("expected 0 argument formatting calls for disabled levels, got %d", calls)
	}
	logger.Error("dev", "%v", arg)
	if calls != 1 {
		t.Errorf("expected enabled level to format once, got %d", calls)
	}
}
