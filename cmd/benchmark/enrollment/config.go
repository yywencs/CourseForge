package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

const selectionPath = "/api/v1/enrollments"
const waitlistPath = "/api/v1/enrollments/waitlist"

type benchmarkScenario string

const (
	scenarioSelection   benchmarkScenario = "selection"
	scenarioIdempotency benchmarkScenario = "idempotency"
	scenarioWaitlist    benchmarkScenario = "waitlist"
)

type benchmarkConfig struct {
	BaseURL         string
	Scenario        benchmarkScenario
	RoundID         uint64
	TeachingClassID uint64
	StudentIDStart  uint64
	Users           int
	Concurrency     int
	Duration        time.Duration
	Timeout         time.Duration
	JWTSigningKey   string
	JWTIssuer       string
	JWTAudience     string
	JWTTokenTTL     time.Duration
	Verify          bool
	MySQLDSN        string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	VerifyTimeout   time.Duration
	VerifyInterval  time.Duration
}

func parseConfig(args []string, output io.Writer) (benchmarkConfig, error) {
	cfg := benchmarkConfig{}
	flags := flag.NewFlagSet("benchmark", flag.ContinueOnError)
	flags.SetOutput(output)

	flags.StringVar(&cfg.BaseURL, "url", "http://127.0.0.1:8080", "API 服务根地址")
	flags.Func("scenario", "压测场景：selection/idempotency/waitlist", func(value string) error {
		cfg.Scenario = benchmarkScenario(value)
		return nil
	})
	flags.Uint64Var(&cfg.RoundID, "round-id", defaultBenchmarkRoundID, "选课轮次 ID")
	flags.Uint64Var(&cfg.TeachingClassID, "teaching-class-id", defaultBenchmarkClassID, "教学班 ID")
	flags.Uint64Var(&cfg.StudentIDStart, "student-id-start", defaultBenchmarkStudentIDStart, "压测学生起始 ID")
	flags.IntVar(&cfg.Users, "users", 1000, "压测学生池大小")
	flags.IntVar(&cfg.Concurrency, "concurrency", 10, "并发工作协程数")
	flags.DurationVar(&cfg.Duration, "duration", 30*time.Second, "整轮压测最大时长")
	flags.DurationVar(&cfg.Timeout, "timeout", 5*time.Second, "单个 HTTP 请求超时")
	flags.StringVar(
		&cfg.JWTSigningKey,
		"jwt-signing-key",
		os.Getenv("COURSEFORGE_BENCHMARK_JWT_SIGNING_KEY"),
		"学生 JWT HMAC 密钥，也可使用 COURSEFORGE_BENCHMARK_JWT_SIGNING_KEY",
	)
	flags.StringVar(&cfg.JWTIssuer, "jwt-issuer", "courseforge", "学生 JWT issuer")
	flags.StringVar(&cfg.JWTAudience, "jwt-audience", "courseforge-student", "学生 JWT audience")
	flags.DurationVar(&cfg.JWTTokenTTL, "jwt-token-ttl", 2*time.Hour, "压测学生 JWT 有效期")
	flags.BoolVar(&cfg.Verify, "verify", true, "压测后校验 MySQL 和 Redis 最终状态")
	flags.StringVar(
		&cfg.MySQLDSN,
		"mysql-dsn",
		os.Getenv("COURSEFORGE_BENCHMARK_MYSQL_DSN"),
		"最终校验使用的 courseforge MySQL DSN",
	)
	flags.StringVar(
		&cfg.RedisAddr,
		"redis-addr",
		envOrDefault("COURSEFORGE_BENCHMARK_REDIS_ADDR", "127.0.0.1:6379"),
		"最终校验使用的 Redis 地址",
	)
	flags.StringVar(
		&cfg.RedisPassword,
		"redis-password",
		os.Getenv("COURSEFORGE_BENCHMARK_REDIS_PASSWORD"),
		"最终校验使用的 Redis 密码",
	)
	flags.IntVar(&cfg.RedisDB, "redis-db", 0, "最终校验使用的 Redis DB")
	flags.DurationVar(&cfg.VerifyTimeout, "verify-timeout", 30*time.Second, "等待最终状态收敛的最长时间")
	flags.DurationVar(&cfg.VerifyInterval, "verify-interval", 200*time.Millisecond, "最终状态轮询间隔")

	if err := flags.Parse(args); err != nil {
		return benchmarkConfig{}, err
	}
	if flags.NArg() != 0 {
		return benchmarkConfig{}, fmt.Errorf("不支持位置参数: %s", strings.Join(flags.Args(), " "))
	}
	if err := cfg.validate(); err != nil {
		return benchmarkConfig{}, err
	}
	return cfg, nil
}

