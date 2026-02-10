package omniq

import "time"

func NowMs() int64 {
	return time.Now().UnixMilli()
}
