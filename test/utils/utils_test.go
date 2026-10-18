package utils_test

import (
	"encoding/json"
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
		{int32(32), 32.0},
		{int64(64), 64.0},
		{uint(7), 7.0},
		{uint64(8), 8.0},
		{float32(1.5), 1.5},
		{3.14, 3.14},
		{json.Number("45.6"), 45.6},
		{json.Number("bad"), 0.0},
		{"123.45", 123.45},
		{"invalid", 0.0},
		{true, 1.0},
		{false, 0.0},
		{struct{}{}, 0.0},
	}

	for _, tt := range tests {
		got := utils.SafeFloat(tt.input)
		if got != tt.expected {
			t.Errorf("SafeFloat(%v) = %v, expected %v", tt.input, got, tt.expected)
		}
	}

	if got := utils.SafeFloat(nil, 9.5); got != 9.5 {
		t.Errorf("SafeFloat(nil, 9.5) = %v, expected 9.5", got)
	}
}

func TestSafeInt(t *testing.T) {
	if utils.SafeInt("10") != 10 {
		t.Errorf("SafeInt(\"10\") != 10")
	}
	if utils.SafeInt(nil, 5) != 5 {
		t.Errorf("SafeInt(nil, 5) != 5")
	}
	if utils.SafeInt(12, 5) != 12 {
		t.Errorf("SafeInt(12, 5) != 12")
	}
}

func TestCleanString(t *testing.T) {
	if utils.CleanString(nil) != "Unknown" {
		t.Errorf("CleanString(nil) should be Unknown")
	}
	if utils.CleanString("") != "Unknown" || utils.CleanString("<nil>") != "Unknown" {
		t.Errorf("CleanString empty/<nil> should be Unknown")
	}
	if utils.CleanString("  hello  ") != "hello" {
		t.Errorf("CleanString should trim whitespace")
	}
}

func TestDetectDeviceFamily(t *testing.T) {
	if utils.DetectDeviceFamily("not-a-map") != "unknown" {
		t.Errorf("expected unknown for non-map")
	}

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

	afRadioData := map[string]any{
		"wireless": map[string]any{
			"radios": []any{
				map[string]any{
					"frequency": map[string]any{"tx": 5840.0},
				},
			},
		},
	}
	if utils.DetectDeviceFamily(afRadioData) != "airfiber" {
		t.Errorf("expected airfiber from wireless.radios[0].frequency.tx")
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
