package cache

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/s2"
	"github.com/redis/go-redis/v9"
	"github.com/vmihailenco/msgpack/v5"
	"golang.org/x/sync/singleflight"
)

const (
	compressionThreshold = 64
	timeLen              = 4
)

const (
	noCompression = 0x0
	s2Compression = 0x1
)

var (
	ErrCacheMiss          = errors.New("cache: key is missing")
	errRedisLocalCacheNil = errors.New("cache: both Redis and LocalCache are nil")
)

type rediser interface {
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *redis.StatusCmd
	SetXX(ctx context.Context, key string, value interface{}, ttl time.Duration) *redis.BoolCmd
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) *redis.BoolCmd

	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd

	HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	HGet(ctx context.Context, key, field string) *redis.StringCmd
	Decr(ctx context.Context, key string) *redis.IntCmd
	Incr(ctx context.Context, key string) *redis.IntCmd
	RPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	BLPop(ctx context.Context, timeout time.Duration, keys ...string) *redis.StringSliceCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Pipeline() redis.Pipeliner
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
	Ping(ctx context.Context) *redis.StatusCmd
	Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub
	XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd
	XReadGroup(ctx context.Context, args *redis.XReadGroupArgs) *redis.XStreamSliceCmd
	XAutoClaim(ctx context.Context, args *redis.XAutoClaimArgs) *redis.XAutoClaimCmd
}

type Item struct {
	Ctx context.Context

	Key   string
	Value interface{}

	// TTL is the cache expiration time.
	// Default TTL is 1 hour.
	TTL time.Duration

	// Do returns value to be cached.
	Do func(*Item) (interface{}, error)

	// SetXX only sets the key if it already exists.
	SetXX bool

	// SetNX only sets the key if it does not already exist.
	SetNX bool

	// SkipLocalCache skips local cache as if it is not set.
	SkipLocalCache bool
}

func (item *Item) Context() context.Context {
	if item.Ctx == nil {
		return context.Background()
	}
	return item.Ctx
}

func (item *Item) value() (interface{}, error) {
	if item.Do != nil {
		return item.Do(item)
	}
	if item.Value != nil {
		return item.Value, nil
	}
	return nil, nil
}

func (item *Item) ttl() time.Duration {
	const defaultTTL = time.Hour

	if item.TTL < 0 {
		return 0
	}

	if item.TTL != 0 {
		if item.TTL < time.Second {
			log.Printf("too short TTL for key=%q: %s", item.Key, item.TTL)
			return defaultTTL
		}
		return item.TTL
	}

	return defaultTTL
}

// ------------------------------------------------------------------------------
type (
	MarshalFunc   func(interface{}) ([]byte, error)
	UnmarshalFunc func([]byte, interface{}) error
)

type Options struct {
	Redis        rediser
	LocalCache   LocalCache
	StatsEnabled bool
	Marshal      MarshalFunc
	Unmarshal    UnmarshalFunc
}

type Cache struct {
	opt *Options

	group singleflight.Group

	marshal   MarshalFunc
	unmarshal UnmarshalFunc

	hits   uint64
	misses uint64
}

// PubSubSubscription 表示一个已经由 Redis 确认的频道订阅。
type PubSubSubscription interface {
	Channel(opts ...redis.ChannelOption) <-chan *redis.Message
	Close() error
}

// StreamMessage 是从 Redis Stream 读取的一条原始消息。
type StreamMessage struct {
	ID     string
	Values map[string]interface{}
}

func New(opt *Options) *Cache {
	cacher := &Cache{
		opt: opt,
	}

	if opt.Marshal == nil {
		cacher.marshal = cacher._marshal
	} else {
		cacher.marshal = opt.Marshal
	}

	if opt.Unmarshal == nil {
		cacher.unmarshal = cacher._unmarshal
	} else {
		cacher.unmarshal = opt.Unmarshal
	}
	return cacher
}

// Set caches the item.
func (cd *Cache) Set(item *Item) error {
	_, _, err := cd.set(item)
	return err
}

