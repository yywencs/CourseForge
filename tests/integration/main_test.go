//go:build integration

package integration

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/platform/cache"
	"github.com/yywencs/courseforge/internal/platform/config"
	"github.com/yywencs/courseforge/internal/platform/observability/logger"
	"github.com/yywencs/courseforge/internal/platform/rabbitmq"

	mysqlDriver "github.com/go-sql-driver/mysql"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const defaultIntegrationDSN = "root:courseforge-integration@tcp(127.0.0.1:13306)/courseforge?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s"
const defaultIntegrationRedisAddr = "127.0.0.1:16379"
const defaultIntegrationRabbitMQAddr = "127.0.0.1:15673"
const defaultIntegrationRabbitMQUser = "courseforge-integration"
const defaultIntegrationRabbitMQPassword = "courseforge-integration"

var (
	integrationCourseForgeDB      *gorm.DB
	integrationRedis              *cache.Cache
	integrationRedisClient        *redis.Client
	integrationRabbitMQConfig     *config.RabbitMQConfig
	integrationRabbitMQConnection *amqp.Connection
)

// TestMain 连接由 compose.integration.yaml 创建的临时 CourseForge MySQL、Redis
// 和 RabbitMQ，并在全部集成测试结束后释放连接。
func TestMain(m *testing.M) {
	logger.Log = zap.NewNop()

	dsn := envOrDefault("COURSEFORGE_INTEGRATION_MYSQL_DSN", defaultIntegrationDSN)
	if err := validateIntegrationDSN(dsn); err != nil {
		fmt.Fprintf(os.Stderr, "invalid integration MySQL DSN: %v\n", err)
		os.Exit(1)
	}
	redisAddr := envOrDefault("COURSEFORGE_INTEGRATION_REDIS_ADDR", defaultIntegrationRedisAddr)
	if err := validateLocalAddress(redisAddr, "Redis"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rabbitMQAddr := envOrDefault("COURSEFORGE_INTEGRATION_RABBITMQ_ADDR", defaultIntegrationRabbitMQAddr)
	rabbitMQHost, rabbitMQPort, err := validateRabbitMQAddress(rabbitMQAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	integrationCourseForgeDB, err = gorm.Open(gormMySQL.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open integration courseforge database: %v\n", err)
		os.Exit(1)
	}
	// 与应用默认配置保持一致，避免并发用例为每个 goroutine 同时新建连接，
	// 从而让 Docker Desktop/OrbStack 的端口转发层成为测试瓶颈。
	integrationSQLDB, err := integrationCourseForgeDB.DB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "get integration courseforge database: %v\n", err)
		os.Exit(1)
	}
	integrationSQLDB.SetMaxOpenConns(25)
	integrationSQLDB.SetMaxIdleConns(15)
	// 限制连接并放宽容器端口转发环境下的读写等待时间；默认按 CPU 数扩大的
	// Redis 连接池会在 100 路并发用例中同时穿过 Docker 代理并触发 3 秒读超时。
	integrationRedisClient = redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		PoolSize:     10,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		PoolTimeout:  10 * time.Second,
	})
	integrationRedis = cache.New(&cache.Options{Redis: integrationRedisClient})
	integrationRabbitMQConfig = &config.RabbitMQConfig{
		Addresses: rabbitMQHost,
		Port:      rabbitMQPort,
		Username:  envOrDefault("COURSEFORGE_INTEGRATION_RABBITMQ_USER", defaultIntegrationRabbitMQUser),
		Password:  envOrDefault("COURSEFORGE_INTEGRATION_RABBITMQ_PASSWORD", defaultIntegrationRabbitMQPassword),
		Topic: config.RabbitMQTopicConfig{
			SelectionResult: "selection_result",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if pingErr := integrationSQLDB.PingContext(ctx); pingErr != nil {
		cancel()
		fmt.Fprintf(os.Stderr, "ping integration courseforge database: %v\n", pingErr)
		os.Exit(1)
	}
	if err := integrationRedisClient.Ping(ctx).Err(); err != nil {
		cancel()
		fmt.Fprintf(os.Stderr, "ping integration Redis: %v\n", err)
		os.Exit(1)
	}
	integrationRabbitMQConnection, err = rabbitmq.NewConnection(integrationRabbitMQConfig)
	cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect integration RabbitMQ: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	if sqlDB, dbErr := integrationCourseForgeDB.DB(); dbErr == nil {
		_ = sqlDB.Close()
	}
	_ = integrationRabbitMQConnection.Close()
	_ = integrationRedisClient.Close()
	os.Exit(code)
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func validateIntegrationDSN(dsn string) error {
	parsed, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		return err
	}
	if parsed.DBName != "courseforge" {
		return fmt.Errorf("database must be courseforge, got %q", parsed.DBName)
	}
	return validateLocalAddress(parsed.Addr, "MySQL")
}

func validateLocalAddress(addr, dependency string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("parse %s address %q: %w", dependency, addr, err)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return fmt.Errorf("refusing non-local %s host %q", dependency, host)
	}
	return nil
}

func validateRabbitMQAddress(addr string) (string, int, error) {
	if err := validateLocalAddress(addr, "RabbitMQ"); err != nil {
		return "", 0, err
	}
	host, portValue, _ := net.SplitHostPort(addr)
	port, err := strconv.Atoi(portValue)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("invalid RabbitMQ port %q", portValue)
	}
	return host, port, nil
}
