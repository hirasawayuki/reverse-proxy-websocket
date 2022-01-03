package client

import "github.com/gorilla/websocket"

type Connection struct {
	pool   *Pool
	ws     *websocket.Conn
	status int
}
