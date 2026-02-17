package util

import (
	"log/slog"
	"os"
	"testing"
)

func TestGetLogLevelFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     slog.Level
	}{
		{
			name:     "debug level",
			envValue: "debug",
			want:     slog.LevelDebug,
		},
		{
			name:     "info level",
			envValue: "info",
			want:     slog.LevelInfo,
		},
		{
			name:     "warn level",
			envValue: "warn",
			want:     slog.LevelWarn,
		},
		{
			name:     "error level",
			envValue: "error",
			want:     slog.LevelError,
		},
		{
			name:     "uppercase DEBUG",
			envValue: "DEBUG",
			want:     slog.LevelDebug,
		},
		{
			name:     "uppercase INFO",
			envValue: "INFO",
			want:     slog.LevelInfo,
		},
		{
			name:     "uppercase WARN",
			envValue: "WARN",
			want:     slog.LevelWarn,
		},
		{
			name:     "uppercase ERROR",
			envValue: "ERROR",
			want:     slog.LevelError,
		},
		{
			name:     "mixed case Debug",
			envValue: "Debug",
			want:     slog.LevelDebug,
		},
		{
			name:     "mixed case Info",
			envValue: "Info",
			want:     slog.LevelInfo,
		},
		{
			name:     "empty string defaults to info",
			envValue: "",
			want:     slog.LevelInfo,
		},
		{
			name:     "unrecognized value defaults to info",
			envValue: "trace",
			want:     slog.LevelInfo,
		},
		{
			name:     "garbage value defaults to info",
			envValue: "notavalidlevel",
			want:     slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalValue, wasSet := os.LookupEnv("LOG_LEVEL")
			defer func() {
				if wasSet {
					os.Setenv("LOG_LEVEL", originalValue)
				} else {
					os.Unsetenv("LOG_LEVEL")
				}
			}()

			os.Setenv("LOG_LEVEL", tt.envValue)
			got := getLogLevelFromEnv()
			if got != tt.want {
				t.Errorf("getLogLevelFromEnv() with LOG_LEVEL=%q = %v, want %v", tt.envValue, got, tt.want)
			}
		})
	}
}

func TestGetLogLevelFromEnv_Unset(t *testing.T) {
	originalValue, wasSet := os.LookupEnv("LOG_LEVEL")
	defer func() {
		if wasSet {
			os.Setenv("LOG_LEVEL", originalValue)
		} else {
			os.Unsetenv("LOG_LEVEL")
		}
	}()

	os.Unsetenv("LOG_LEVEL")
	got := getLogLevelFromEnv()
	if got != slog.LevelInfo {
		t.Errorf("getLogLevelFromEnv() with LOG_LEVEL unset = %v, want %v", got, slog.LevelInfo)
	}
}
