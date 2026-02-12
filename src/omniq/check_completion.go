package omniq

import (
	"fmt"
	"strings"
)

type CheckCompletion struct {
	ops            *OmniqOps
	defaultChildID string
}

func newCheckCompletion(ops *OmniqOps, defaultChildID string) *CheckCompletion {
	return &CheckCompletion{ops: ops, defaultChildID: defaultChildID}
}

func (c *CheckCompletion) InitJobCounter(key string, expected int) error {
	if c == nil || c.ops == nil {
		return fmt.Errorf("check_completion not available")
	}
	return c.ops.CheckCompletionInitJobCounter(key, expected)
}

func (c *CheckCompletion) JobDecrement(key string, childID ...string) (int, error) {
	if c == nil || c.ops == nil {
		return 0, fmt.Errorf("check_completion not available")
	}

	cid := strings.TrimSpace(c.defaultChildID)
	if len(childID) > 0 && strings.TrimSpace(childID[0]) != "" {
		cid = strings.TrimSpace(childID[0])
	}
	if cid == "" {
		return 0, fmt.Errorf("childID is required")
	}

	return c.ops.CheckCompletionJobDecrement(key, cid)
}
