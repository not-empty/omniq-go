package omniq

import (
	"fmt"
	"strings"
)

type Exec struct {
	ops            *OmniqOps
	opts           *PublishOpts
	defaultChildID string
}

func newExec(ops *OmniqOps, opts *PublishOpts,  defaultChildID string) *Exec {
	return &Exec{ops: ops, opts: opts, defaultChildID: defaultChildID}
}

func (c *Exec) Publish(opts PublishOpts) (string, error) {
    if c != nil && c.opts != nil {
        if opts.MaxAttempts == 0 {
            opts.MaxAttempts = c.opts.MaxAttempts
        }
        if opts.Timeout == 0 {
            opts.Timeout = c.opts.Timeout
        }
        if opts.Backoff == 0 {
            opts.Backoff = c.opts.Backoff
        }
    }
    return c.ops.Publish(opts)
}

func (c *Exec) Pause(queue string) (string, error) {
	return c.ops.Pause(queue)
}

func (c *Exec) Resume(queue string) (int, error) {
	return c.ops.Resume(queue)
}

func (c *Exec) IsPaused(queue string) (bool, error) {
	return c.ops.IsPaused(queue)
}

func (c *Exec) ChildsInit(key string, expected int) error {
	if c == nil || c.ops == nil {
		return fmt.Errorf("Childs init not available")
	}
	return c.ops.ChildsInit(key, expected)
}

func (c *Exec) ChildAck(key string, childID ...string) (int, error) {
	if c == nil || c.ops == nil {
		return 0, fmt.Errorf("child ack not available")
	}

	cid := strings.TrimSpace(c.defaultChildID)
	if len(childID) > 0 && strings.TrimSpace(childID[0]) != "" {
		cid = strings.TrimSpace(childID[0])
	}
	if cid == "" {
		return 0, fmt.Errorf("childID is required")
	}

	return c.ops.ChildAck(key, cid)
}
