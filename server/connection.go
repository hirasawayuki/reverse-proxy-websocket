package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/root-gg/wsp"
)

type ConnectionsStatus int

const (
	Idle ConnectionsStatus = iota
	Busy
	Closed
)

type Connection struct {
	pool         *Pool
	ws           *websocket.Conn
	status       ConnectionsStatus
	idleSince    time.Time
	lock         sync.Mutex
	nextResponse chan chan io.Reader
}

func NewConnection(pool *Pool, ws *websocket.Conn) *Connection {
	c := new(Connection)
	c.pool = pool
	c.ws = ws
	c.nextResponse = make(chan chan io.Reader)
	c.status = Idle
	c.Release()
	go c.read()

	return c
}

func (connection *Connection) read() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Websocket crash recovered : %s", r)
		}
		connection.Close()
	}()

	for {
		if connection.status == Closed {
			break
		}

		_, reader, err := connection.ws.NextReader()
		if err != nil {
			break
		}

		if connection.status != Busy {
			break
		}

		c := <-connection.nextResponse
		if c == nil {
			break
		}

		c <- reader
		<-c
	}
}

func (connection *Connection) proxyRequest(w http.ResponseWriter, r *http.Request) (err error) {
	log.Printf("proxy request to %s", connection.pool.id)

	jsonReq, err := json.Marshal(wsp.SerializeHTTPRequest(r))
	if err != nil {
		return fmt.Errorf("unable to serialize request : %w", err)
	}

	if err := connection.ws.WriteMessage(websocket.TextMessage, jsonReq); err != nil {
		return fmt.Errorf("unable to write request : %w", err)
	}

	bodyWriter, err := connection.ws.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return fmt.Errorf("unable to get request body writer : %w", err)
	}

	if _, err := io.Copy(bodyWriter, r.Body); err != nil {
		return fmt.Errorf("unble to pipe request body : %w", err)
	}
	if err := bodyWriter.Close(); err != nil {
		return fmt.Errorf("unable to pipe request body (close) : %w", err)
	}

	responseChannel := make(chan (io.Reader))
	connection.nextResponse <- responseChannel
	responseReader, ok := <-responseChannel
	if responseReader == nil {
		if ok {
			close(responseChannel)
		}

		return fmt.Errorf("unable to get http response reader: %w", err)
	}

	jsonResponse, err := io.ReadAll(responseReader)
	if err != nil {
		close(responseChannel)
		return fmt.Errorf("unable to read http response : %w", err)
	}

	close(responseChannel)

	httpResponse := new(wsp.HTTPResponse)
	if err := json.Unmarshal(jsonResponse, httpResponse); err != nil {
		return fmt.Errorf("unable to unserialize http response : %w", err)
	}

	for header, values := range httpResponse.Header {
		for _, value := range values {
			w.Header().Add(header, value)
		}
	}
	w.WriteHeader(httpResponse.StatusCode)

	responseBodyChannel := make(chan (io.Reader))
	connection.nextResponse <- responseBodyChannel
	responseBodyReader, ok := <-responseBodyChannel
	if responseBodyReader == nil {
		if ok {
			close(responseChannel)
		}
		return fmt.Errorf("unable to get http response body reader : %w", err)
	}

	if _, err := io.Copy(w, responseBodyReader); err != nil {
		close(responseBodyChannel)
		return fmt.Errorf("unable to pipe response body : %w", err)
	}

	close(responseBodyChannel)
	connection.Release()

	return
}

func (connection *Connection) Take() bool {
	connection.lock.Unlock()
	defer connection.lock.Unlock()

	if connection.status == Closed {
		return false
	}

	if connection.status == Busy {
		return false
	}

	connection.status = Busy
	return true
}

func (connection *Connection) Release() {
	connection.lock.Lock()
	defer connection.lock.Unlock()

	if connection.status == Closed {
		return
	}

	connection.idleSince = time.Now()
	connection.status = Idle

	go connection.pool.Offer(connection)
}

func (connection *Connection) Close() {
	connection.lock.Lock()
	defer connection.lock.Unlock()

	connection.close()
}

func (connection *Connection) close() {
	if connection.status == Closed {
		return
	}

	log.Printf("Closing connection from %s", connection.pool.id)
	defer func() { connection.status = Closed }()

	close(connection.nextResponse)
	connection.ws.Close()
}
