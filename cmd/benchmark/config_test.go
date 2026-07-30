package main

import (
	"io"
	"testing"
	"time"
)

func TestParseConfig(t *testing.T) {
	config, err := parseConfig([]string{
		"--url", "http://example.test:8080/",
		"--round-id", "9000000000101",
		"--teaching-class-id", "9000000000301",
		"--student-id-start", "9100000000000",
		"--users", "200",
		"--concurrency", "20",
		"--duration", "45s",
		"--timeout", "3s",
	}, io.Discard)
	if err != nil {
		t.Fatalf("parseConfig() error = %v, want nil", err)
	}

	if got, want := config.endpoint(), "http://example.test:8080"+selectionPath; got != want {
		t.Fatalf("endpoint() = %q, want %q", got, want)
	}
	if config.RoundID != 9_000_000_000_101 ||
		config.TeachingClassID != 9_000_000_000_301 ||
		config.StudentIDStart != 9_100_000_000_000 ||
		config.Users != 200 ||
		config.Concurrency != 20 {
		t.Fatalf("parsed numeric config = %+v, want provided values", config)
	}
	if config.Duration != 45*time.Second || config.Timeout != 3*time.Second {
		t.Fatalf("parsed durations = %s/%s, want 45s/3s", config.Duration, config.Timeout)
	}
}

func TestParseConfigRejectsInvalidValues(t *testing.T) {
	testCases := []struct {
		name string
		args []string
	}{
		{name: "invalid URL", args: []string{"--url", "127.0.0.1:8080"}},
		{name: "URL with path", args: []string{"--url", "http://127.0.0.1:8080/api"}},
		{name: "zero concurrency", args: []string{"--concurrency", "0"}},
		{name: "zero users", args: []string{"--users", "0"}},
		{name: "zero duration", args: []string{"--duration", "0s"}},
		{name: "zero round", args: []string{"--round-id", "0"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := parseConfig(testCase.args, io.Discard); err == nil {
				t.Fatal("parseConfig() error = nil, want validation error")
			}
		})
	}
}
