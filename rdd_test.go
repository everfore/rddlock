package rddlock

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"gopkg.in/ezbuy/redis-orm.v1/orm"
	redis "gopkg.in/redis.v5"
)

var rds *orm.RedisStore

func redisSetUp(host string, port int, password string, db int) {
	store, err := orm.NewRedisClient(host, port, password, db)
	if err != nil {
		panic(err)
	}
	rds = store
}

func redisClusterSetUp(addrs []string) {
	store, err := orm.NewRedisClusterClient(&redis.ClusterOptions{
		Addrs: addrs,
	})
	if err != nil {
		panic(err)
	}
	rds = store
}

func init() {
	redisSetUp("localhost", 32768, "", 0)
	// redisClusterSetUp([]string{"192.168.199.237:7000", "192.168.199.235:7001", "192.168.199.235:7000"})
	// redisClusterSetUp([]string{"192.168.199.1:11110", "192.168.199.1:11111", "192.168.199.1:11112"})
}

func TestLockAndUnLock(t *testing.T) {
	key := "LOCK-AND-UNLOCK"
	timeout_ms := 10000 // 10s
	locked, ex := Lock(rds, key, timeout_ms)
	if !locked {
		panic("lock failed!")
	}
	unlocked := UnLock(rds, key, ex)
	fmt.Printf("1.normal-lock locked %s:%t, unlock:%t\n", key, locked, unlocked)

	locked, ex1 := Lock(rds, key, timeout_ms)
	if !locked {
		panic("lock failed!")
	}

	locked, ex2 := Lock(rds, key, timeout_ms)
	if locked {
		panic("lock success!")
	}

	unlocked = UnLock(rds, key, ex)
	if unlocked {
		fmt.Printf("ex:%d, ex1:%d, ex2:%d\n", ex, ex1, ex2)
		panic("ex unlock success")
	}

	unlocked = UnLock(rds, key, ex2)
	if unlocked {
		panic("ex2 unlock success")
	}

	unlocked = UnLock(rds, key, ex1)
	if !unlocked {
		panic("ex1 unlock failed")
	}

	fmt.Println("2.innormal-lock lock & unlock test pass.")
}

func TestLockRetryAndUnLock(t *testing.T) {
	key := "LOCK-RETRY-AND-UNLOCK"
	timeout_ms := 10000 // 10s
	locked, ex := LockRetry(rds, key, 10, 10)
	if !locked {
		panic("lock failed!")
	}

	locked, ex01 := LockRetry(rds, key, timeout_ms, 100) // 前一个锁超时，可获得锁
	if !locked {
		panic("lock failed!")
	}

	unlocked := UnLock(rds, key, ex01)
	if !unlocked {
		panic("unlock failed!")
	}

	fmt.Printf("1.normal-lock-retry locked %s:%t, unlock:%t\n", key, locked, unlocked)

	locked, ex1 := LockRetry(rds, key, timeout_ms, 10)
	if !locked {
		panic("lock failed!")
	}

	locked, ex2 := LockRetry(rds, key, timeout_ms, 10)
	if locked {
		panic("lock success!")
	}

	unlocked = UnLock(rds, key, ex)
	if unlocked {
		fmt.Printf("ex:%d, ex1:%d, ex2:%d\n", ex, ex1, ex2)
		panic("ex unlock success")
	}

	unlocked = UnLock(rds, key, ex01)
	if unlocked {
		fmt.Printf("ex:%d, ex1:%d, ex2:%d\n", ex, ex1, ex2)
		panic("ex01 unlock success")
	}

	unlocked = UnLock(rds, key, ex2)
	if unlocked {
		panic("ex2 unlock success")
	}

	unlocked = UnLock(rds, key, ex1)
	if !unlocked {
		panic("ex1 unlock failed")
	}

	fmt.Println("2.innormal-lock-retry lock & unlock test pass.")
}

func TestLockCurrency(t *testing.T) {
	key := "LOCK-CURRENCY"
	all := int32(100)
	counter := int32(0)
	for i := 0; i < 200; i++ {
		go func() {
			locked, ex := Lock(rds, key, 5)
			if locked {
				counter++
				if all > 0 {
					all--
					if counter+all != 100 {
						fmt.Println("ERR")
					}
				}
				unlocked := UnLock(rds, key, ex)
				if !unlocked {
					fmt.Println("Unlocked.")
				}
			}
			fmt.Printf("counter:%d, all:%d\n", counter, all)
		}()
		time.Sleep(5e7)
	}
	if counter+all < 100 {
		fmt.Println("ERROR")
	}
}

