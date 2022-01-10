package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hirasawayuki/reverse-proxy-websocket/wsp"
)

const (
	CONNECTING = iota
	IDLE
	RUNNING
)

type Connection struct {
	pool   *Pool
	ws     *websocket.Conn
	status int
}

func NewConnection(pool *Pool) *Connection {
	c := new(Connection)
	c.pool = pool
	c.status = CONNECTING
	return c
}

func (connection *Connection) Connect(ctx context.Context) (err error) {
	log.Printf("Connecting to %s", connection.pool.target)
	connection.ws, _, err = connection.pool.client.dialer.DialContext(
		ctx,
		connection.pool.target,
		http.Header{"X-SECRET-KEY": {connection.pool.secretKey}},
	)

	if err != nil {
		return err
	}

	log.Printf("Connected to %s", connection.pool.target)

	greeting := fmt.Sprintf(
		"%s_%d",
		connection.pool.client.Config.ID,
		connection.pool.client.Config.PoolIdleSize,
	)
	if err := connection.ws.WriteMessage(websocket.TextMessage, []byte(greeting)); err != nil {
		log.Println("greeting error :", err)
		connection.Close()
		return err
	}

	go connection.serve(ctx)
	return
}

func (connection *Connection) serve(ctx context.Context) {
	defer connection.Close()

	go func() {
		for {
			time.Sleep(30 * time.Second)
			err := connection.ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second))
			if err != nil {
				connection.Close()
			}
		}
	}()

	for {
		connection.status = IDLE
		_, jsonRequest, err := connection.ws.ReadMessage()
		if err != nil {
			log.Println("Unable to read request", err)
			break
		}

		connection.status = RUNNING

		go connection.pool.connector(ctx)

		httpRequest := new(wsp.HTTPRequest)
		err = json.Unmarshal(jsonRequest, httpRequest)
		if err != nil {
			connection.error(fmt.Sprintf("Unable to deserialize json http request : %s\n", err))
			break
		}

		req, err := wsp.UnserializeHTTPRequest(httpRequest)
		if err != nil {
			connection.error(fmt.Sprintf("Unable to deserialize http request : %v\n", err))
			break
		}

		log.Printf("[%s] %s", req.Method, req.URL.String())

		_, bodyReader, err := connection.ws.NextReader()
		if err != nil {
			log.Printf("Unable to get response body reader : %v", err)
			break
		}
		req.Body = io.NopCloser(bodyReader)

		resp, err := connection.pool.client.client.Do(req)
		if err != nil {
			err = connection.error(fmt.Sprintf("Unable to execute request : %v\n", err))
			if err != nil {
				break
			}
			continue
		}

		jsonResponse, err := json.Marshal(wsp.SerializeHTTPResponse(resp))
		if err != nil {
			err = connection.error(fmt.Sprintf("Unable to serialize response : %v\n", err))
			if err != nil {
				break
			}
			continue
		}

		err = connection.ws.WriteMessage(websocket.TextMessage, jsonResponse)
		if err != nil {
			log.Printf("Unable to write response : %v", err)
			break
		}

		bodyWriter, err := connection.ws.NextWriter(websocket.BinaryMessage)
		if err != nil {
			log.Printf("Unable to get response body writer : %v", err)
			break
		}
		_, err = io.Copy(bodyWriter, resp.Body)
		if err != nil {
			log.Printf("Unable to get pipe response body : %v", err)
			break
		}
		bodyWriter.Close()
	}
}

func (connection *Connection) error(msg string) (err error) {
	resp := wsp.NewHTTPResponse()
	resp.StatusCode = 527

	log.Println(msg)
	resp.ContentLength = int64(len(msg))

	jsonResponse, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Unable to serialize response : %v", err)
		return
	}

	err = connection.ws.WriteMessage(websocket.TextMessage, jsonResponse)
	if err != nil {
		log.Printf("Unable to write response body : %v", err)
		return
	}

	err = connection.ws.WriteMessage(websocket.BinaryMessage, []byte(msg))
	if err != nil {
		log.Printf("Unable to write response body : %v", err)
		return
	}

	return
}

func (connection *Connection) Close() {
	connection.pool.lock.Lock()

	defer connection.pool.lock.Unlock()
	connection.pool.remove(connection)
	connection.ws.Close()
}