func (cd *Cache) set(item *Item) ([]byte, bool, error) {
	value, err := item.value()
	if err != nil {
		return nil, false, err
	}

	b, err := cd.Marshal(value)
	if err != nil {
		return nil, false, err
	}

	if cd.opt.LocalCache != nil && !item.SkipLocalCache {
		cd.opt.LocalCache.Set(item.Key, b)
	}

	if cd.opt.Redis == nil {
		if cd.opt.LocalCache == nil {
			return b, true, errRedisLocalCacheNil
		}
		return b, true, nil
	}

	ttl := item.ttl()
	if ttl == 0 {
		return b, true, nil
	}

	if item.SetXX {
		return b, true, cd.opt.Redis.SetXX(item.Context(), item.Key, b, ttl).Err()
	}
	if item.SetNX {
		return b, true, cd.opt.Redis.SetNX(item.Context(), item.Key, b, ttl).Err()
	}
	return b, true, cd.opt.Redis.Set(item.Context(), item.Key, b, ttl).Err()
}

// Exists reports whether value for the given key exists.
func (cd *Cache) Exists(ctx context.Context, key string) bool {
	_, err := cd.getBytes(ctx, key, false)
	return err == nil
}

// Get gets the value for the given key.
func (cd *Cache) Get(ctx context.Context, key string, value interface{}) error {
	return cd.get(ctx, key, value, false)
}

// Get gets the value for the given key skipping local cache.
func (cd *Cache) GetSkippingLocalCache(
	ctx context.Context, key string, value interface{},
) error {
	return cd.get(ctx, key, value, true)
}

func (cd *Cache) get(
	ctx context.Context,
	key string,
	value interface{},
	skipLocalCache bool,
) error {
	b, err := cd.getBytes(ctx, key, skipLocalCache)
	if err != nil {
		return err
	}
	return cd.unmarshal(b, value)
}

func (cd *Cache) getBytes(ctx context.Context, key string, skipLocalCache bool) ([]byte, error) {
	if !skipLocalCache && cd.opt.LocalCache != nil {
		b, ok := cd.opt.LocalCache.Get(key)
		if ok {
			return b, nil
		}
	}

	if cd.opt.Redis == nil {
		if cd.opt.LocalCache == nil {
			return nil, errRedisLocalCacheNil
		}
		return nil, ErrCacheMiss
	}

	b, err := cd.opt.Redis.Get(ctx, key).Bytes()
	if err != nil {
		if cd.opt.StatsEnabled {
			atomic.AddUint64(&cd.misses, 1)
		}
		if err == redis.Nil {
			return nil, ErrCacheMiss
		}
		return nil, err
	}

	if cd.opt.StatsEnabled {
		atomic.AddUint64(&cd.hits, 1)
	}

	if !skipLocalCache && cd.opt.LocalCache != nil {
		cd.opt.LocalCache.Set(key, b)
	}
	return b, nil
}

// Once gets the item.Value for the given item.Key from the cache or
// executes, caches, and returns the results of the given item.Func,
// making sure that only one execution is in-flight for a given item.Key
// at a time. If a duplicate comes in, the duplicate caller waits for the
// original to complete and receives the same results.
func (cd *Cache) Once(item *Item) error {
	b, cached, err := cd.getSetItemBytesOnce(item)
	if err != nil {
		return err
	}

	if item.Value == nil || len(b) == 0 {
		return nil
	}

	if err := cd.unmarshal(b, item.Value); err != nil {
		if cached {
			_ = cd.Delete(item.Context(), item.Key)
			return cd.Once(item)
		}
		return err
	}

	return nil
}

