package server

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Host        string
	Port        int
	Timeout     int
	IdleTimeout int
	SecretKey   string
}

func (c Config) GetAddr() string {
	return c.Host + ":" + strconv.Itoa(c.Port)
}

func (c Config) GetTimeout() time.Duration {
	return time.Duration(c.Timeout) * time.Millisecond
}

func NewConfig() (config *Config) {
	config = new(Config)
	config.Host = "127.0.0.1"
	config.Port = 8080
	config.Timeout = 1000
	config.IdleTimeout = 60000
	return
}

func LoadConfiguration(path string) (config *Config, err error) {
	config = NewConfig()

	bytes, err := os.ReadFile(path)
	if err != nil {
		return
	}

	err = yaml.Unmarshal(bytes, config)
	if err != nil {
		return
	}

	return
}
