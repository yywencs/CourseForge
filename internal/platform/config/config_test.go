package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestRepositoryYAMLConfigsDecode(t *testing.T) {
	paths := []string{
		"../../../configs/config.example.yaml",
		"../../../deploy/configs/config.yaml",
		"../../../deploy/benchmark/config.yaml",
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(filepath.Dir(path))+"/"+filepath.Base(path), func(t *testing.T) {
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read config: %v", err)
			}
			v := viper.New()
			v.SetConfigType("yaml")
			if err := v.ReadConfig(bytes.NewReader(body)); err != nil {
				t.Fatalf("parse config: %v", err)
			}
			var cfg Config
			if err := v.Unmarshal(&cfg); err != nil {
				t.Fatalf("decode config: %v", err)
			}
		})
	}
}

func TestInitViperConfigEnvironmentOverrides(t *testing.T) {
	tempDir := t.TempDir()
	configBody := []byte(`
auth:
  jwt:
    signing_key: file-signing-key
    issuer: courseforge
    audience: courseforge-student
    clock_skew: 30s
    token_ttl: 2h
data:
  mysql:
    dsn: file-dsn
    courseforge_dsn: file-courseforge-dsn
  redis:
    password: file-redis-password
asynq:
  redis:
    password: file-asynq-password
enrollment:
  selection_stream:
    group: file-selection-group
    concurrency: 2
    batch_size: 100
    batch_wait: 10ms
    block_timeout: 1s
    claim_idle: 30s
rabbitmq:
  username: file-user
  password: file-rabbitmq-password
  publisher:
    pool_size: 8
  listener:
    simple:
      prefetch: 1
      default_concurrency: 1
      max_retries: 3
      retry_delays: [1s, 5s, 30s]
      concurrency:
        outbox_events_queue: 8
      batch_size:
        outbox_events_queue: 100
      batch_wait:
        outbox_events_queue: 10ms
dcc:
  rate_limit:
    enabled: true
`)
	if err := os.WriteFile(filepath.Join(tempDir, "config.yaml"), configBody, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
		Conf = nil
	})

	t.Setenv("COURSEFORGE_DATA_MYSQL_DSN", "env-dsn")
	t.Setenv(
		"COURSEFORGE_AUTH_JWT_SIGNING_KEY",
		"environment-signing-key-with-at-least-32-bytes",
	)
	t.Setenv("COURSEFORGE_DATA_MYSQL_COURSEFORGE_DSN", "env-courseforge-dsn")
	t.Setenv("COURSEFORGE_DATA_REDIS_PASSWORD", "env-redis-password")
	t.Setenv("COURSEFORGE_ASYNQ_REDIS_PASSWORD", "env-asynq-password")
	t.Setenv("COURSEFORGE_ENROLLMENT_SELECTION_STREAM_BATCH_SIZE", "200")
	t.Setenv("COURSEFORGE_RABBITMQ_USERNAME", "env-user")
	t.Setenv("COURSEFORGE_RABBITMQ_PASSWORD", "env-rabbitmq-password")
	t.Setenv("COURSEFORGE_RABBITMQ_PUBLISHER_POOL_SIZE", "6")
	t.Setenv("COURSEFORGE_RABBITMQ_LISTENER_SIMPLE_DEFAULT_CONCURRENCY", "2")
	t.Setenv("COURSEFORGE_RABBITMQ_LISTENER_SIMPLE_CONCURRENCY_OUTBOX_EVENTS_QUEUE", "6")
	t.Setenv("COURSEFORGE_DCC_RATE_LIMIT_ENABLED", "false")

	InitViperConfig()

	if Conf.Auth.JWT.SigningKey != "environment-signing-key-with-at-least-32-bytes" {
		t.Fatalf("JWT signing key was not overridden by environment")
	}
	if err := Conf.Auth.JWT.Validate(); err != nil {
		t.Fatalf("JWT config validation failed: %v", err)
	}
	if Conf.Data.Database.Dsn != "env-dsn" {
		t.Fatalf("mysql dsn = %q, want environment override", Conf.Data.Database.Dsn)
	}
	if Conf.Data.Database.CourseForgeDSN != "env-courseforge-dsn" {
		t.Fatalf(
			"courseforge mysql dsn = %q, want environment override",
			Conf.Data.Database.CourseForgeDSN,
		)
	}
	if Conf.Data.Redis.Password != "env-redis-password" {
		t.Fatalf("redis password = %q, want environment override", Conf.Data.Redis.Password)
	}
	if Conf.Asynq.Redis.Password != "env-asynq-password" {
		t.Fatalf("asynq redis password = %q, want environment override", Conf.Asynq.Redis.Password)
	}
	if Conf.Enrollment.SelectionStream.Group != "file-selection-group" ||
		Conf.Enrollment.SelectionStream.BatchSize != 200 ||
		Conf.Enrollment.SelectionStream.BatchWait != 10*time.Millisecond {
		t.Fatalf(
			"selection stream config = %#v, want environment override",
			Conf.Enrollment.SelectionStream,
		)
	}
	if Conf.RabbitMQ.Username != "env-user" || Conf.RabbitMQ.Password != "env-rabbitmq-password" {
		t.Fatalf("rabbitmq credentials were not overridden by environment")
	}
	if Conf.RabbitMQ.Publisher.PoolSize != 6 {
		t.Fatalf("rabbitmq publisher pool size = %d, want 6", Conf.RabbitMQ.Publisher.PoolSize)
	}
	if Conf.RabbitMQ.Listener.Simple.Prefetch != 1 ||
		Conf.RabbitMQ.Listener.Simple.DefaultConcurrency != 2 ||
		Conf.RabbitMQ.Listener.Simple.MaxRetries != 3 ||
		len(Conf.RabbitMQ.Listener.Simple.RetryDelays) != 3 ||
		Conf.RabbitMQ.Listener.Simple.RetryDelays[1] != 5*time.Second ||
		Conf.RabbitMQ.Listener.Simple.Concurrency["outbox_events_queue"] != 6 ||
		Conf.RabbitMQ.Listener.Simple.BatchSize["outbox_events_queue"] != 100 ||
		Conf.RabbitMQ.Listener.Simple.BatchWait["outbox_events_queue"] != 10*time.Millisecond {
		t.Fatalf("rabbitmq listener config = %#v, want retry and concurrency settings",
			Conf.RabbitMQ.Listener.Simple)
	}
	if Conf.Dcc.RateLimit.Enabled {
		t.Fatal("rate limit enabled flag was not overridden by environment")
	}
}

