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

	"prizeforge/internal/infrastructure/adapter"
	"prizeforge/pkg/cache"
	"prizeforge/pkg/config"
	"prizeforge/pkg/logger"

	mysqlDriver "github.com/go-sql-driver/mysql"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const defaultIntegrationDSN = "root:prizeforge-integration@tcp(127.0.0.1:13306)/courseforge?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s"
const defaultIntegrationRedisAddr = "127.0.0.1:16379"
const defaultIntegrationRabbitMQAddr = "127.0.0.1:15673"
const defaultIntegrationRabbitMQUser = "prizeforge-integration"
const defaultIntegrationRabbitMQPassword = "prizeforge-integration"

var (
	integrationCourseforgeDB      *gorm.DB
	integrationRedis              *cache.Cache
	integrationRedisClient        *redis.Client
	integrationRabbitMQConfig     *config.RabbitMQConfig
	integrationRabbitMQConnection *amqp.Connection
)

// TestMain 连接由 compose.integration.yaml 创建的临时 CourseForge MySQL、Redis
// 和 RabbitMQ，并在全部集成测试结束后释放连接。
func TestMain(m *testing.M) {
	logger.Log = zap.NewNop()

	dsn := envOrDefault("PRIZEFORGE_INTEGRATION_MYSQL_DSN", defaultIntegrationDSN)
	if err := validateIntegrationDSN(dsn); err != nil {
		fmt.Fprintf(os.Stderr, "invalid integration MySQL DSN: %v\n", err)
		os.Exit(1)
	}
	redisAddr := envOrDefault("PRIZEFORGE_INTEGRATION_REDIS_ADDR", defaultIntegrationRedisAddr)
	if err := validateLocalAddress(redisAddr, "Redis"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rabbitMQAddr := envOrDefault("PRIZEFORGE_INTEGRATION_RABBITMQ_ADDR", defaultIntegrationRabbitMQAddr)
	rabbitMQHost, rabbitMQPort, err := validateRabbitMQAddress(rabbitMQAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	integrationCourseforgeDB, err = gorm.Open(gormMySQL.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open integration courseforge database: %v\n", err)
		os.Exit(1)
	}
	integrationRedisClient = redis.NewClient(&redis.Options{Addr: redisAddr})
	integrationRedis = cache.New(&cache.Options{Redis: integrationRedisClient})
	integrationRabbitMQConfig = &config.RabbitMQConfig{
		Addresses: rabbitMQHost,
		Port:      rabbitMQPort,
		Username:  envOrDefault("PRIZEFORGE_INTEGRATION_RABBITMQ_USER", defaultIntegrationRabbitMQUser),
		Password:  envOrDefault("PRIZEFORGE_INTEGRATION_RABBITMQ_PASSWORD", defaultIntegrationRabbitMQPassword),
		Topic: config.RabbitMQTopicConfig{
			SelectionResult: "selection_result",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if sqlDB, dbErr := integrationCourseforgeDB.DB(); dbErr != nil {
		cancel()
		fmt.Fprintf(os.Stderr, "get integration courseforge database: %v\n", dbErr)
		os.Exit(1)
	} else if pingErr := sqlDB.PingContext(ctx); pingErr != nil {
		cancel()
		fmt.Fprintf(os.Stderr, "ping integration courseforge database: %v\n", pingErr)
		os.Exit(1)
	}
	if err := integrationRedisClient.Ping(ctx).Err(); err != nil {
		cancel()
		fmt.Fprintf(os.Stderr, "ping integration Redis: %v\n", err)
		os.Exit(1)
	}
	integrationRabbitMQConnection, err = adapter.NewConnection(integrationRabbitMQConfig)
	cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect integration RabbitMQ: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	if sqlDB, dbErr := integrationCourseforgeDB.DB(); dbErr == nil {
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
