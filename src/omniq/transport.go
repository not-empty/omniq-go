package omniq

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisArg any

type RedisLike interface {
	EvalSha(sha string, numkeys int, args ...any) (any, error)
	Eval(script string, numkeys int, args ...any) (any, error)
	ScriptLoad(script string) (string, error)

	Exists(key string) (int64, error)
	HGet(key, field string) (*string, error)
	HGetAll(key string) (map[string]string, error)
	LLen(key string) (int64, error)
	ScanKeys(match string) ([]string, error)
	ZCard(key string) (int64, error)
	ZRange(key string, start, end int64) ([]string, error)
	ZRangeWithScores(key string, start, end int64) ([]MonitorZItem, error)
	ZRevRangeWithScores(key string, start, end int64) ([]MonitorZItem, error)
	Get(key string) (*string, error)
	HMGet(key string, fields ...string) ([]*string, error)
	ZScore(key, member string) (*float64, error)

	Ping() error
}

type RedisConnOpts struct {
	Host                 string
	Port                 int
	DB                   int
	Username             string
	Password             string
	SSL                  bool
	SocketTimeout        *time.Duration
	SocketConnectTimeout *time.Duration
}

func looksLikeClusterError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "cluster support disabled") ||
		strings.Contains(msg, "cluster mode is not enabled") ||
		(strings.Contains(msg, "unknown command") && strings.Contains(msg, "cluster")) ||
		strings.Contains(msg, "this instance has cluster support disabled") ||
		strings.Contains(msg, "err this instance has cluster support disabled") ||
		strings.Contains(msg, "only (p)subscribe / (p)unsubscribe / ping / quit allowed in this context") ||
		strings.Contains(msg, "moved") ||
		strings.Contains(msg, "ask")
}

type redisWrap struct {
	rdb     redis.Cmdable
	client  *redis.Client
	cluster *redis.ClusterClient
}

func (w *redisWrap) ctx() context.Context { return context.Background() }

func (w *redisWrap) Ping() error {
	return w.rdb.Ping(w.ctx()).Err()
}

func (w *redisWrap) EvalSha(sha string, numkeys int, args ...any) (any, error) {
	keys, argv := splitKeysArgs(numkeys, args)
	return w.rdb.EvalSha(w.ctx(), sha, keys, argv...).Result()
}

func (w *redisWrap) Eval(script string, numkeys int, args ...any) (any, error) {
	keys, argv := splitKeysArgs(numkeys, args)
	return w.rdb.Eval(w.ctx(), script, keys, argv...).Result()
}

func (w *redisWrap) ScriptLoad(script string) (string, error) {
	return w.rdb.ScriptLoad(w.ctx(), script).Result()
}

func (w *redisWrap) Exists(key string) (int64, error) {
	return w.rdb.Exists(w.ctx(), key).Result()
}

func (w *redisWrap) HGet(key, field string) (*string, error) {
	v, err := w.rdb.HGet(w.ctx(), key, field).Result()
	if err == nil {
		return &v, nil
	}
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return nil, err
}

func (w *redisWrap) HGetAll(key string) (map[string]string, error) {
	return w.rdb.HGetAll(w.ctx(), key).Result()
}

func (w *redisWrap) LLen(key string) (int64, error) {
	return w.rdb.LLen(w.ctx(), key).Result()
}

func (w *redisWrap) ScanKeys(match string) ([]string, error) {
	const scanCount int64 = 200

	if w.cluster != nil {
		seen := make(map[string]struct{})
		var mu sync.Mutex

		err := w.cluster.ForEachMaster(w.ctx(), func(ctx context.Context, c *redis.Client) error {
			var cursor uint64

			for {
				keys, next, err := c.Scan(ctx, cursor, match, scanCount).Result()
				if err != nil {
					return err
				}

				for _, key := range keys {
					if strings.TrimSpace(key) == "" {
						continue
					}
					mu.Lock()
					seen[key] = struct{}{}
					mu.Unlock()
				}

				if next == 0 {
					break
				}
				cursor = next
			}

			return nil
		})
		if err != nil {
			return nil, err
		}

		out := make([]string, 0, len(seen))
		for key := range seen {
			out = append(out, key)
		}
		sort.Strings(out)
		return out, nil
	}

	if w.client != nil {
		var cursor uint64
		out := make([]string, 0)

		for {
			keys, next, err := w.client.Scan(w.ctx(), cursor, match, scanCount).Result()
			if err != nil {
				return nil, err
			}

			for _, key := range keys {
				if strings.TrimSpace(key) == "" {
					continue
				}
				out = append(out, key)
			}

			if next == 0 {
				break
			}
			cursor = next
		}

		return out, nil
	}

	return []string{}, nil
}