func TestRateLimitConfigValidate(t *testing.T) {
	valid := RateLimitConfig{
		Enabled: true, EntryTTL: 10 * time.Minute, MaxEntries: 100,
		Login: LoginRateLimitConfig{
			Global:  validRateLimitPolicy(),
			IP:      validRateLimitPolicy(),
			Account: validRateLimitPolicy(),
		},
		Selection: SelectionRateLimitConfig{
			Global:  validRateLimitPolicy(),
			Student: validRateLimitPolicy(),
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	disabled := RateLimitConfig{}
	if err := disabled.Validate(); err != nil {
		t.Fatalf("disabled Validate() error = %v, want nil", err)
	}

	invalid := valid
	invalid.Login.Account.Window = 0
	if err := invalid.Validate(); err == nil ||
		!strings.Contains(err.Error(), "login.account.window") {
		t.Fatalf("invalid policy error = %v, want login.account.window", err)
	}
}

func TestObjectStorageConfigValidate(t *testing.T) {
	disabled := ObjectStorageConfig{}
	if err := disabled.Validate(); err != nil {
		t.Fatalf("disabled Validate() error = %v", err)
	}
	valid := ObjectStorageConfig{
		Enabled: true, Endpoint: "127.0.0.1:9000", AccessKey: "courseforge",
		SecretKey: "secret", Bucket: "courseforge", UploadURLTTL: time.Minute,
		PlaybackURLTTL: time.Minute, MaxVideoSizeBytes: 1024,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	valid.Bucket = ""
	if err := valid.Validate(); err == nil || !strings.Contains(err.Error(), "bucket") {
		t.Fatalf("missing bucket error = %v", err)
	}
}

func validRateLimitPolicy() RateLimitPolicyConfig {
	return RateLimitPolicyConfig{Requests: 10, Window: time.Second, Burst: 20}
}

func TestJWTAuthConfigResolvesAdministratorAudience(t *testing.T) {
	tests := []struct {
		name   string
		config JWTAuthConfig
		want   string
	}{
		{
			name: "explicit audience",
			config: JWTAuthConfig{
				Audience: "courseforge-student", AdministratorAudience: "custom-admin",
			},
			want: "custom-admin",
		},
		{
			name:   "derive from student audience",
			config: JWTAuthConfig{Audience: "courseforge-student"},
			want:   "courseforge-administrator",
		},
		{
			name:   "derive from generic audience",
			config: JWTAuthConfig{Audience: "courseforge"},
			want:   "courseforge-administrator",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if got := testCase.config.ResolvedAdministratorAudience(); got != testCase.want {
				t.Fatalf("ResolvedAdministratorAudience() = %q, want %q", got, testCase.want)
			}
		})
	}
}
