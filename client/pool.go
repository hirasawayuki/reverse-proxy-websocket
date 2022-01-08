package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type Pool struct {
	lock sync.RWMutex

	client      *Client
	target      string
	secretKey   string
	connections []*Connection
	done        chan struct{}
}

func NewPool(client *Client, target string, secretKey string) (pool *Pool) {
	pool = new(Pool)
	pool.client = client
	pool.target = target
	pool.connections = make([]*Connection, 0)
	pool.secretKey = secretKey
	pool.done = make(chan struct{})
	return
}

func (pool *Pool) Start(ctx context.Context) {
	pool.connector(ctx)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
	L:
		for {
			select {
			case <-pool.done:
				break L
			case <-ticker.C:
				pool.connector(ctx)
			}
		}
	}()
}

func (pool *Pool) connector(ctx context.Context) {
	pool.lock.Lock()
	defer pool.lock.Unlock()

	poolSize := pool.Size()

	toCreate := pool.client.Config.PoolIdleSize - poolSize.idle
	if poolSize.total == 0 {
		toCreate = 1
	}

	if poolSize.total+toCreate > pool.client.Config.PoolMaxSize {
		toCreate = pool.client.Config.PoolMaxSize - poolSize.total
	}

	for i := 0; i < toCreate; i++ {
		conn := NewConnection(pool)
		pool.connections = append(pool.connections, conn)

		go func() {
			err := conn.Connect(ctx)
			if err != nil {
				log.Printf("Unable to connect to %s : %s", pool.target, err)

				pool.lock.Lock()
				defer pool.lock.Unlock()
				pool.remove(conn)
			}
		}()
	}
}

func (pool *Pool) add(conn *Connection) {
	pool.connections = append(pool.connections, conn)
}

func (pool *Pool) remove(conn *Connection) {
	var filtered []*Connection
	for _, c := range pool.connections {
		if conn != c {
			filtered = append(filtered, c)
		}
	}
	pool.connections = filtered
}

func (pool *Pool) Shutdown() {
	close(pool.done)
	for _, conn := range pool.connections {
		conn.Close()
	}
}

type PoolSize struct {
	connecting int
	idle       int
	running    int
	total      int
}

func (poolSize *PoolSize) String() string {
	return fmt.Sprintf("Connecting %d, idle %d, running %d, total %d", poolSize.connecting, poolSize.idle, poolSize.running, poolSize.total)
}

func (pool *Pool) Size() (poolSize *PoolSize) {
	poolSize = new(PoolSize)
	poolSize.total = len(pool.connections)
	for _, connection := range pool.connections {
		switch connection.status {
		case CONNECTING:
			poolSize.connecting++
		case IDLE:
			poolSize.idle++
		case RUNNING:
			poolSize.running++
		}
	}

	return
}
