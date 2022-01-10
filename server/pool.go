package server

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Pool struct {
	server      *Server
	id          PoolID
	size        int
	connections []*Connection
	idle        chan *Connection
	done        bool
	lock        sync.Mutex
}

type PoolID string

func NewPool(server *Server, id PoolID) *Pool {
	p := new(Pool)
	p.server = server
	p.id = id
	p.idle = make(chan *Connection)

	return p
}

func (pool *Pool) Register(ws *websocket.Conn) {
	pool.lock.Lock()
	defer pool.lock.Unlock()

	if pool.done {
		return
	}

	log.Printf("Register new connection from %s", pool.id)
	connection := NewConnection(pool, ws)
	pool.connections = append(pool.connections, connection)
}

func (pool *Pool) Offer(connection *Connection) {
	pool.idle <- connection
}

func (pool *Pool) Clean() {
	idle := 0
	var connections []*Connection

	for _, connection := range pool.connections {
		connection.lock.Lock()
		if connection.status == Idle {
			idle++
			if idle > pool.size {
				if int(time.Now().Sub(connection.idleSince).Seconds())*1000 > pool.server.Config.IdleTimeout {
					connection.close()
				}
			}
		}
		connection.lock.Unlock()
		if connection.status == Closed {
			continue
		}

		connections = append(connections, connection)
	}
	pool.connections = connections
}

func (pool *Pool) IsEmpty() bool {
	pool.lock.Lock()
	defer pool.lock.Unlock()

	pool.Clean()
	return len(pool.connections) == 0
}

func (pool *Pool) Shutdown() {
	pool.lock.Lock()
	defer pool.lock.Unlock()

	pool.done = true

	for _, connection := range pool.connections {
		connection.Close()
	}

	pool.Clean()
}

type PoolSize struct {
	Idle   int
	Busy   int
	Closed int
}

func (pool *Pool) Size() (ps *PoolSize) {
	pool.lock.Lock()
	defer pool.lock.Unlock()

	ps = new(PoolSize)
	for _, connection := range pool.connections {
		if connection.status == Idle {
			ps.Idle++
		} else if connection.status == Busy {
			ps.Busy++
		} else if connection.status == Closed {
			ps.Closed++
		}
	}

	return
}
