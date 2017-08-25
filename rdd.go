package rddlock

import (
	"log"
	"time"

	redis "gopkg.in/redis.v5"
)

// 直接删除key，可能会有问题：若删除之前，该key已经超时且被其他进程获得锁，将会删除其他进程的锁；删除之后，锁被释放，进而会有其他进程2获得锁。。。雪崩
func UnLock(rds redis.Cmdable, key string) bool {
	cmd := rds.Del(key)
	err := cmd.Err()
	if err != nil {
		log.Println(err)
		// 删除锁失败，会导致其他进程长时间等待锁
		return false
	}
	return true
}

func expiredTime(timeout_ms int) (int64, int64) {
	now := time.Now()
	return now.Unix(), now.Add(time.Duration(timeout_ms * 1000000)).Unix()
}

func Lock(rds redis.Cmdable, key string, timeout_ms int) bool {
	now, ex := expiredTime(timeout_ms)
	setnxCmd := rds.SetNX(key, ex, 0)
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
		prevTime, err := getCmd.Int64()
		if err != nil {
			log.Println("get key int64 err:", err)
			return false
		}
		// 已经过期，可以尝试获得锁了
		if now > prevTime {
			getSetCmd2 := rds.GetSet(key, ex)
			if getSetCmd2.Val() == getCmd.Val() {
				return true
			}
		} else {
			return false
		}
	}
	return false
}

// 重试 retry_times 次
func LockRetry(rds redis.Cmdable, key string, timeout_ms, retry_times int) bool {
	for i := 0; i < retry_times; i++ {
		now, ex := expiredTime(timeout_ms)
		setnxCmd := rds.SetNX(key, ex, 0)
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
			prevTime, err := getCmd.Int64()
			if err != nil {
				log.Println("get key int64 err:", err)
			} else {
				// 已经过期，可以尝试获得锁了
				if now > prevTime {
					getSetCmd2 := rds.GetSet(key, ex)
					if getSetCmd2.Val() == getCmd.Val() {
						return true
					}
				}
			}
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

func SyncDo(rds redis.Cmdable, key string, timeout_ms int, do func(rollback chan bool) chan bool) bool {
	timeout := time.NewTicker(time.Duration(timeout_ms * 1000000))
	Lock(rds, key, timeout_ms+1000)
	defer UnLock(rds, key)
	rollback := make(chan bool, 1)
	doret := do(rollback)
	select {
	case <-timeout.C:
		rollback <- true
		break
	case <-doret:
		return true
	}
	return false
}