func TestLockRetryCurrency(t *testing.T) {
	key := "LOCK-RETRY-CURRENCY"
	all := int32(100)
	counter := int32(0)
	for i := 0; i < 200; i++ {
		go func() {
			locked, ex := Lock(rds, key, 5)
			if locked {
				counter++
				if all > 0 {
					all--
					if counter+all != 100 {
						fmt.Println("ERR")
					}
				}
				unlocked := UnLock(rds, key, ex)
				if !unlocked {
					fmt.Println("err: Unlocked.")
				}
			}
			fmt.Printf("counter:%d, all:%d\n", counter, all)
		}()
		time.Sleep(5e7)
	}
	if counter+all < 100 {
		fmt.Println("ERROR")
	}
}

func TestLockTime(t *testing.T) {
	var succTime, failedTime float32
	var succCount, failedCount int
	key := "LOCK-TIME"
	for i := 0; i < 200; i++ {
		go func() {
			start := time.Now()
			locked, ex := Lock(rds, key, 5)
			cost := time.Now().Sub(start)
			if locked {
				succTime += float32(cost.Nanoseconds())
				succCount++
				unlocked := UnLock(rds, key, ex)
				if !unlocked {
					fmt.Println("err: Unlocked.")
				}
			} else {
				failedTime += float32(cost.Nanoseconds())
				failedCount++
			}
		}()
		time.Sleep(5e7)
	}
	fmt.Printf("\n\nsuccess:%d, avg:%+v ms\n", succCount, succTime/(float32(succCount)*float32(1000000.0)))
	fmt.Printf("failed:%d, avg:%+v ms\n", failedCount, failedTime/(float32(failedCount)*float32(1000000.0)))
}

func TestLockRetryTime(t *testing.T) {
	var succTime, failedTime float32
	var succCount, failedCount int
	key := "LOCK-RETRY-TIME"
	for i := 0; i < 200; i++ {
		go func() {
			start := time.Now()
			locked, ex := LockRetry(rds, key, 10, 5)
			cost := time.Now().Sub(start)
			if locked {
				succTime += float32(cost.Nanoseconds())
				succCount++
				unlocked := UnLock(rds, key, ex)
				if !unlocked {
					fmt.Println("err: Unlocked.")
				}
			} else {
				failedTime += float32(cost.Nanoseconds())
				failedCount++
			}
		}()
		time.Sleep(5e7)
	}
	fmt.Printf("\n\nsuccess:%d, avg:%+v ms\n", succCount, succTime/(float32(succCount)*float32(1000000.0)))
	fmt.Printf("failed:%d, avg:%+v ms\n", failedCount, failedTime/(float32(failedCount)*float32(1000000.0)))
}

func BenchmarkLock(b *testing.B) {
	var lockCount int32
	for i := 0; i < b.N; i++ {
		ok, ex := Lock(rds, "key", 1)
		go UnLock(rds, "key", ex)
		if ok {
			b.Log("locked")
			atomic.AddInt32(&lockCount, 1)
		}
	}
	fmt.Printf("b.N:%d, locked:%d\n", b.N, lockCount)
}

func TestSyncDo(t *testing.T) {
	start := time.Now()
	success := SyncDo(rds, "sync-do-locker", 1000, func(timeout chan bool) chan bool {
		ret := make(chan bool, 1)
		go func() {
			fmt.Println("doing...")
			time.Sleep(5e9)
			select {
			case <-timeout:
				break
			case ret <- true:
				fmt.Println("success end.")
			}
		}()
		return ret
	})
	fmt.Printf("successed:%+v, cost:%+v\n", success, time.Now().Sub(start))
}

func TestUnLockUnSafe(t *testing.T) {
	locked, _ := Lock(rds, "TestSafeUnLock", 1000)
	fmt.Printf("locked:%+v\n", locked)
	unlocked := UnLockUnsafe(rds, "TestSafeUnLock")
	fmt.Printf("unlocked:%+v\n", unlocked)
}