func (cd *Cache) getSetItemBytesOnce(item *Item) (b []byte, cached bool, err error) {
	if cd.opt.LocalCache != nil {
		b, ok := cd.opt.LocalCache.Get(item.Key)
		if ok {
			return b, true, nil
		}
	}

	v, err, _ := cd.group.Do(item.Key, func() (interface{}, error) {
		b, err := cd.getBytes(item.Context(), item.Key, item.SkipLocalCache)
		if err == nil {
			cached = true
			return b, nil
		}

		b, ok, err := cd.set(item)
		if ok {
			return b, nil
		}
		return nil, err
	})
	if err != nil {
		return nil, false, err
	}
	return v.([]byte), cached, nil
}

func (cd *Cache) Delete(ctx context.Context, key string) error {
	if cd.opt.LocalCache != nil {
		cd.opt.LocalCache.Del(key)
	}

	if cd.opt.Redis == nil {
		if cd.opt.LocalCache == nil {
			return errRedisLocalCacheNil
		}
		return nil
	}

	_, err := cd.opt.Redis.Del(ctx, key).Result()
	return err
}

func (cd *Cache) Decr(ctx context.Context, key string) (int64, error) {
	if cd.opt.Redis == nil {
		return 0, errRedisLocalCacheNil
	}
	return cd.opt.Redis.Decr(ctx, key).Result()
}

func (cd *Cache) Incr(ctx context.Context, key string) (int64, error) {
	if cd.opt.Redis == nil {
		return 0, errRedisLocalCacheNil
	}
	return cd.opt.Redis.Incr(ctx, key).Result()
}

func (cd *Cache) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	if cd.opt.Redis == nil {
		return false, errRedisLocalCacheNil
	}

	// 如果是基本类型，直接传给 Redis 库（go-redis 内部会自动处理 string/int/[]byte）
	switch v := value.(type) {
	case string, []byte, int, int64, float64, bool:
		return cd.opt.Redis.SetNX(ctx, key, v, expiration).Result()
	default:
		// 只有复杂对象才序列化
		b, err := cd.Marshal(value)
		if err != nil {
			return false, err
		}
		return cd.opt.Redis.SetNX(ctx, key, b, expiration).Result()
	}
}

func (cd *Cache) RPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	if cd.opt.Redis == nil {
		return 0, errRedisLocalCacheNil
	}

	args := make([]interface{}, len(values))
	for i, v := range values {
		switch v.(type) {
		// 1. 基本类型：直接透传，利用 go-redis 内部的处理能力
		case string, []byte, int, int64, float64, bool:
			args[i] = v

		// 2. 复杂类型：执行自定义序列化 (如 JSON)
		default:
			b, err := cd.Marshal(v)
			if err != nil {
				return 0, fmt.Errorf("marshal value at index %d failed: %w", i, err)
			}
			args[i] = b
		}
	}

	return cd.opt.Redis.RPush(ctx, key, args...).Result()
}

func (cd *Cache) BLPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error) {
	if cd.opt.Redis == nil {
		return nil, errRedisLocalCacheNil
	}
	return cd.opt.Redis.BLPop(ctx, timeout, keys...).Result()
}

func (cd *Cache) DeleteFromLocalCache(key string) {
	if cd.opt.LocalCache != nil {
		cd.opt.LocalCache.Del(key)
	}
}

func (cd *Cache) Marshal(value interface{}) ([]byte, error) {
	return cd.marshal(value)
}

func (cd *Cache) _marshal(value interface{}) ([]byte, error) {
	switch value := value.(type) {
	case nil:
		return nil, nil
	case []byte:
		return value, nil
	case string:
		return []byte(value), nil
	}

	b, err := msgpack.Marshal(value)
	if err != nil {
		return nil, err
	}

	return compress(b), nil
}

func compress(data []byte) []byte {
	if len(data) < compressionThreshold {
		n := len(data) + 1
		b := make([]byte, n, n+timeLen)
		copy(b, data)
		b[len(b)-1] = noCompression
		return b
	}

	n := s2.MaxEncodedLen(len(data)) + 1
	b := make([]byte, n, n+timeLen)
	b = s2.Encode(b, data)
	b = append(b, s2Compression)
	return b
}

func (cd *Cache) Unmarshal(b []byte, value interface{}) error {
	return cd.unmarshal(b, value)
}

