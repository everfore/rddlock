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
	// redisClusterSetUp([]string{"192.168.199.1:11110", "192.168.199.1:11111", "192.168.199.1:11112"})
}

func TestLock(t *testing.T) {
	start := time.Now()
	locked := Lock(rds, "locker", 1000)
	t.Log("locked:", locked)

	unlocked := UnLock(rds, "locker")
	t.Log("unlocked:", unlocked)
	t.Logf("cost:%+v", time.Now().Sub(start))

	locked = Lock(rds, "locker", 1000)
	t.Log("locked:", locked)

	locked = Lock(rds, "locker", 1000)
	t.Log("locked2:", locked)

	unlocked = UnLock(rds, "locker")
	t.Log("unlocked:", unlocked)
}

func TestSync(t *testing.T) {
	all := int32(100)
	counter := int32(0)
	for i := 0; i < 200; i++ {
		go func() {
			start := time.Now()
			locked := Lock(rds, "locker-for-all", 5)
			fmt.Printf("locked:%+v, cost %+v\n", locked, time.Now().Sub(start))
			if locked {
				// atomic.AddInt32(&counter, 1)
				counter++
				if all > 0 {
					all--
					// atomic.AddInt32(&all, -1)
					if counter+all != 100 {
						fmt.Println("ERR")
					}
				}
				defer UnLock(rds, "locker-for-all")
				fmt.Println("locked", counter, "-", all)
			}
		}()
		time.Sleep(5e7)
	}
	if counter+all < 100 {
		fmt.Println("ERROR")
	}
	fmt.Printf("counter:%d, all:%d\n", counter, all)
}

/**
[cmd] go test -v -test.run Time

local-redis
=== RUN   TestTime
success:200, avg:1.1732105 ms
failed:0, avg:NaN ms
--- PASS: TestTime (10.53s)
PASS
ok  	github.com/everfore/rddlock	10.546s

uat-redis
=== RUN   TestTime
success:200, avg:5.2903905 ms
failed:0, avg:NaN ms
--- PASS: TestTime (10.56s)
PASS
ok  	github.com/everfore/rddlock	10.839s
*/
// go test -v -test.run Time
func TestTime(t *testing.T) {
	var succTime, failedTime float32
	var succCount, failedCount int
	for i := 0; i < 200; i++ {
		go func() {
			start := time.Now()
			locked := Lock(rds, "locker-for-all", 5)
			cost := time.Now().Sub(start)
			if locked {
				succTime += float32(cost.Nanoseconds())
				succCount++
				defer UnLock(rds, "locker-for-all")
			} else {
				failedTime += float32(cost.Nanoseconds())
				failedCount++
			}
		}()
		time.Sleep(5e7)
	}
	fmt.Printf("success:%d, avg:%+v ms\n", succCount, succTime/(float32(succCount)*float32(1000000.0)))
	fmt.Printf("failed:%d, avg:%+v ms\n", failedCount, failedTime/(float32(failedCount)*float32(1000000.0)))
}

func BenchmarkLock(b *testing.B) {
	var lockCount int32
	for i := 0; i < b.N; i++ {
		go UnLock(rds, "key")
		ok := Lock(rds, "key", 1)
		if ok {
			b.Log("locked")
			atomic.AddInt32(&lockCount, 1)
		}
	}
	time.Sleep(1e9)
	ok := Lock(rds, "key", 1)
	fmt.Println("locked:", ok)
	if !ok {
		UnLock(rds, "key")
	}
	ok = Lock(rds, "key", 1)
	fmt.Println("again locked:", ok)
	fmt.Printf("b.N:%d, locked:%d\n", b.N, lockCount)
}

func TestLockRetry(t *testing.T) {
	lockKey := "lockRetry-key"
	all := int32(100)
	counter := int32(0)
	for i := 0; i < 200; i++ {
		go func() {
			locked := LockRetry(rds, lockKey, 10, 10)
			if locked {
				counter++
				if all > 0 {
					all--
					if counter+all != 100 {
						fmt.Println("ERR")
					}
				}
			}
			fmt.Printf("locked:%+v, %d - %d\n", locked, counter, all)
			defer UnLock(rds, lockKey)
		}()
		time.Sleep(time.Millisecond)
	}
	if counter+all < 100 {
		fmt.Println("ERROR")
	}
}

/**
[cmd] go test -v -test.run RetryTime

local-redis
=== RUN   TestLockRetryTime
success:200, avg:1.1741205 ms
failed:0, avg:NaN ms
--- PASS: TestLockRetryTime (10.58s)
PASS
ok  	github.com/everfore/rddlock	10.597s

uat-redis
=== RUN   TestLockRetryTime
success:200, avg:12.572702 ms
failed:0, avg:NaN ms
--- PASS: TestLockRetryTime (10.59s)
PASS
ok  	github.com/everfore/rddlock	10.634s
*/
func TestLockRetryTime(t *testing.T) {
	var succTime, failedTime float32
	var succCount, failedCount int
	for i := 0; i < 200; i++ {
		go func() {
			start := time.Now()
			locked := LockRetry(rds, "locker-for-all", 10, 5)
			cost := time.Now().Sub(start)
			if locked {
				succTime += float32(cost.Nanoseconds())
				succCount++
				defer UnLock(rds, "locker-for-all")
			} else {
				failedTime += float32(cost.Nanoseconds())
				failedCount++
			}
		}()
		time.Sleep(5e7)
	}
	fmt.Printf("success:%d, avg:%+v ms\n", succCount, succTime/(float32(succCount)*float32(1000000.0)))
	fmt.Printf("failed:%d, avg:%+v ms\n", failedCount, failedTime/(float32(failedCount)*float32(1000000.0)))
}
