package main

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	redis "github.com/redis/go-redis/v9"
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

type recordingSelectionPreheatPipeline struct {
	deleted []string
	values  map[string]interface{}
	ttls    map[string]time.Duration
}

func (p *recordingSelectionPreheatPipeline) Del(
	ctx context.Context,
	keys ...string,
) *redis.IntCmd {
	p.deleted = append(p.deleted, keys...)
	return redis.NewIntCmd(ctx)
}

func (p *recordingSelectionPreheatPipeline) Set(
	ctx context.Context,
	key string,
	value interface{},
	expiration time.Duration,
) *redis.StatusCmd {
	if p.values == nil {
		p.values = make(map[string]interface{})
		p.ttls = make(map[string]time.Duration)
	}
	p.values[key] = value
	p.ttls[key] = expiration
	return redis.NewStatusCmd(ctx)
}

func TestQueueSelectionPreheatUsesProductionKeys(t *testing.T) {
	pipe := &recordingSelectionPreheatPipeline{}
	queueSelectionPreheat(
		context.Background(),
		pipe,
		defaultBenchmarkRoundID,
		benchmarkTermID,
		benchmarkCourseID,
		defaultBenchmarkStudentIDStart,
		245,
		10,
	)

	wantDeleted := []string{
		"courseforge:selection:pending:9000000000101:9100000000000",
		"courseforge:selection:course:9000000000004:9100000000000:9000000000005",
	}
	if len(pipe.deleted) != len(wantDeleted) {
		t.Fatalf("deleted keys = %#v, want %#v", pipe.deleted, wantDeleted)
	}
	for index, key := range wantDeleted {
		if pipe.deleted[index] != key {
			t.Fatalf("deleted[%d] = %q, want %q", index, pipe.deleted[index], key)
		}
	}
	wantValues := map[string]interface{}{
		"courseforge:selection:quota:credit:9000000000101:9100000000000": int64(245),
		"courseforge:selection:quota:course:9000000000101:9100000000000": 10,
	}
	for key, value := range wantValues {
		if pipe.values[key] != value {
			t.Fatalf("key %q value = %#v, want %#v", key, pipe.values[key], value)
		}
		if pipe.ttls[key] != 0 {
			t.Fatalf("key %q TTL = %s, want no expiration", key, pipe.ttls[key])
		}
	}
}
