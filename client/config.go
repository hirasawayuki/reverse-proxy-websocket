package client

import (
	"os"

	uuid "github.com/nu7hatch/gouuid"
	"gopkg.in/yaml.v2"
)

type Config struct {
	ID           string
	Targets      []string
	PoolIdleSize int
	PoolMaxSize  int
	SecretKey    string
}

func NewConfig() (config *Config) {
	config = new(Config)

	id, err := uuid.NewV4()
	if err != nil {
		panic(err)
	}
	config.ID = id.String()
	config.Targets = []string{"ws://127.0.0.1:8000/register"}
	config.PoolIdleSize = 10
	config.PoolMaxSize = 100
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
