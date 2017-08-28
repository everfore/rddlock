package rddlock

import (
	"fmt"
	"log"
	"strconv"
	"time"

	redis "gopkg.in/redis.v5"
)

func UnLock(rds redis.Cmdable, key string, ex interface{}) bool {
	if fmt.Sprintf("%d", ex) == "0" {
		// if ex <= 0 {
		// 如果没有获得锁，不能尝试解锁
		return false
	}
	script := `local key = redis.call('get',KEYS[1])
if (key == KEYS[2])
then
	return redis.call('del',KEYS[1])
end
return key`
	eval := rds.Eval(script, []string{key, fmt.Sprintf("%+v", ex)})
	if eval.Err() != nil {
		log.Printf("unlock failed, err:%+v", eval.Err())
		return false
	}
	if fmt.Sprintf("%+v", eval.Val()) == "1" {
		return true
	}
	// log.Printf("unlock failed, ex should be:%+v, got ex:%+v\n", eval.Val(), ex)
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

// 直接删除key，增加安全事件，若在安全时间内，key即将过期，就等待key过期（其他进程判断超时），以免造成雪崩
func UnLockSafe(rds redis.Cmdable, key string, safeDelTime_ms int64) bool {
	ex, err := rds.Get(key).Int64()
	if err != nil {
		log.Println(err)
		return false
	}
	now := time.Now().UnixNano()
	if now+safeDelTime*1000000 > ex {
		log.Println("the key is going to expire.")
		return false
	}
	err = rds.Del(key).Err()
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

func Lock(rds redis.Cmdable, key string, timeout_ms int) (bool, string) {
	now, ex := expiredTime(timeout_ms)
	setnxCmd := rds.SetNX(key, ex, 0)
	if setnxCmd.Val() {
		// log.Println("setnx got a lock, ex:", ex)
		return true, strconv.FormatInt(ex, 10)
	}
	getCmd := rds.Get(key)
	if getCmd.Val() == "" {
		getSetCmd := rds.GetSet(key, ex)
		if getSetCmd.Val() == "" {
			// log.Println("get nil, then getset got a lock, ex:", ex)
			return true, strconv.FormatInt(ex, 10)
		}
	} else {
		prevTime, err := getCmd.Int64()
		if err != nil {
			log.Println("get key int64 err:", err)
			return false, "0"
		}
		// 已经过期，可以尝试获得锁了
		if now > prevTime {
			getSetCmd2 := rds.GetSet(key, ex)
			if getSetCmd2.Val() == getCmd.Val() {
				// log.Println("timeout, then getset got a lock, ex:", ex)
				return true, strconv.FormatInt(ex, 10)
			}
		} else {
			return false, "0"
		}
	}
	return false, "0"
}

// 重试 retry_times 次
func LockRetry(rds redis.Cmdable, key string, timeout_ms, retry_times int) (bool, string) {
	for i := 0; i < retry_times; i++ {
		now, ex := expiredTime(timeout_ms)
		setnxCmd := rds.SetNX(key, ex, 0)
		if setnxCmd.Val() {
			return true, strconv.FormatInt(ex, 10)
		}
		getCmd := rds.Get(key)
		if getCmd.Val() == "" {
			getSetCmd := rds.GetSet(key, ex)
			if getSetCmd.Val() == "" {
				return true, strconv.FormatInt(ex, 10)
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
						return true, strconv.FormatInt(ex, 10)
					}
				}
			}
		}
		time.Sleep(time.Millisecond)
	}
	return false, "0"
}

// 执行异步任务
func SyncDo(rds redis.Cmdable, key string, timeout_ms int, do func(timeout chan bool) chan bool) error {
	ticker := time.NewTicker(time.Duration(timeout_ms * 1000000))
	locked, ex := LockRetry(rds, key, timeout_ms, 10)
	if !locked {
		return fmt.Errorf("lock the key(%s) failed!", key)
	}
	defer func() {
		unlocked := UnLock(rds, key, ex)
		if !unlocked {
			UnLockUnsafe(rds, key)
		}
	}()
	timeout := make(chan bool, 1)
	doRet := do(timeout)
	select {
	case <-ticker.C:
		timeout <- true
		return fmt.Errorf("timeout!")
	case <-doRet:
		return nil
	}
	return fmt.Errorf("unexcepted exit!")
}
