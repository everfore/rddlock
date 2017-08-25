package rddlock

import (
	"log"
	"time"

	redis "gopkg.in/redis.v5"
)

func UnLock(rds redis.Cmdable, key string) bool {
	cmd := rds.Del(key)
	err := cmd.Err()
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}

func Lock(rds redis.Cmdable, key string, timeout_ms, retry int) bool {
	now, ex := expiredTime(timeout_ms)
	setnxCmd := rds.SetNX(key, ex, 0)
	// log.Printf("SetNX:%+v", setnxCmd.Val())
	if setnxCmd.Val() {
		return true
	}
	getCmd := rds.Get(key)
	if getCmd.Val() == "" {
		getSetCmd := rds.GetSet(key, ex)
		if getSetCmd.Val() == "" {
			return true
		}
	} else {
		// log.Println("get is not nil, peek timeout")
		prevTime, err := getCmd.Int64()
		if err != nil {
			log.Println("get key int64 err:", err)
			return false
		}
		// 已经过期，可以尝试获得锁了
		if now > prevTime {
			getSetCmd2 := rds.GetSet(key, ex)
			if getSetCmd2.Val() == getCmd.Val() {
				// log.Println("getset then got a locker.", getSetCmd2.Val())
				return true
			}
		} else {
			return false
		}
	}
	// log.Println("now:", now)
	return false
}

func expiredTime(timeout_ms int) (int64, int64) {
	now := time.Now()
	return now.Unix(), now.Add(time.Millisecond).Unix()
}
