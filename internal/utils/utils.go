// Package utils provides conversion and helper functions.
package utils

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// SafeFloat safely converts an arbitrary value to float64, returning defaultVal if conversion fails.
func SafeFloat(value any, defaultVal ...float64) float64 {
	def := 0.0
	if len(defaultVal) > 0 {
		def = defaultVal[0]
	}
	if value == nil {
		return def
	}

	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case uint:
		return float64(v)
	case uint64:
		return float64(v)
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f
		}
	case string:
		v = strings.TrimSpace(v)
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	case bool:
		if v {
			return 1.0
		}
		return 0.0
	}
	return def
}

// SafeInt safely converts an arbitrary value to int, returning defaultVal if conversion fails.
func SafeInt(value any, defaultVal ...int) int {
	def := 0
	if len(defaultVal) > 0 {
		def = defaultVal[0]
	}
	if value == nil {
		return def
	}

	f := SafeFloat(value, float64(def))
	return int(f)
}

// CleanString converts any value to a trimmed string or returns "Unknown" if nil or empty.
func CleanString(value any) string {
	if value == nil {
		return "Unknown"
	}
	s := strings.TrimSpace(fmt.Sprintf("%v", value))
	if s == "" || s == "<nil>" {
		return "Unknown"
	}
	return s
}

// DetectDeviceFamily determines whether device is "airfiber" or "airmax".
func DetectDeviceFamily(dataRoot any) string {
	rootMap, ok := dataRoot.(map[string]any)
	if !ok {
		return "unknown"
	}

	// Check device.identification.model
	if dev, ok := rootMap["device"].(map[string]any); ok {
		if ident, ok := dev["identification"].(map[string]any); ok {
			model := strings.ToLower(fmt.Sprintf("%v", ident["model"]))
			if strings.Contains(model, "airfiber") || strings.Contains(model, "af5xhd") || strings.Contains(model, "5xhd") {
				return "airfiber"
			}
		}
	}

	// Check wireless.radios[0].frequency.tx
	if wireless, ok := rootMap["wireless"].(map[string]any); ok {
		if radios, ok := wireless["radios"].([]any); ok && len(radios) > 0 {
			if r0, ok := radios[0].(map[string]any); ok {
				if freq, ok := r0["frequency"].(map[string]any); ok {
					if _, hasTx := freq["tx"]; hasTx {
						return "airfiber"
					}
				}
			}
		}
	}

	return "airmax"
}

// Truncate truncates a string to maxLen characters, appending an ellipsis if truncated.
func Truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		if maxLen <= 3 {
			return s[:maxLen]
		}
		return s[:maxLen-3] + "..."
	}
	return s
}
