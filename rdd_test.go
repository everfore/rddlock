package rddlock

import (
	"gopkg.in/ezbuy/redis-orm.v1/orm"
	redis "gopkg.in/redis.v5"
	"testing"
)

var _redis_store *orm.RedisStore

func redisSetUp(host string, port int, password string, db int) {
	store, err := orm.NewRedisClient(host, port, password, db)
	if err != nil {
		panic(err)
	}
	_redis_store = store
}

func redisClusterSetUp(addrs []string) {
	store, err := orm.NewRedisClusterClient(&redis.ClusterOptions{
		Addrs: addrs,
	})
	if err != nil {
		panic(err)
	}
	_redis_store = store
}

func init() {
	redisSetUp(host, port, password, db)
}

func TestUnLock(t *testing.T) {
	UnLock(_redis_store, "locker")
}
