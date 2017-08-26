# rddlock
redis distribute lock

redis 分布式锁实现

## Usage

__Lock & UnLock__

```golang
lockkey := "lock-key"
timeout_ms := 3000

locked, ex := rddlock.Lock(rds, lockkey, timeout_ms)
defer reelock.UnLock(rds, lockkey, ex)
```

__LockRetry__

```golang
retry_times := 10
locked, ex := reelock.LockRetry(rds, lockkey, timeout_ms, retry_times) // get lock by retry
defer reelock.UnLock(rds, lockkey, ex)
```

__UnLockUnsafe__


```golang
locked, _ := rddlock.Lock(rds, lockkey, timeout_ms)
defer reelock.UnLockUnsafe(rds, lockkey)
```

__SyncDo__

```golang
err := SyncDo(rds, lockkey, timeout_ms, func(timeout chan bool) chan bool {
		ret := make(chan bool, 1)
		go func() {
			fmt.Println("doing...")
			// TODO SOMETHING
			select {
			case <-timeout:
				// do the rollback
				break
			case ret <- true:
				fmt.Println("success end.")
			}
		}()
		return ret
	})
```

## Example

```golang
package main

import (
	"fmt"

	"github.com/everfore/rddlock"
	"gopkg.in/ezbuy/redis-orm.v1/orm"
	redis "gopkg.in/redis.v5"
)

func main() {
	// rds, err := orm.NewRedisClient("localhost", 32768, "", 0)
	rds, err := orm.NewRedisClusterClient(&redis.ClusterOptions{
		Addrs: []string{"XXX", "XXX", "XXX"},
	})
	if err != nil {
		panic(err)
	}

	lock_key := "lock-key"

	locked, ex := rddlock.Lock(rds, lock_key, 5)
	if locked {
		fmt.Printf("LOCK %s: %+v\n", lock_key, locked)
		unlocked := rddlock.UnLock(rds, lock_key, ex)
		if unlocked {
			fmt.Printf("UNLOCK %s: %+v\n", lock_key, unlocked)
		} else {
			unlocked = rddlock.UnLockUnsafe(rds, lock_key)
			fmt.Printf("UNLOCK-UNSAFE %s: %+v\n", lock_key, unlocked)
		}
	}

	// retry lock

	// 1. lock the key first
	locked, _ = rddlock.Lock(rds, lock_key, 5)
	fmt.Printf("FIRST step, LOCK %s:%+v\n", lock_key, locked)
	// 2. retry to lock the locked key
	locked, _ = rddlock.LockRetry(rds, lock_key, 100, 100)
	fmt.Printf("SECOND step, LOCK-RETRY %s:%+v\n", lock_key, locked)
}

// Output
// LOCK lock-key: true
// UNLOCK lock-key: true
// FIRST step, LOCK lock-key:true
// SECOND step, LOCK-RETRY lock-key:true
```

## test

```
success:200, avg:1.1074123 ms
failed:0, avg:NaN ms
--- PASS: TestLockTime (10.59s)

#local-redis
=== RUN   TestLockRetryTime
success:200, avg:1.1741205 ms
failed:0, avg:NaN ms
--- PASS: TestLockRetryTime (10.58s)

#uat-redis
=== RUN   TestLockRetryTime
success:200, avg:12.572702 ms
failed:0, avg:NaN ms
--- PASS: TestLockRetryTime (10.59s)
```
