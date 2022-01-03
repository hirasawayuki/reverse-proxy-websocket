package client

import (
	"net/http"

	"github.com/gorilla/websocket"
)

type Client struct {
	Config *Config
	client *http.Client
	dialer *websocket.Dialer
	pools  map[string]*Pool
}

func NewClient(config *Config) (c *Client) {
	c = new(Client)
	c.Config = config
	c.client = &http.Client{}
	c.dialer = &websocket.Dialer{}
	c.pools = make(map[string]*Pool)
	return
}
