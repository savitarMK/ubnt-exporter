package utils_test

import (
	"testing"

	"ubnt-exporter/internal/utils"
)

func TestSafeFloat(t *testing.T) {
	tests := []struct {
		input    any
		expected float64
	}{
		{nil, 0.0},
		{42, 42.0},
		{3.14, 3.14},
		{"123.45", 123.45},
		{"invalid", 0.0},
		{true, 1.0},
		{false, 0.0},
	}

	for _, tt := range tests {
		got := utils.SafeFloat(tt.input)
		if got != tt.expected {
			t.Errorf("SafeFloat(%v) = %v, expected %v", tt.input, got, tt.expected)
		}
	}
}

func TestSafeInt(t *testing.T) {
	if utils.SafeInt("10") != 10 {
		t.Errorf("SafeInt(\"10\") != 10")
	}
	if utils.SafeInt(nil, 5) != 5 {
		t.Errorf("SafeInt(nil, 5) != 5")
	}
}

func TestCleanString(t *testing.T) {
	if utils.CleanString(nil) != "Unknown" {
		t.Errorf("CleanString(nil) should be Unknown")
	}
	if utils.CleanString("  hello  ") != "hello" {
		t.Errorf("CleanString should trim whitespace")
	}
}

func TestDetectDeviceFamily(t *testing.T) {
	afData := map[string]any{
		"device": map[string]any{
			"identification": map[string]any{
				"model": "airFiber 5XHD",
			},
		},
	}
	if utils.DetectDeviceFamily(afData) != "airfiber" {
		t.Errorf("expected airfiber")
	}

	amData := map[string]any{
		"device": map[string]any{
			"identification": map[string]any{
				"model": "NanoStation 5AC",
			},
		},
	}
	if utils.DetectDeviceFamily(amData) != "airmax" {
		t.Errorf("expected airmax")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"exact", 5, "exact"},
		{"abcdefghij", 7, "abcd..."},
		{"abc", 2, "ab"},
	}

	for _, tc := range tests {
		got := utils.Truncate(tc.input, tc.maxLen)
		if got != tc.expected {
			t.Errorf("Truncate(%q, %d) = %q, expected %q", tc.input, tc.maxLen, got, tc.expected)
		}
	}
}