func (cd *Cache) _unmarshal(b []byte, value interface{}) error {
	if len(b) == 0 {
		return nil
	}

	switch value := value.(type) {
	case nil:
		return nil
	case *[]byte:
		clone := make([]byte, len(b))
		copy(clone, b)
		*value = clone
		return nil
	case *string:
		*value = string(b)
		return nil
	}

	switch c := b[len(b)-1]; c {
	case noCompression:
		b = b[:len(b)-1]
	case s2Compression:
		b = b[:len(b)-1]

		var err error
		b, err = s2.Decode(nil, b)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown compression method: %x", c)
	}

	return msgpack.Unmarshal(b, value)
}

func (cd *Cache) HSetWithTTL(ctx context.Context, key string, values interface{}, ttl time.Duration) error {
	if cd.opt.Redis == nil {
		return ErrCacheMiss
	}

	pipe := cd.opt.Redis.Pipeline()
	pipe.HSet(ctx, key, values)

	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (cd *Cache) HGet(ctx context.Context, key string, field string) (string, error) {
	if cd.opt.Redis == nil {
		return "", ErrCacheMiss
	}

	// 直接透传调用底层 HGet
	cmd := cd.opt.Redis.HGet(ctx, key, field)
	return cmd.Result()
}

//------------------------------------------------------------------------------

type Stats struct {
	Hits   uint64
	Misses uint64
}

// Eval executes a Lua script on Redis.
func (cd *Cache) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	if cd.opt.Redis == nil {
		return nil, errRedisLocalCacheNil
	}
	return cd.opt.Redis.Eval(ctx, script, keys, args...).Result()
}

// Ping verifies that the backing Redis instance is reachable.
func (cd *Cache) Ping(ctx context.Context) error {
	if cd.opt.Redis == nil {
		return errRedisLocalCacheNil
	}
	return cd.opt.Redis.Ping(ctx).Err()
}

// Publish 向 Redis Pub/Sub 频道发布二进制消息。
func (cd *Cache) Publish(ctx context.Context, channel string, message []byte) error {
	if cd == nil || cd.opt == nil || cd.opt.Redis == nil {
		return errRedisLocalCacheNil
	}
	return cd.opt.Redis.Publish(ctx, channel, message).Err()
}

// Subscribe 订阅 Redis Pub/Sub 频道，并等待 Redis 返回订阅确认。
func (cd *Cache) Subscribe(
	ctx context.Context,
	channels ...string,
) (PubSubSubscription, error) {
	if cd == nil || cd.opt == nil || cd.opt.Redis == nil {
		return nil, errRedisLocalCacheNil
	}
	subscription := cd.opt.Redis.Subscribe(ctx, channels...)
	if _, err := subscription.Receive(ctx); err != nil {
		_ = subscription.Close()
		return nil, err
	}
	return subscription, nil
}

// EnsureStreamConsumerGroup 创建 Consumer Group；组已存在时按成功处理。
// start 通常使用 "0"，确保部署切换前已经写入 Stream 的记录不会被跳过。
func (cd *Cache) EnsureStreamConsumerGroup(
	ctx context.Context,
	stream string,
	group string,
	start string,
) error {
	if cd == nil || cd.opt == nil || cd.opt.Redis == nil {
		return errRedisLocalCacheNil
	}
	err := cd.opt.Redis.XGroupCreateMkStream(ctx, stream, group, start).Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

// ReadStreamGroup 阻塞读取 Consumer Group 中尚未投递的新消息。
func (cd *Cache) ReadStreamGroup(
	ctx context.Context,
	stream string,
	group string,
	consumer string,
	count int64,
	block time.Duration,
) ([]StreamMessage, error) {
	if cd == nil || cd.opt == nil || cd.opt.Redis == nil {
		return nil, errRedisLocalCacheNil
	}
	streams, err := cd.opt.Redis.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    count,
		Block:    block,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return flattenStreamMessages(streams), nil
}

// ClaimStaleStreamMessages 领取超过 minIdle 仍未确认的消息，用于消费者崩溃恢复。
func (cd *Cache) ClaimStaleStreamMessages(
	ctx context.Context,
	stream string,
	group string,
	consumer string,
	minIdle time.Duration,
	start string,
	count int64,
) ([]StreamMessage, string, error) {
	if cd == nil || cd.opt == nil || cd.opt.Redis == nil {
		return nil, start, errRedisLocalCacheNil
	}
	messages, next, err := cd.opt.Redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumer,
		MinIdle:  minIdle,
		Start:    start,
		Count:    count,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, next, nil
	}
	if err != nil {
		return nil, start, err
	}
	entries := make([]StreamMessage, 0, len(messages))
	for _, message := range messages {
		entries = append(entries, StreamMessage{ID: message.ID, Values: message.Values})
	}
	return entries, next, nil
}

