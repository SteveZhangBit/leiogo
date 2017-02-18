package redis

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/SteveZhangBit/leiogo/middleware"

	"github.com/SteveZhangBit/leiogo"
	"github.com/garyburd/redigo/redis"
)

type RedisWriter struct {
	Addr     string
	PoolSize int
	connPool chan redis.Conn
}

func (r *RedisWriter) Open(spider *leiogo.Spider) error {
	// add connections to the pool
	for i := 0; i < r.PoolSize; i++ {
		if conn, err := redis.Dial("tcp", r.Addr); err != nil {
			// If it's not possible to create the connection to the server,
			// there's no need for the program to go on.
			panic(err.Error())
		} else {
			r.connPool <- conn
		}
	}
	return nil
}

func (r *RedisWriter) Close(reason string, spider *leiogo.Spider) error {
	for i := 0; i < r.PoolSize; i++ {
		conn := <-r.connPool
		conn.Close()
	}
	return nil
}

func (r *RedisWriter) NotExists(filepath string) bool {
	conn := <-r.connPool

	exists, err := redis.Bool(conn.Do("EXISTS", filepath))
	// put back the connection
	r.connPool <- conn

	return err != nil || !exists
}

func (r *RedisWriter) WriteFile(req *leiogo.Request, res *http.Response) (info string, writerErr error) {
	filepath := req.Meta["__filepath__"].(string)

	// Create a tcp connection to the target.
	conn := <-r.connPool

	// Read all the response body into a byte array, this will later write into redis as it is.
	var body []byte
	if body, writerErr = ioutil.ReadAll(res.Body); writerErr == nil {
		// Write the bytes into redis, the key is the filepath.
		if _, writerErr = conn.Do("SET", filepath, body); writerErr == nil {
			// After writing, we should push the key into a list. This is useful when we
			// have another progress reading the data and write it to disk.
			if _, writerErr = conn.Do("RPUSH", "leiogo.redis.queue", filepath); writerErr == nil {
				writerErr = &middleware.DropTaskError{Message: "File cached completed"}
			}
		}
	}

	// put back the connection
	r.connPool <- conn

	return fmt.Sprintf("Cached %s to redis at %s", filepath, r.Addr), writerErr
}

func NewRedisWriter(addr string, size int) *RedisWriter {
	r := &RedisWriter{Addr: addr, PoolSize: size}
	r.connPool = make(chan redis.Conn, r.PoolSize)
	return r
}

type RedisFileReader struct {
	Addr string
}

func (r *RedisFileReader) ReadForever() {
	var conn redis.Conn
	var err error

	conn, err = redis.Dial("tcp", r.Addr)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	for {
		var key string
		var blpopResult []string
		var buf []byte

		blpopResult, err = redis.Strings(conn.Do("BLPOP", "leiogo.redis.queue", "0"))
		if err != nil {
			fmt.Println(err)
			return
		}
		key = blpopResult[1]

		buf, err = redis.Bytes(conn.Do("GET", key))
		if err != nil {
			fmt.Println(err)
			return
		}
		err = ioutil.WriteFile(key, buf, 0660)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Saved %s\n", key)
		}
	}
}
