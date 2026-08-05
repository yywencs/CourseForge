package main

import (
	"io"
	"testing"
	"time"
)

const testJWTKey = "benchmark-test-signing-key-at-least-32-bytes"

func TestParseConfig(t *testing.T) {
	cfg, err := parseConfig([]string{
		"--targets", "http://127.0.0.1:8080,https://example.com",
		"--video-id", "42", "--clients", "10", "--publishers", "2",
		"--jwt-signing-key", testJWTKey,
	}, io.Discard)
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}
	if len(cfg.Targets) != 2 || cfg.VideoID != 42 || cfg.Clients != 10 || cfg.Publishers != 2 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if got := cfg.websocketURL(1); got != "wss://example.com/api/v1/course-videos/42/danmakus/realtime" {
		t.Fatalf("websocketURL() = %q", got)
	}
}

func TestRunStateExcludesWarmupAndEnd(t *testing.T) {
	base := time.Unix(100, 0)
	state := &runState{}
	state.start(base)
	if state.shouldRecord(base.Add(-time.Nanosecond)) || !state.shouldRecord(base) {
		t.Fatal("measurement start boundary is incorrect")
	}
	state.end(base.Add(time.Second))
	if state.shouldRecord(base.Add(time.Second)) {
		t.Fatal("measurement end must be exclusive")
	}
}
