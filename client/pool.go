package client

import "sync"

type Pool struct {
	client      *Client
	target      string
	secretKey   string
	connections []*Connection
	lock        sync.RWMutex
	done        chan struct{}
}
