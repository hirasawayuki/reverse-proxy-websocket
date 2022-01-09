package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hirasawayuki/reverse-proxy-websocket/client"
)

func main() {
	ctx := context.Background()

	configFile := flag.String("config", "wsp_client.cfg", "config file path")
	flag.Parse()

	config, err := client.LoadConfiguration(*configFile)
	if err != nil {
		log.Fatalf("Unable to load configuration : %s", err)
	}

	proxy := client.NewClient(config)
	proxy.Start(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	proxy.Shutdown()
}
