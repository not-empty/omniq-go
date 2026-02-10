package omniq

import "fmt"

type Client struct {
	ops OmniqOps
}

type ClientOpts struct {
	Redis      RedisLike
	RedisURL   string
	Host       string
	Port       int
	DB         int
	Username   string
	Password   string
	SSL        bool
	ScriptsDir string
}

func NewClient(opts ClientOpts) (*Client, error) {
	var r RedisLike
	if opts.Redis != nil {
		r = opts.Redis
	} else {
		port := opts.Port
		if port == 0 {
			port = 6379
		}

		conn := RedisConnOpts{
			RedisURL: opts.RedisURL,
			Host:     opts.Host,
			Port:     port,
			DB:       opts.DB,
			Username: opts.Username,
			Password: opts.Password,
			SSL:      opts.SSL,
		}

		var err error
		r, err = BuildRedisClient(conn)
		if err != nil {
			return nil, err
		}
	}

	sdir := opts.ScriptsDir
	if sdir == "" {
		sdir = DefaultScriptsDir()
	}

	scripts, err := LoadScripts(r, sdir)
	if err != nil {
		return nil, err
	}

	return &Client{
		ops: OmniqOps{R: r, Scripts: scripts},
	}, nil
}

func QueueBaseName(queueName string) string {
	return QueueBase(queueName)
}

// Publish matches the Consume() style: a single options object.
// This avoids positional-arg mistakes and mirrors Python's named args.
func (c *Client) Publish(opts PublishOpts) (string, error) {
	return c.ops.Publish(opts)
}

func (c *Client) Reserve(queue string, nowMsOverride int64) (ReserveResult, error) {
	return c.ops.Reserve(queue, nowMsOverride)
}

func (c *Client) Heartbeat(queue, jobID, leaseToken string, nowMsOverride int64) (int64, error) {
	return c.ops.Heartbeat(queue, jobID, leaseToken, nowMsOverride)
}

func (c *Client) AckSuccess(queue, jobID, leaseToken string, nowMsOverride int64) error {
	return c.ops.AckSuccess(queue, jobID, leaseToken, nowMsOverride)
}

func (c *Client) AckFail(queue, jobID, leaseToken string, nowMsOverride int64) (AckFailResult, error) {
	return c.ops.AckFail(queue, jobID, leaseToken, nil, nowMsOverride)
}

func (c *Client) PromoteDelayed(queue string, maxPromote int, nowMsOverride int64) (int, error) {
	return c.ops.PromoteDelayed(queue, maxPromote, nowMsOverride)
}

func (c *Client) ReapExpired(queue string, maxReap int, nowMsOverride int64) (int, error) {
	return c.ops.ReapExpired(queue, maxReap, nowMsOverride)
}

func (c *Client) Pause(queue string) (string, error) {
	return c.ops.Pause(queue)
}

func (c *Client) Resume(queue string) (int, error) {
	return c.ops.Resume(queue)
}

func (c *Client) IsPaused(queue string) (bool, error) {
	return c.ops.IsPaused(queue)
}

type ConsumeLogger func(msg string)

func DefaultConsumeLogger(msg string) { fmt.Println(msg) }

type ConsumeHandler func(ctx JobCtx) error

type ConsumeOpts struct {
	Queue string

	Handler ConsumeHandler

	PollIntervalS      float64
	PromoteIntervalS   float64
	PromoteBatch       int
	ReapIntervalS      float64
	ReapBatch          int
	HeartbeatIntervalS *float64

	Verbose bool
	Logger  ConsumeLogger
	Drain   bool
}

func (c *Client) Consume(opts ConsumeOpts) error {
	return consumeLoop(&c.ops, opts)
}

func (c *Client) Ops() *OmniqOps {
	return &c.ops
}