func (w *redisWrap) ZCard(key string) (int64, error) {
	return w.rdb.ZCard(w.ctx(), key).Result()
}

func (w *redisWrap) ZRange(key string, start, end int64) ([]string, error) {
	return w.rdb.ZRange(w.ctx(), key, start, end).Result()
}

func (w *redisWrap) ZRangeWithScores(key string, start, end int64) ([]MonitorZItem, error) {
	rows, err := w.rdb.ZRangeWithScores(w.ctx(), key, start, end).Result()
	if err != nil {
		return nil, err
	}

	out := make([]MonitorZItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, MonitorZItem{
			Member: fmt.Sprint(row.Member),
			Score:  row.Score,
		})
	}
	return out, nil
}

func (w *redisWrap) ZRevRangeWithScores(key string, start, end int64) ([]MonitorZItem, error) {
	rows, err := w.rdb.ZRevRangeWithScores(w.ctx(), key, start, end).Result()
	if err != nil {
		return nil, err
	}

	out := make([]MonitorZItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, MonitorZItem{
			Member: fmt.Sprint(row.Member),
			Score:  row.Score,
		})
	}
	return out, nil
}

func (w *redisWrap) Get(key string) (*string, error) {
	v, err := w.rdb.Get(w.ctx(), key).Result()
	if err == nil {
		return &v, nil
	}
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return nil, err
}

func (w *redisWrap) HMGet(key string, fields ...string) ([]*string, error) {
	raw, err := w.rdb.HMGet(w.ctx(), key, fields...).Result()
	if err != nil {
		return nil, err
	}

	out := make([]*string, 0, len(raw))
	for _, it := range raw {
		if it == nil {
			out = append(out, nil)
			continue
		}
		s := fmt.Sprint(it)
		out = append(out, &s)
	}
	return out, nil
}

func (w *redisWrap) ZScore(key, member string) (*float64, error) {
	v, err := w.rdb.ZScore(w.ctx(), key, member).Result()
	if err == nil {
		return &v, nil
	}
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return nil, err
}

func redisArgsToAny(args []RedisArg) []any {
	out := make([]any, 0, len(args))
	for _, a := range args {
		out = append(out, any(a))
	}
	return out
}

func BuildRedisClient(opts RedisConnOpts) (RedisLike, error) {
	if opts.Host == "" {
		return nil, fmt.Errorf("RedisConnOpts requires host")
	}

	addr := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	var tlsCfg *tls.Config
	if opts.SSL {
		tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	{
		c := redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        []string{addr},
			Username:     opts.Username,
			Password:     opts.Password,
			TLSConfig:    tlsCfg,
			ReadTimeout:  durOrZero(opts.SocketTimeout),
			WriteTimeout: durOrZero(opts.SocketTimeout),
			DialTimeout:  durOrZero(opts.SocketConnectTimeout),
		})

		err := c.Ping(context.Background()).Err()
		if err == nil {
			return &redisWrap{rdb: c, cluster: c}, nil
		}
		_ = c.Close()

		if !looksLikeClusterError(err) {
			return nil, err
		}
	}

	c := redis.NewClient(&redis.Options{
		Addr:         addr,
		DB:           opts.DB,
		Username:     opts.Username,
		Password:     opts.Password,
		TLSConfig:    tlsCfg,
		ReadTimeout:  durOrZero(opts.SocketTimeout),
		WriteTimeout: durOrZero(opts.SocketTimeout),
		DialTimeout:  durOrZero(opts.SocketConnectTimeout),
	})

	if err := c.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &redisWrap{rdb: c, client: c}, nil
}

func durOrZero(d *time.Duration) time.Duration {
	if d == nil {
		return 0
	}
	return *d
}
