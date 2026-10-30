package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"
)

func run() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	configPath := flag.String("config", filepath.Join(filepath.Dir(exe), "config.json"), "configuration file (default: beside executable)")
	listen := flag.String("listen", "", "override the configured host:port for this run")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("DayZ Bank Viewer", version)
		return nil
	}
	abs, err := filepath.Abs(*configPath)
	if err != nil {
		return err
	}
	c, err := loadConfig(abs)
	if err != nil {
		return fmt.Errorf("read configuration: %w", err)
	}
	address := c.Listen
	if *listen != "" {
		address = *listen
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %w. Another application may be using this port; set listen in config.json or use -listen 127.0.0.1:20785", address, err)
	}
	defer listener.Close()
	tcp := listener.Addr().(*net.TCPAddr)
	m := newManager(c, abs)
	defer m.Close()
	srv := &http.Server{Handler: handler(m, tcp.IP.IsLoopback()), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go m.Schedule(ctx)
	m.Refresh("")
	log.Printf("DayZ Bank Viewer %s · http://%s · configuration: %s", version, listener.Addr(), abs)
	if !tcp.IP.IsLoopback() {
		log.Print("Listening beyond localhost. This interface has no built-in login; restrict access to trusted administrators with your firewall or an authenticated reverse proxy.")
	}
	failures := make(chan error, 1)
	go func() { failures <- srv.Serve(listener) }()
	select {
	case err := <-failures:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}
func main() {
	if err := run(); err != nil {
		log.Print(strings.TrimSpace(err.Error()))
		os.Exit(1)
	}
}
