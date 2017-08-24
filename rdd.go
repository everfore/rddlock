package rddlock

import (
	redis "gopkg.in/redis.v5"
)

func UnLock(rds redis.Cmdable, key string) bool {
	cmd := rds.Del(key)
	err := cmd.Err()
	if err != nil {
		return false
	}
	return true
}
