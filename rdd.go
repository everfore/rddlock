package rddlock

import (
	"fmt"
	"log"
	"strconv"
	"time"

	redis "gopkg.in/redis.v5"
)

func UnLock(rds redis.Cmdable, key string, ex int64) bool {
	if ex <= 0 {
		// 如果没有获得锁，不能尝试解锁
		return false
	}
	script := `local key = redis.call('get',KEYS[1])
if (key == KEYS[2])
then
	return redis.call('del',KEYS[1])
end
return 0`
	eval := rds.Eval(script, []string{key, strconv.FormatInt(ex, 10)})
	if eval.Err() != nil {
		log.Printf("unlock failed, err:%+v", eval.Err())
		return false
	}
	if fmt.Sprintf("%+v", eval.Val()) == "1" {
		return true
	}
	return false
}

// 直接删除key，可能会有问题：若删除之前，该key已经超时且被其他进程获得锁，将会删除其他进程的锁；删除之后，锁被释放，进而会有其他进程2获得锁。。。雪崩
func UnLockUnsafe(rds redis.Cmdable, key string) bool {
	err := rds.Del(key).Err()
	if err != nil {
		log.Println(err)
		// 删除锁失败，会导致其他进程长时间等待锁
		return false
	}
	return true
}

func expiredTime(timeout_ms int) (int64, int64) {
	now := time.Now()
	return now.UnixNano(), now.Add(time.Duration(timeout_ms * 1000000)).UnixNano()
}

func Lock(rds redis.Cmdable, key string, timeout_ms int) (bool, int64) {
	now, ex := expiredTime(timeout_ms)
	setnxCmd := rds.SetNX(key, ex, 0)
	if setnxCmd.Val() {
		return true, ex
	}
	getCmd := rds.Get(key)
	if getCmd.Val() == "" {
		getSetCmd := rds.GetSet(key, ex)
		if getSetCmd.Val() == "" {
			return true, ex
		}
	} else {
		prevTime, err := getCmd.Int64()
		if err != nil {
			log.Println("get key int64 err:", err)
			return false, 0
		}
		// 已经过期，可以尝试获得锁了
		if now > prevTime {
			getSetCmd2 := rds.GetSet(key, ex)
			if getSetCmd2.Val() == getCmd.Val() {
				return true, ex
			}
		} else {
			return false, 0
		}
	}
	return false, 0
}

// 重试 retry_times 次
func LockRetry(rds redis.Cmdable, key string, timeout_ms, retry_times int) (bool, int64) {
	for i := 0; i < retry_times; i++ {
		now, ex := expiredTime(timeout_ms)
		setnxCmd := rds.SetNX(key, ex, 0)
		if setnxCmd.Val() {
			return true, ex
		}
		getCmd := rds.Get(key)
		if getCmd.Val() == "" {
			getSetCmd := rds.GetSet(key, ex)
			if getSetCmd.Val() == "" {
				return true, ex
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
						return true, ex
					}
				}
			}
		}
		time.Sleep(time.Millisecond)
	}
	return false, 0
}

// 执行异步任务
func SyncDo(rds redis.Cmdable, key string, timeout_ms int, do func(timeout chan bool) chan bool) bool {
	ticker := time.NewTicker(time.Duration(timeout_ms * 1000000))
	_, ex := Lock(rds, key, timeout_ms)
	defer UnLock(rds, key, ex)
	timeout := make(chan bool, 1)
	doRet := do(timeout)
	select {
	case <-ticker.C:
		timeout <- true
		log.Println("timeout, rollback!")
		return false
	case <-doRet:
		return true
	}
	return false
}
