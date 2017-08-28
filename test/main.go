package main

import (
	"fmt"
	"net/http"

	"github.com/beego/mux"
	"github.com/everfore/rddlock"
	"gopkg.in/ezbuy/redis-orm.v1/orm"
	redis "gopkg.in/redis.v5"
)

type Msg struct {
	Message string
	Code    int
}

func newMsg(msg string, code int) *Msg {
	return &Msg{
		Message: msg,
		Code:    code,
	}
}

var (
	ticket = int32(10)
	given  = int32(0)
	key    = "ticked_key"
	rds    redis.Cmdable
)

func init() {
	var err error
	rds, err = orm.NewRedisClient("localhost", 32768, "", 0)
	// rds, err = orm.NewRedisClusterClient(&redis.ClusterOptions{
	// 	Addrs: []string{"192.168.199.237:7000", "192.168.199.235:7001", "192.168.199.235:7000"},
	// })

	if err != nil {
		panic(err)
	}
}

func main() {
	mx := mux.New()
	mx.Get("/buy", func(rw http.ResponseWriter, req *http.Request) {
		buy := false
		if ticket > 0 {
			buy = true
			ticket--
			given++
			fmt.Printf("buy:%+v, ticket:%d, given:%d\n", buy, ticket, given)
			fmt.Fprintf(rw, "%+v", newMsg("Success", int(given)))
		} else {
			fmt.Fprintf(rw, "%+v", newMsg("Failed", int(given)))
		}
	})
	mx.Get("/buy1", func(rw http.ResponseWriter, req *http.Request) {
		// locked, ex := rddlock.Lock(rds, key, 1000)
		locked, ex := rddlock.LockRetry(rds, key, 1000, 10)
		buy := false
		gotTicket := ticket
		if locked {
			fmt.Printf("ticket:[%d], lock ex:%+v, and the redis value is:%s\n", ticket, ex, rds.Get(key).Val())
			if ticket > 0 {
				buy = true
				ticket--
				given++
				fmt.Printf("buy:%+v, ticket:%d, given:%d\n", buy, ticket, given)
			}
			unlocked := rddlock.UnLock(rds, key, ex)
			if !unlocked {
				unlocked = rddlock.UnLockUnsafe(rds, key)
				// fmt.Printf("ticket:[%d], unlock use ex:%+v, but unlock failed, the redis ex is:%s\n", ticket, ex, rds.Get(key).Val())
				fmt.Println("unsafe unlock:", unlocked)
			}
		} else {
			fmt.Printf("ticket:[%d], lock failed ex:%+v, and the redis value is:%s\n", ticket, ex, rds.Get(key).Val())
		}
		if buy {
			fmt.Fprintf(rw, "%+v", newMsg(fmt.Sprintf("Success:%d", gotTicket), int(given)))
		} else {
			fmt.Fprintf(rw, "%+v", newMsg(fmt.Sprintf("Failed:%d", gotTicket), int(given)))
		}
	})
	mx.Get("/reset", func(rw http.ResponseWriter, req *http.Request) {
		locked, _ := rddlock.LockRetry(rds, key, 1000, 10)
		if locked {
			given = 0
			ticket = 15
			defer rddlock.UnLockUnsafe(rds, key)
			fmt.Fprintf(rw, "%+v", newMsg("ok", 200))
		} else {
			fmt.Fprintf(rw, "%+v", newMsg("lock failed", 301))
		}
		fmt.Println("\n*************************")
	})
	mx.Get("/status", func(rw http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(rw, "%+v", newMsg(fmt.Sprintf("ticket:%d, given:%d", ticket, given), 0))
	})

	http.ListenAndServe(":8080", mx)
}
