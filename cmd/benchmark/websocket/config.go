package main

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type benchmarkConfig struct {
	Targets       []string
	VideoID       uint64
	Clients       int
	Publishers    int
	PublishEvery  time.Duration
	RampUpEvery   time.Duration
	Warmup        time.Duration
	Duration      time.Duration
	Drain         time.Duration
	Timeout       time.Duration
	StudentIDBase uint64
	JWTSigningKey string
	JWTIssuer     string
	JWTAudience   string
	JWTTokenTTL   time.Duration
	ResultRoot    string
}

func parseConfig(args []string, output io.Writer) (benchmarkConfig, error) {
	var targets string
	cfg := benchmarkConfig{}
	flags := flag.NewFlagSet("websocket-benchmark", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&targets, "targets", "http://127.0.0.1:8080", "逗号分隔的 API 实例根地址")
	flags.Uint64Var(&cfg.VideoID, "video-id", 0, "可播放的预览视频 ID")
	flags.IntVar(&cfg.Clients, "clients", 1000, "WebSocket 客户端数")
	flags.IntVar(&cfg.Publishers, "publishers", 4, "HTTP 弹幕发布协程数")
	flags.DurationVar(&cfg.PublishEvery, "publish-every", 100*time.Millisecond, "每个发布协程的发送间隔")
	flags.DurationVar(&cfg.RampUpEvery, "ramp-up-every", 2*time.Millisecond, "相邻客户端建连间隔")
	flags.DurationVar(&cfg.Warmup, "warmup", 10*time.Second, "不计入结果的预热时长")
	flags.DurationVar(&cfg.Duration, "duration", time.Minute, "正式测量时长")
	flags.DurationVar(&cfg.Drain, "drain", 5*time.Second, "停止发布后的排空时长")
	flags.DurationVar(&cfg.Timeout, "timeout", 5*time.Second, "建连和 HTTP 请求超时")
	flags.Uint64Var(&cfg.StudentIDBase, "student-id-start", 9_100_000_000_000, "压测学生起始 ID")
	flags.StringVar(&cfg.JWTSigningKey, "jwt-signing-key", os.Getenv("COURSEFORGE_BENCHMARK_JWT_SIGNING_KEY"), "学生 JWT HMAC 密钥")
	flags.StringVar(&cfg.JWTIssuer, "jwt-issuer", "courseforge", "学生 JWT issuer")
	flags.StringVar(&cfg.JWTAudience, "jwt-audience", "courseforge-student", "学生 JWT audience")
	flags.DurationVar(&cfg.JWTTokenTTL, "jwt-token-ttl", 2*time.Hour, "学生 JWT 有效期")
	flags.StringVar(&cfg.ResultRoot, "result-root", "benchmark-results/websocket", "结果输出根目录")
	if err := flags.Parse(args); err != nil {
		return benchmarkConfig{}, err
	}
	if flags.NArg() != 0 {
		return benchmarkConfig{}, fmt.Errorf("不支持位置参数: %s", strings.Join(flags.Args(), " "))
	}
	for _, target := range strings.Split(targets, ",") {
		if target = strings.TrimSpace(target); target != "" {
			cfg.Targets = append(cfg.Targets, strings.TrimRight(target, "/"))
		}
	}
	if err := cfg.validate(); err != nil {
		return benchmarkConfig{}, err
	}
	return cfg, nil
}

func (c benchmarkConfig) validate() error {
	if len(c.Targets) == 0 {
		return fmt.Errorf("targets 至少包含一个地址")
	}
	for _, target := range c.Targets {
		parsed, err := url.Parse(target)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || (parsed.Path != "" && parsed.Path != "/") {
			return fmt.Errorf("target 必须是 HTTP(S) 服务根地址: %q", target)
		}
	}
	if c.VideoID == 0 || c.StudentIDBase == 0 {
		return fmt.Errorf("video-id 和 student-id-start 必须大于 0")
	}
	if c.Clients <= 0 || c.Publishers <= 0 {
		return fmt.Errorf("clients 和 publishers 必须大于 0")
	}
	if c.PublishEvery <= 0 || c.RampUpEvery < 0 || c.Warmup < 0 || c.Duration <= 0 || c.Drain < 0 || c.Timeout <= 0 {
		return fmt.Errorf("时长参数不合法")
	}
	if len(c.JWTSigningKey) < 32 || strings.TrimSpace(c.JWTIssuer) == "" || strings.TrimSpace(c.JWTAudience) == "" || c.JWTTokenTTL <= 0 {
		return fmt.Errorf("JWT 配置不合法，签名密钥至少需要 32 字节")
	}
	if strings.TrimSpace(c.ResultRoot) == "" {
		return fmt.Errorf("result-root 不能为空")
	}
	return nil
}

func (c benchmarkConfig) websocketURL(clientIndex int) string {
	parsed, _ := url.Parse(c.Targets[clientIndex%len(c.Targets)])
	if parsed.Scheme == "https" {
		parsed.Scheme = "wss"
	} else {
		parsed.Scheme = "ws"
	}
	parsed.Path = "/api/v1/course-videos/" + strconv.FormatUint(c.VideoID, 10) + "/danmakus/realtime"
	return parsed.String()
}

func (c benchmarkConfig) publishURL(workerIndex int) string {
	return c.Targets[workerIndex%len(c.Targets)] + "/api/v1/course-videos/" + strconv.FormatUint(c.VideoID, 10) + "/danmakus"
}
