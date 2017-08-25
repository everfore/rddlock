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
}

func TestLock(t *testing.T) {
	start := time.Now()
	locked := Lock(rds, "locker", 1000, 5)
	t.Log("locked:", locked)

	unlocked := UnLock(rds, "locker")
	t.Log("unlocked:", unlocked)
	t.Logf("cost:%+v", time.Now().Sub(start))

	locked = Lock(rds, "locker", 1000, 5)
	t.Log("locked:", locked)

	locked = Lock(rds, "locker", 1000, 5)
	t.Log("locked2:", locked)

	unlocked = UnLock(rds, "locker")
	t.Log("unlocked:", unlocked)
}

func TestSync(t *testing.T) {
	fmt.Println("TestSync")
	all := int32(100)
	counter := int32(0)
	for i := 0; i < 200; i++ {
		go func() {
			locked := Lock(rds, "locker-for-all", 5, 1)
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
		time.Sleep(5e5)
	}
	if counter+all < 100 {
		fmt.Println("ERROR")
	}
	fmt.Printf("counter:%d, all:%d\n", counter, all)
}

func BenchmarkLock(b *testing.B) {
	var lockCount int32
	for i := 0; i < b.N; i++ {
		go UnLock(rds, "key")
		ok := Lock(rds, "key", 1, 1)
		if ok {
			b.Log("locked")
			atomic.AddInt32(&lockCount, 1)
		}
	}
	time.Sleep(1e9)
	ok := Lock(rds, "key", 1, 1)
	fmt.Println("locked:", ok)
	if !ok {
		UnLock(rds, "key")
	}
	ok = Lock(rds, "key", 1, 1)
	fmt.Println("again locked:", ok)
	fmt.Printf("b.N:%d, locked:%d\n", b.N, lockCount)
}