func (c benchmarkConfig) validate() error {
	parsedURL, err := url.Parse(c.BaseURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("url 必须是有效的 HTTP(S) 地址: %q", c.BaseURL)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("url 只支持 http 或 https: %q", c.BaseURL)
	}
	if parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return fmt.Errorf("url 不能包含 query 或 fragment: %q", c.BaseURL)
	}
	if parsedURL.Path != "" && parsedURL.Path != "/" {
		return fmt.Errorf("url 应为服务根地址，不能包含路径: %q", c.BaseURL)
	}
	if c.normalizedScenario() != scenarioSelection &&
		c.normalizedScenario() != scenarioIdempotency &&
		c.normalizedScenario() != scenarioWaitlist {
		return fmt.Errorf("scenario 仅支持 selection、idempotency 或 waitlist")
	}
	if c.RoundID == 0 || c.TeachingClassID == 0 || c.StudentIDStart == 0 {
		return fmt.Errorf("round-id、teaching-class-id 和 student-id-start 必须大于 0")
	}
	if c.Users <= 0 {
		return fmt.Errorf("users 必须大于 0")
	}
	if uint64(c.Users-1) > math.MaxUint64-c.StudentIDStart {
		return fmt.Errorf("student-id-start 与 users 组合发生溢出")
	}
	if c.Concurrency <= 0 {
		return fmt.Errorf("concurrency 必须大于 0")
	}
	if c.Duration <= 0 {
		return fmt.Errorf("duration 必须大于 0")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout 必须大于 0")
	}
	if len(c.JWTSigningKey) < 32 {
		return fmt.Errorf("jwt-signing-key 必须至少包含 32 个字节")
	}
	if strings.TrimSpace(c.JWTIssuer) == "" || strings.TrimSpace(c.JWTAudience) == "" {
		return fmt.Errorf("jwt-issuer 和 jwt-audience 不能为空")
	}
	if c.JWTTokenTTL <= 0 {
		return fmt.Errorf("jwt-token-ttl 必须大于 0")
	}
	if !c.Verify {
		return nil
	}
	parsedDSN, err := mysqlDriver.ParseDSN(strings.TrimSpace(c.MySQLDSN))
	if err != nil {
		return fmt.Errorf("mysql-dsn 无效: %w", err)
	}
	if parsedDSN.DBName != "courseforge" {
		return fmt.Errorf("mysql-dsn 必须连接 courseforge，当前数据库为 %q", parsedDSN.DBName)
	}
	if c.normalizedScenario() != scenarioWaitlist && strings.TrimSpace(c.RedisAddr) == "" {
		return fmt.Errorf("redis-addr 不能为空")
	}
	if c.RedisDB < 0 {
		return fmt.Errorf("redis-db 不能小于 0")
	}
	if c.VerifyTimeout <= 0 {
		return fmt.Errorf("verify-timeout 必须大于 0")
	}
	if c.VerifyInterval <= 0 || c.VerifyInterval > c.VerifyTimeout {
		return fmt.Errorf("verify-interval 必须大于 0 且不超过 verify-timeout")
	}
	return nil
}

func (c benchmarkConfig) endpoint() string {
	path := selectionPath
	if c.normalizedScenario() == scenarioWaitlist {
		path = waitlistPath
	}
	return strings.TrimRight(c.BaseURL, "/") + path
}

func (c benchmarkConfig) normalizedScenario() benchmarkScenario {
	if c.Scenario == "" {
		return scenarioSelection
	}
	return c.Scenario
}

func (c benchmarkConfig) studentID(sequence uint64) uint64 {
	return c.StudentIDStart + (sequence-1)%uint64(c.Users)
}
