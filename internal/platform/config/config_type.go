package config

import (
	"fmt"
	"strings"
	"time"
)

// Config 对应整个 config.yaml 文件的根节点
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Data     DataConfig     `mapstructure:"data"`
	Log      LogConfig      `mapstructure:"log"`
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
	Asynq    AsynqConfig    `mapstructure:"asynq"`
	Dcc      DccConfig      `mapstructure:"dcc"`
}

type AuthConfig struct {
	JWT JWTAuthConfig `mapstructure:"jwt"`
}

type JWTAuthConfig struct {
	SigningKey            string        `mapstructure:"signing_key"`
	Issuer                string        `mapstructure:"issuer"`
	Audience              string        `mapstructure:"audience"`
	AdministratorAudience string        `mapstructure:"administrator_audience"`
	ClockSkew             time.Duration `mapstructure:"clock_skew"`
	TokenTTL              time.Duration `mapstructure:"token_ttl"`
}

// ResolvedAdministratorAudience 返回管理员服务使用的 JWT audience。
// 旧配置未声明该字段时，从学生 audience 派生，避免部署升级必须同步修改配置。
func (c JWTAuthConfig) ResolvedAdministratorAudience() string {
	if audience := strings.TrimSpace(c.AdministratorAudience); audience != "" {
		return audience
	}
	audience := strings.TrimSpace(c.Audience)
	if strings.HasSuffix(audience, "-student") {
		return strings.TrimSuffix(audience, "-student") + "-administrator"
	}
	return audience + "-administrator"
}

func (c JWTAuthConfig) Validate() error {
	if len(c.SigningKey) < 32 {
		return fmt.Errorf("auth.jwt.signing_key must contain at least 32 bytes")
	}
	if strings.TrimSpace(c.Issuer) == "" {
		return fmt.Errorf("auth.jwt.issuer is required")
	}
	if strings.TrimSpace(c.Audience) == "" {
		return fmt.Errorf("auth.jwt.audience is required")
	}
	if c.TokenTTL <= 0 {
		return fmt.Errorf("auth.jwt.token_ttl must be positive")
	}
	if c.ClockSkew < 0 {
		return fmt.Errorf("auth.jwt.clock_skew must not be negative")
	}
	return nil
}

// --- Server 部分 ---

type ServerConfig struct {
	API            HttpConfig `mapstructure:"api"`
	Admin          HttpConfig `mapstructure:"admin"`
	Http           HttpConfig `mapstructure:"http"`
	GRPC           HttpConfig `mapstructure:"grpc"`
	TrustedProxies []string   `mapstructure:"trusted_proxies"`
}

type HttpConfig struct {
	Addr    string `mapstructure:"addr"`
	Timeout string `mapstructure:"timeout"`
}

// --- Data 部分 ---

type DataConfig struct {
	Database      DatabaseConfig      `mapstructure:"mysql"`
	Redis         RedisConfig         `mapstructure:"redis"`
	Etcd          EtcdConfig          `mapstructure:"etcd"`
	ObjectStorage ObjectStorageConfig `mapstructure:"object_storage"`
}

type ObjectStorageConfig struct {
	Enabled           bool          `mapstructure:"enabled"`
	Endpoint          string        `mapstructure:"endpoint"`
	AccessKey         string        `mapstructure:"access_key"`
	SecretKey         string        `mapstructure:"secret_key"`
	Bucket            string        `mapstructure:"bucket"`
	Region            string        `mapstructure:"region"`
	UseSSL            bool          `mapstructure:"use_ssl"`
	UploadURLTTL      time.Duration `mapstructure:"upload_url_ttl"`
	PlaybackURLTTL    time.Duration `mapstructure:"playback_url_ttl"`
	MaxVideoSizeBytes int64         `mapstructure:"max_video_size_bytes"`
}

func (c ObjectStorageConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if strings.TrimSpace(c.Endpoint) == "" {
		return fmt.Errorf("data.object_storage.endpoint is required")
	}
	if strings.TrimSpace(c.AccessKey) == "" || strings.TrimSpace(c.SecretKey) == "" {
		return fmt.Errorf("data.object_storage credentials are required")
	}
	if strings.TrimSpace(c.Bucket) == "" {
		return fmt.Errorf("data.object_storage.bucket is required")
	}
	if c.UploadURLTTL <= 0 || c.PlaybackURLTTL <= 0 {
		return fmt.Errorf("data.object_storage URL TTLs must be positive")
	}
	if c.MaxVideoSizeBytes <= 0 {
		return fmt.Errorf("data.object_storage.max_video_size_bytes must be positive")
	}
	return nil
}

type DatabaseConfig struct {
	Dsn            string        `mapstructure:"dsn"`
	CourseforgeDsn string        `mapstructure:"courseforge_dsn"`
	MaxOpenConns   int           `mapstructure:"max_open_conns"`
	MaxIdleConns   int           `mapstructure:"max_idle_conns"`
	MaxLifeTime    time.Duration `mapstructure:"max_life_time"`
	MaxIdleTime    time.Duration `mapstructure:"max_idle_time"`
	DbCount        int           `mapstructure:"db_count"`
	TbCount        int           `mapstructure:"tb_count"`
}

type RedisConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	Password       string `mapstructure:"password"`
	DB             int    `mapstructure:"db"`
	PoolSize       int    `mapstructure:"pool_size"`
	MinIdleSize    int    `mapstructure:"min_idle_size"`
	IdleTimeout    int    `mapstructure:"idle_timeout"`
	ConnectTimeout int    `mapstructure:"connect_timeout"`
	RetryAttempts  int    `mapstructure:"retry_attempts"`
	RetryInterval  int    `mapstructure:"retry_interval"`
	PingInterval   int    `mapstructure:"ping_interval"`
	KeepAlive      bool   `mapstructure:"keep_alive"`
}

type EtcdConfig struct {
	Endpoints []string      `mapstructure:"endpoints"`
	Timeout   time.Duration `mapstructure:"timeout"`
}

// --- RabbitMQ 部分 ---

type RabbitMQConfig struct {
	Addresses string                  `mapstructure:"addresses"`
	Port      int                     `mapstructure:"port"`
	Username  string                  `mapstructure:"username"`
	Password  string                  `mapstructure:"password"`
	Publisher RabbitMQPublisherConfig `mapstructure:"publisher"`
	Listener  RabbitMQListener        `mapstructure:"listener"`
	Topic     RabbitMQTopicConfig     `mapstructure:"topic"`
}

type RabbitMQPublisherConfig struct {
	PoolSize int `mapstructure:"pool_size"`
}

type RabbitMQListener struct {
	Simple RabbitMQSimple `mapstructure:"simple"`
}

type RabbitMQSimple struct {
	Prefetch           int            `mapstructure:"prefetch"`
	DefaultConcurrency int            `mapstructure:"default_concurrency"`
	Concurrency        map[string]int `mapstructure:"concurrency"`
}

type RabbitMQTopicConfig struct {
	SelectionResult string `mapstructure:"selection_result"`
}

func (c RabbitMQTopicConfig) Validate() error {
	if strings.TrimSpace(c.SelectionResult) == "" {
		return fmt.Errorf("selection_result is required")
	}
	return nil
}

// --- Asynq 部分 ---

type AsynqConfig struct {
	Redis       RedisConfig `mapstructure:"redis"`
	Concurrency int         `mapstructure:"concurrency"`
}

// --- DCC 部分 ---

type DccConfig struct {
	RateLimit     RateLimitConfig `mapstructure:"rate_limit"`
	EnableDegrade bool            `mapstructure:"enable_degrade"`
	BlackList     string          `mapstructure:"black_list"`
}

type RateLimitConfig struct {
	Enabled    bool                     `mapstructure:"enabled"`
	EntryTTL   time.Duration            `mapstructure:"entry_ttl"`
	MaxEntries int                      `mapstructure:"max_entries"`
	Login      LoginRateLimitConfig     `mapstructure:"login"`
	Selection  SelectionRateLimitConfig `mapstructure:"selection"`
}

type LoginRateLimitConfig struct {
	Global  RateLimitPolicyConfig `mapstructure:"global"`
	IP      RateLimitPolicyConfig `mapstructure:"ip"`
	Account RateLimitPolicyConfig `mapstructure:"account"`
}

type SelectionRateLimitConfig struct {
	Global  RateLimitPolicyConfig `mapstructure:"global"`
	Student RateLimitPolicyConfig `mapstructure:"student"`
}

type RateLimitPolicyConfig struct {
	Requests int           `mapstructure:"requests"`
	Window   time.Duration `mapstructure:"window"`
	Burst    int           `mapstructure:"burst"`
}

func (c RateLimitConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.EntryTTL <= 0 {
		return fmt.Errorf("dcc.rate_limit.entry_ttl must be positive")
	}
	if c.MaxEntries <= 0 {
		return fmt.Errorf("dcc.rate_limit.max_entries must be positive")
	}
	policies := []struct {
		name   string
		policy RateLimitPolicyConfig
	}{
		{name: "login.global", policy: c.Login.Global},
		{name: "login.ip", policy: c.Login.IP},
		{name: "login.account", policy: c.Login.Account},
		{name: "selection.global", policy: c.Selection.Global},
		{name: "selection.student", policy: c.Selection.Student},
	}
	for _, item := range policies {
		if err := item.policy.validate(item.name); err != nil {
			return err
		}
	}
	return nil
}

func (c RateLimitPolicyConfig) validate(name string) error {
	if c.Requests <= 0 {
		return fmt.Errorf("dcc.rate_limit.%s.requests must be positive", name)
	}
	if c.Window <= 0 {
		return fmt.Errorf("dcc.rate_limit.%s.window must be positive", name)
	}
	if c.Burst <= 0 {
		return fmt.Errorf("dcc.rate_limit.%s.burst must be positive", name)
	}
	return nil
}

// --- Log 部分 ---

type LogConfig struct {
	Level      string `mapstructure:"level"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}
