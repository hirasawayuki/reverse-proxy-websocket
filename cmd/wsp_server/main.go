package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hirasawayuki/reverse-proxy-websocket/server"
)

func main() {
	configfile := flag.String("config", "wsp_server.cfg", "config file path")
	flag.Parse()

	config, err := server.LoadConfiguration(*configfile)
	if err != nil {
		log.Fatalf("Unable to load configuration : %s", err)
	}

	server := server.NewServer(config)
	server.Start()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	server.Shutdown()
}
