package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/patrickhener/gofws/webshell"
)

func main() {
	var payloadPath string

	// rand seed
	rand.Seed(time.Now().UnixNano())
	id := rand.Intn(99999-10000+1) + 10000

	// get flags
	req := flag.String("req", "", "Path to request file, hook with @@payload@@, default empty")
	payload := flag.String("payload", "", "Optional: Surrounding payload, hook with @@cmd@@, default empty")
	proxy := flag.String("proxy", "", "Optional: Proxy to use [http://127.0.0.1:8080], default empty")
	interval := flag.Int("interval", 1, "Query interval to use, default [1]")
	flag.Parse()

	// Sanity Checks
	if *req == "" {
		fmt.Println("You need to define a target request with payload hook")
		flag.Usage()
		os.Exit(1)
	}

	// Absolute path to req file from provided flag value
	reqPath, err := filepath.Abs(*req)
	if err != nil {
		fmt.Printf("Error reading the absolute path for %s: %s\n", *req, err)
		os.Exit(1)
	}

	// Set PayloadPath if provided
	if *payload != "" {
		payloadPath, err = filepath.Abs(*payload)
		if err != nil {
			fmt.Printf("Error reading the absolute path for %s: %s\n", *req, err)
			os.Exit(1)
		}
	}

	// Setup shell params
	s := webshell.WebShell{
		PayloadPath: payloadPath,
		ReqPath:     reqPath,
		Proxy:       *proxy,
		Interval:    *interval,
		Session:     id,
		Stdin:       fmt.Sprintf("/dev/shm/input.%d", id),
		Stdout:      fmt.Sprintf("/dev/shm/output.%d", id),
	}

	// Init Shell
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Init(ctx)

	// Start infinite loop
	exitCh := make(chan struct{})
	go s.Loop(ctx, cancel, exitCh)

	// Block for signal handling
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	go func() {
		for range signalCh {
			cancel()
			return
		}
	}()
	<-exitCh
}
