package main

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestParsePrepareConfig(t *testing.T) {
	t.Setenv(
		"COURSEFORGE_BENCHMARK_MYSQL_DSN",
		"root:password@tcp(localhost:3306)/courseforge?parseTime=true",
	)
	t.Setenv("COURSEFORGE_BENCHMARK_REDIS_ADDR", "localhost:6379")

	config, err := parsePrepareConfig([]string{
		"--confirm-reset",
		"--users", "2000",
		"--capacity", "500",
		"--credit-limit", "24.5",
		"--course-limit", "10",
		"--timeout", "90s",
	}, io.Discard)
	if err != nil {
		t.Fatalf("parsePrepareConfig() error = %v, want nil", err)
	}
	if config.Users != 2000 || config.Capacity != 500 ||
		config.CreditLimit != 24.5 || config.CourseLimit != 10 {
		t.Fatalf("config = %+v, want provided benchmark settings", config)
	}
	if config.Timeout != 90*time.Second || config.RedisAddr != "localhost:6379" {
		t.Fatalf("timeout/redis = %s/%q, want 90s/localhost:6379", config.Timeout, config.RedisAddr)
	}
}

func TestParsePrepareConfigRequiresExplicitConfirmation(t *testing.T) {
	t.Setenv(
		"COURSEFORGE_BENCHMARK_MYSQL_DSN",
		"root:password@tcp(localhost:3306)/courseforge?parseTime=true",
	)

	_, err := parsePrepareConfig(nil, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "--confirm-reset") {
		t.Fatalf("parsePrepareConfig() error = %v, want confirmation error", err)
	}
}

func TestParsePrepareConfigRejectsNonCourseForgeDSN(t *testing.T) {
	t.Setenv(
		"COURSEFORGE_BENCHMARK_MYSQL_DSN",
		"root:password@tcp(localhost:3306)/unrelated_database?parseTime=true",
	)
	_, err := parsePrepareConfig([]string{"--confirm-reset"}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "courseforge") {
		t.Fatalf("parsePrepareConfig() error = %v, want courseforge validation error", err)
	}
}

func TestRepeatedValues(t *testing.T) {
	if got, want := repeatedValues(3, "(?, ?)"), "(?, ?),(?, ?),(?, ?)"; got != want {
		t.Fatalf("repeatedValues() = %q, want %q", got, want)
	}
}
