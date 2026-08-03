package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
rabbitmq:
  username: file-user
  password: file-rabbitmq-password
  publisher:
    pool_size: 8
  topic:
    selection_result: file-selection-result
  listener:
    simple:
      prefetch: 1
      default_concurrency: 1
      concurrency:
        selection_result_queue: 8
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

	t.Setenv("PRIZEFORGE_DATA_MYSQL_DSN", "env-dsn")
	t.Setenv(
		"PRIZEFORGE_AUTH_JWT_SIGNING_KEY",
		"environment-signing-key-with-at-least-32-bytes",
	)
	t.Setenv("PRIZEFORGE_DATA_MYSQL_COURSEFORGE_DSN", "env-courseforge-dsn")
	t.Setenv("PRIZEFORGE_DATA_REDIS_PASSWORD", "env-redis-password")
	t.Setenv("PRIZEFORGE_ASYNQ_REDIS_PASSWORD", "env-asynq-password")
	t.Setenv("PRIZEFORGE_RABBITMQ_USERNAME", "env-user")
	t.Setenv("PRIZEFORGE_RABBITMQ_PASSWORD", "env-rabbitmq-password")
	t.Setenv("PRIZEFORGE_RABBITMQ_PUBLISHER_POOL_SIZE", "6")
	t.Setenv("PRIZEFORGE_RABBITMQ_TOPIC_SELECTION_RESULT", "env-selection-result")
	t.Setenv("PRIZEFORGE_RABBITMQ_LISTENER_SIMPLE_DEFAULT_CONCURRENCY", "2")
	t.Setenv("PRIZEFORGE_RABBITMQ_LISTENER_SIMPLE_CONCURRENCY_SELECTION_RESULT_QUEUE", "6")
	t.Setenv("PRIZEFORGE_DCC_RATE_LIMIT_ENABLED", "false")

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
	if Conf.Data.Database.CourseforgeDsn != "env-courseforge-dsn" {
		t.Fatalf(
			"courseforge mysql dsn = %q, want environment override",
			Conf.Data.Database.CourseforgeDsn,
		)
	}
	if Conf.Data.Redis.Password != "env-redis-password" {
		t.Fatalf("redis password = %q, want environment override", Conf.Data.Redis.Password)
	}
	if Conf.Asynq.Redis.Password != "env-asynq-password" {
		t.Fatalf("asynq redis password = %q, want environment override", Conf.Asynq.Redis.Password)
	}
	if Conf.RabbitMQ.Username != "env-user" || Conf.RabbitMQ.Password != "env-rabbitmq-password" {
		t.Fatalf("rabbitmq credentials were not overridden by environment")
	}
	if Conf.RabbitMQ.Publisher.PoolSize != 6 {
		t.Fatalf("rabbitmq publisher pool size = %d, want 6", Conf.RabbitMQ.Publisher.PoolSize)
	}
	if Conf.RabbitMQ.Topic.SelectionResult != "env-selection-result" {
		t.Fatalf("rabbitmq topic config = %#v, want environment overrides", Conf.RabbitMQ.Topic)
	}
	if Conf.RabbitMQ.Listener.Simple.Prefetch != 1 ||
		Conf.RabbitMQ.Listener.Simple.DefaultConcurrency != 2 ||
		Conf.RabbitMQ.Listener.Simple.Concurrency["selection_result_queue"] != 6 {
		t.Fatalf("rabbitmq listener config = %#v, want prefetch=1 default=2 selection=6",
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

func TestRabbitMQTopicConfigValidate(t *testing.T) {
	valid := RabbitMQTopicConfig{
		SelectionResult: "selection-result",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	missing := valid
	missing.SelectionResult = ""
	if err := missing.Validate(); err == nil || !strings.Contains(err.Error(), "selection_result is required") {
		t.Fatalf("missing topic Validate() error = %v, want selection_result required", err)
	}
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
