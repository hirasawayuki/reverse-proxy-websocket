package server

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/root-gg/wsp"
)

type Server struct {
	Config     *Config
	upgrader   websocket.Upgrader
	pools      []*Pool
	lock       sync.RWMutex
	done       chan struct{}
	dispatcher chan *ConnectionRequest
	server     *http.Server
}

type ConnectionRequest struct {
	connection chan *Connection
}

func NewConnectionRequest(timeout time.Duration) (cr *ConnectionRequest) {
	cr = new(ConnectionRequest)
	cr.connection = make(chan *Connection)
	return
}

func NewServer(config *Config) (server *Server) {
	rand.Seed(time.Now().Unix())

	server = new(Server)
	server.Config = config
	server.upgrader = websocket.Upgrader{}
	server.done = make(chan struct{})
	server.dispatcher = make(chan *ConnectionRequest)
	return
}

func (s *Server) Start() {
	go func() {
	L:
		for {
			select {
			case <-s.done:
				break L
			case <-time.After(5 * time.Second):
				s.clean()
			}
		}
	}()

	r := http.NewServeMux()
	r.HandleFunc("/register", s.Register)
	r.HandleFunc("/request", s.Request)
	r.HandleFunc("/status", s.status)

	go s.dispatchConnections()
	s.server = &http.Server{
		Addr:    s.Config.GetAddr(),
		Handler: r,
	}

	go func() { log.Fatal(s.server.ListenAndServe()) }()
}

func (s *Server) dispatchConnections() {
	for {
		request, ok := <-s.dispatcher
		if !ok {
			break
		}

		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, s.Config.GetTimeout())
		defer cancel()
	L:
		for {
			select {
			case <-ctx.Done():
				break L
			default:
			}

			s.lock.RLock()
			if len(s.pools) == 0 {
				s.lock.RUnlock()
				break
			}

			cases := make([]reflect.SelectCase, len(s.pools)+1)
			for i, ch := range s.pools {
				cases[i] = reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: reflect.ValueOf(ch.idle),
				}
			}
			cases[len(cases)-1] = reflect.SelectCase{
				Dir: reflect.SelectDefault,
			}
			s.lock.RUnlock()

			_, value, ok := reflect.Select(cases)
			if !ok {
				continue
			}
			connection, _ := value.Interface().(*Connection)
			if connection.Take() {
				request.connection <- connection
				break
			}
		}
		close(request.connection)
	}
}

func (s *Server) clean() {
	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.pools) == 0 {
		return
	}

	idle := 0
	busy := 0
	var pools []*Pool
	for _, pool := range s.pools {
		if pool.IsEmpty() {
			log.Printf("Removing empty connection pool : %s", pool.id)
			pool.Shutdown()
		} else {
			pools = append(pools, pool)
		}

		ps := pool.Size()
		idle += ps.Idle
		busy += ps.Busy
	}

	log.Printf("%d pools, %d idle, %d busy", len(pools), idle, busy)
	s.pools = pools
}

func (s *Server) Request(w http.ResponseWriter, r *http.Request) {
	dstURL := r.Header.Get("X-PROXY-DESTINATION")
	if dstURL == "" {
		wsp.ProxyError(w, "Missing X-PROXY-DESTINATION header")
		return
	}
	URL, err := url.Parse(dstURL)
	if err != nil {
		wsp.ProxyError(w, "Unable to parse X-PROXY-DESTINATION header")
		return
	}
	r.URL = URL

	log.Printf("[%s] %s", r.Method, r.URL.String())

	if len(s.pools) == 0 {
		wsp.ProxyError(w, "No proxy available")
		return
	}

	request := NewConnectionRequest(s.Config.GetTimeout())
	s.dispatcher <- request
	connection := <-request.connection
	if connection == nil {
		wsp.ProxyError(w, "Unable to get a proxy connection")
		return
	}

	if err := connection.proxyRequest(w, r); err != nil {
		log.Println(err)
		connection.Close()
		wsp.ProxyError(w, err)
	}
}

func (s *Server) Register(w http.ResponseWriter, r *http.Request) {
	secretKey := r.Header.Get("X-SECRET-KEY")
	if secretKey != s.Config.SecretKey {
		wsp.ProxyErrorf(w, "Invalid X-SECRET-KEY")
		return
	}

	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		wsp.ProxyError(w, "HTTP upgrade error : %v", err)
		return
	}

	_, greeting, err := ws.ReadMessage()
	if err != nil {
		wsp.ProxyError(w, "Unable to read greeting message : %s", err)
		ws.Close()
		return
	}

	split := strings.Split(string(greeting), "_")
	id := PoolID(split[0])
	size, err := strconv.Atoi(split[1])
	if err != nil {
		wsp.ProxyErrorf(w, "Unable to parse greeting message : %s", err)
		ws.Close()
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	var pool *Pool
	for _, p := range s.pools {
		if p.id == id {
			pool = p
			break
		}
	}
	if pool == nil {
		pool = NewPool(s, id)
		s.pools = append(s.pools, pool)
	}

	pool.size = size
	pool.Register(ws)
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}

func (s *Server) Shutdown(w http.ResponseWriter, r *http.Request) {
	close(s.done)
	close(s.dispatcher)
	for _, pool := range s.pools {
		pool.Shutdown()
	}
	s.clean()
}