// AcknowledgeStreamMessages 在 MySQL 提交后原子确认并删除 Stream 消息。
func (cd *Cache) AcknowledgeStreamMessages(
	ctx context.Context,
	stream string,
	group string,
	ids ...string,
) error {
	if cd == nil || cd.opt == nil || cd.opt.Redis == nil {
		return errRedisLocalCacheNil
	}
	if len(ids) == 0 {
		return nil
	}
	args := make([]interface{}, 0, len(ids)+1)
	args = append(args, group)
	for _, id := range ids {
		args = append(args, id)
	}
	const acknowledgeScript = `
		local acknowledged = redis.call('XACK', KEYS[1], ARGV[1], unpack(ARGV, 2))
		local deleted = redis.call('XDEL', KEYS[1], unpack(ARGV, 2))
		return {acknowledged, deleted}
	`
	return cd.opt.Redis.Eval(ctx, acknowledgeScript, []string{stream}, args...).Err()
}

// DeadLetterStreamMessage 将无法处理的消息写入死信 Stream，再从原 Consumer Group
// 确认并删除；三个动作由同一个 Lua 脚本原子完成。
func (cd *Cache) DeadLetterStreamMessage(
	ctx context.Context,
	stream string,
	group string,
	deadLetterStream string,
	message StreamMessage,
	reason string,
) error {
	if cd == nil || cd.opt == nil || cd.opt.Redis == nil {
		return errRedisLocalCacheNil
	}
	payload := ""
	if value, exists := message.Values["event"]; exists {
		payload = fmt.Sprint(value)
	}
	const deadLetterScript = `
		redis.call(
			'XADD', KEYS[2], '*',
			'original_stream_id', ARGV[2],
			'event', ARGV[3],
			'error', ARGV[4]
		)
		redis.call('XACK', KEYS[1], ARGV[1], ARGV[2])
		redis.call('XDEL', KEYS[1], ARGV[2])
		return 1
	`
	return cd.opt.Redis.Eval(
		ctx,
		deadLetterScript,
		[]string{stream, deadLetterStream},
		group,
		message.ID,
		payload,
		reason,
	).Err()
}

func flattenStreamMessages(streams []redis.XStream) []StreamMessage {
	count := 0
	for _, stream := range streams {
		count += len(stream.Messages)
	}
	entries := make([]StreamMessage, 0, count)
	for _, stream := range streams {
		for _, message := range stream.Messages {
			entries = append(entries, StreamMessage{ID: message.ID, Values: message.Values})
		}
	}
	return entries
}

// Pipeline returns a Redis pipeline for batch operations.
func (cd *Cache) Pipeline() redis.Pipeliner {
	if cd.opt.Redis == nil {
		return nil
	}
	return cd.opt.Redis.Pipeline()
}

// Stats returns cache statistics.
func (cd *Cache) Stats() *Stats {
	if !cd.opt.StatsEnabled {
		return nil
	}
	return &Stats{
		Hits:   atomic.LoadUint64(&cd.hits),
		Misses: atomic.LoadUint64(&cd.misses),
	}
}
