package main

import (
	"strings"
	"testing"
)

func TestExtractFacebookErrorMessage(t *testing.T) {
	tests := []struct {
		name       string
		payload    map[string]any
		statusCode int
		contains   string
	}{
		{
			name: "full error payload",
			payload: map[string]any{
				"error": map[string]any{
					"message":       "Invalid OAuth access token",
					"code":          float64(190),
					"error_subcode": float64(463),
				},
			},
			statusCode: 400,
			contains:   "Invalid OAuth access token",
		},
		{
			name:       "missing error object",
			payload:    map[string]any{},
			statusCode: 500,
			contains:   "status 500",
		},
		{
			name: "message only",
			payload: map[string]any{
				"error": map[string]any{
					"message": "Generic facebook error",
				},
			},
			statusCode: 400,
			contains:   "Generic facebook error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractFacebookErrorMessage(tc.payload, tc.statusCode)
			if got == "" {
				t.Fatalf("expected non-empty message")
			}
			if !strings.Contains(got, tc.contains) {
				t.Fatalf("expected %q to contain %q", got, tc.contains)
			}
		})
	}
}
