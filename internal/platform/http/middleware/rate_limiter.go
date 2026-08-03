package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"prizeforge/internal/platform/config"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const unknownRateLimitKey = "unknown"

type keyedBucketEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// localKeyedLimiter 为每个业务 key 保存独立令牌桶，并通过空闲过期和容量上限
// 防止随机 IP、账号或用户 ID 造成无界内存增长。
type localKeyedLimiter struct {
	mu          sync.Mutex
	policy      config.RateLimitPolicyConfig
	entryTTL    time.Duration
	maxEntries  int
	lastCleanup time.Time
	entries     map[string]*keyedBucketEntry
}

func newLocalKeyedLimiter(
	policy config.RateLimitPolicyConfig,
	entryTTL time.Duration,
	maxEntries int,
) *localKeyedLimiter {
	return &localKeyedLimiter{
		policy:     policy,
		entryTTL:   entryTTL,
		maxEntries: maxEntries,
		entries:    make(map[string]*keyedBucketEntry),
	}
}

func (l *localKeyedLimiter) allowAt(key string, now time.Time) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		key = unknownRateLimitKey
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.lastCleanup.IsZero() || now.Sub(l.lastCleanup) >= l.entryTTL {
		l.cleanupExpiredLocked(now)
	}

	entry, ok := l.entries[key]
	if !ok {
		if len(l.entries) >= l.maxEntries {
			l.cleanupExpiredLocked(now)
			if len(l.entries) >= l.maxEntries {
				return false
			}
		}
		entry = &keyedBucketEntry{
			limiter: newTokenBucket(l.policy),
		}
		l.entries[key] = entry
	}
	entry.lastSeen = now
	return entry.limiter.AllowN(now, 1)
}

func (l *localKeyedLimiter) cleanupExpiredLocked(now time.Time) {
	cutoff := now.Add(-l.entryTTL)
	for key, entry := range l.entries {
		if entry.lastSeen.Before(cutoff) {
			delete(l.entries, key)
		}
	}
	l.lastCleanup = now
}

func newTokenBucket(policy config.RateLimitPolicyConfig) *rate.Limiter {
	requestsPerSecond := float64(policy.Requests) / policy.Window.Seconds()
	return rate.NewLimiter(rate.Limit(requestsPerSecond), policy.Burst)
}

// LoginRateLimiter 分别限制登录入口的总流量、来源 IP 和目标账号。
// 三个维度使用独立令牌桶，避免只更换账号或只更换 IP 就绕过限制。
type LoginRateLimiter struct {
	enabled bool
	now     func() time.Time
	global  *rate.Limiter
	ip      *localKeyedLimiter
	account *localKeyedLimiter
}

func NewLoginRateLimiter(cfg config.RateLimitConfig) *LoginRateLimiter {
	limiter := &LoginRateLimiter{enabled: cfg.Enabled, now: time.Now}
	if !cfg.Enabled {
		return limiter
	}
	limiter.global = newTokenBucket(cfg.Login.Global)
	limiter.ip = newLocalKeyedLimiter(cfg.Login.IP, cfg.EntryTTL, cfg.MaxEntries)
	limiter.account = newLocalKeyedLimiter(
		cfg.Login.Account,
		cfg.EntryTTL,
		cfg.MaxEntries,
	)
	return limiter
}

func (l *LoginRateLimiter) Allow(clientIP string, account string) bool {
	return l.AllowSource(clientIP) && l.AllowAccount(account)
}

// AllowSource 在解析请求体之前限制登录接口总流量和来源 IP。
func (l *LoginRateLimiter) AllowSource(clientIP string) bool {
	if l == nil || !l.enabled {
		return true
	}
	now := l.now()
	if !l.global.AllowN(now, 1) {
		return false
	}
	if !l.ip.allowAt(clientIP, now) {
		return false
	}
	return true
}

// AllowAccount 在请求体解析成功后限制目标账号。
func (l *LoginRateLimiter) AllowAccount(account string) bool {
	if l == nil || !l.enabled {
		return true
	}
	now := l.now()
	account = strings.ToLower(strings.TrimSpace(account))
	return l.account.allowAt(account, now)
}

// SelectionRateLimiter 同时保护整个提交选课入口和单个学生。
type SelectionRateLimiter struct {
	enabled bool
	now     func() time.Time
	global  *rate.Limiter
	student *localKeyedLimiter
}

func NewSelectionRateLimiter(cfg config.RateLimitConfig) *SelectionRateLimiter {
	limiter := &SelectionRateLimiter{enabled: cfg.Enabled, now: time.Now}
	if !cfg.Enabled {
		return limiter
	}
	limiter.global = newTokenBucket(cfg.Selection.Global)
	limiter.student = newLocalKeyedLimiter(
		cfg.Selection.Student,
		cfg.EntryTTL,
		cfg.MaxEntries,
	)
	return limiter
}

func (l *SelectionRateLimiter) Allow(studentID uint64) bool {
	if l == nil || !l.enabled {
		return true
	}
	if studentID == 0 {
		return false
	}
	now := l.now()
	if !l.global.AllowN(now, 1) {
		return false
	}
	return l.student.allowAt(strconv.FormatUint(studentID, 10), now)
}

// Handle 必须安装在 JWT 鉴权中间件之后，从已验签的 subject 获取学生 ID。
func (l *SelectionRateLimiter) Handle(c *gin.Context) {
	if l == nil || !l.enabled {
		c.Next()
		return
	}
	studentID, ok := AuthenticatedSubjectID(c)
	if !ok {
		abortAuthentication(c)
		return
	}
	if !l.Allow(studentID) {
		abortRateLimit(c)
		return
	}
	c.Next()
}

func abortRateLimit(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"code": http.StatusTooManyRequests,
		"info": "请求过于频繁，请稍后重试",
		"data": nil,
	})
}
