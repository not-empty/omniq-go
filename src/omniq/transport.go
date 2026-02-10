package omniq

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
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
	LLen(key string) (int64, error)
	ZCard(key string) (int64, error)
	ZRange(key string, start, end int64) ([]string, error)
	Get(key string) (*string, error)
	HMGet(key string, fields ...string) ([]*string, error)
	ZScore(key, member string) (*float64, error)

	Ping() error
}

type RedisConnOpts struct {
	RedisURL            string
	Host                string
	Port                int
	DB                  int
	Username            string
	Password            string
	SSL                 bool
	SocketTimeout       *time.Duration
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
	rdb redis.Cmdable
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

func (w *redisWrap) LLen(key string) (int64, error) {
	return w.rdb.LLen(w.ctx(), key).Result()
}

func (w *redisWrap) ZCard(key string) (int64, error) {
	return w.rdb.ZCard(w.ctx(), key).Result()
}

func (w *redisWrap) ZRange(key string, start, end int64) ([]string, error) {
	return w.rdb.ZRange(w.ctx(), key, start, end).Result()
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
	if opts.RedisURL != "" {
		ropts, err := redis.ParseURL(opts.RedisURL)
		if err != nil {
			return nil, fmt.Errorf("parse redis_url: %w", err)
		}
		if opts.SSL && ropts.TLSConfig == nil {
			ropts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		if opts.SocketTimeout != nil {
			ropts.ReadTimeout = *opts.SocketTimeout
			ropts.WriteTimeout = *opts.SocketTimeout
		}
		if opts.SocketConnectTimeout != nil {
			ropts.DialTimeout = *opts.SocketConnectTimeout
		}

		c := redis.NewClient(ropts)
		if err := c.Ping(context.Background()).Err(); err != nil {
			return nil, err
		}
		return &redisWrap{rdb: c}, nil
	}

	if opts.Host == "" {
		return nil, fmt.Errorf("RedisConnOpts requires host (or redis_url)")
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
			return &redisWrap{rdb: c}, nil
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
	return &redisWrap{rdb: c}, nil
}

func durOrZero(d *time.Duration) time.Duration {
	if d == nil {
		return 0
	}
	return *d
}
