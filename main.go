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
	"runtime"
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
	configPath := flag.String("config", defaultConfigPath(exe), "configuration file (default: beside executable)")
	listen := flag.String("listen", "", "override the configured host:port for this run")
	desktop := flag.Bool("desktop", false, "open the small desktop control window")
	headless := flag.Bool("headless", false, "run without a desktop or browser")
	packageStop := flag.Bool("package-stop", false, "stop this installed application during package removal or upgrade")
	serviceSetup := flag.String("service-setup", "", "prepare and verify optional unattended service (administrator only)")
	service := flag.Bool("service", false, "run under the native service manager")
	action := flag.String("control", "", "status, start, stop, restart, updates, enable or disable")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("DayZ Bank Viewer", version)
		return nil
	}
	if *packageStop {
		return stopPackagedProcesses()
	}
	if *serviceSetup != "" {
		return configureUnattended(*serviceSetup)
	}
	explicitConfig := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			explicitConfig = true
		}
	})
	if *service && !explicitConfig {
		*configPath = systemConfigPath()
	}
	if *configPath == "" {
		return errors.New("could not locate your configuration folder")
	}
	abs, err := filepath.Abs(*configPath)
	if err != nil {
		return err
	}
	if resolved, e := filepath.EvalSymlinks(abs); e == nil {
		abs = resolved
	} else if parent, e := filepath.EvalSymlinks(filepath.Dir(abs)); e == nil {
		abs = filepath.Join(parent, filepath.Base(abs))
	}
	if *action != "" {
		return controlAction(abs, *action)
	}
	if *desktop || (!*headless && !*service && len(os.Args) == 1 && (runtime.GOOS == "windows" || os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "")) {
		return desktopWindow(abs)
	}
	if *service {
		if runtime.GOOS == "windows" {
			f, e := os.OpenFile(abs+".log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if e != nil {
				return e
			}
			defer f.Close()
			log.SetOutput(f)
		}
		return serviceHost(func() error { return serve(abs, *listen) })
	}
	return serve(abs, *listen)
}
func serve(abs, override string) error {
	c, err := loadConfig(abs)
	if err != nil {
		return fmt.Errorf("read configuration: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return err
	}
	unlock, err := acquireLock(abs + ".lock")
	if err != nil {
		return err
	}
	defer unlock()
	address := c.Listen
	if override != "" {
		address = override
	}
	listener, err := bindAvailable(address)
	if err != nil {
		return err
	}
	defer listener.Close()
	tcp := listener.Addr().(*net.TCPAddr)
	m := newManager(c, abs)
	defer m.Close()
	srv := &http.Server{Handler: handler(m, tcp.IP.IsLoopback()), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if serviceStop != nil {
		go func() {
			select {
			case <-serviceStop:
				cancel()
			case <-ctx.Done():
			}
		}()
	}
	go m.Schedule(ctx)
	m.Refresh("")
	log.Printf("DayZ Bank Viewer %s · http://%s · configuration: %s", version, listener.Addr(), abs)
	if !tcp.IP.IsLoopback() {
		log.Print("Listening beyond localhost. This interface has no built-in login; restrict access to trusted administrators with your firewall or an authenticated reverse proxy.")
	}
	failures := make(chan error, 1)
	go func() { failures <- srv.Serve(listener) }()
	host := tcp.IP.String()
	if tcp.IP.IsUnspecified() {
		host = "127.0.0.1"
	}
	cleanup, err := publishInstance(abs, "http://"+net.JoinHostPort(host, fmt.Sprint(tcp.Port)), cancel)
	if err != nil {
		srv.Close()
		return err
	}
	defer cleanup()
	if serviceReady != nil {
		serviceReady()
	}
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
		interactive := len(os.Args) == 1
		for _, arg := range os.Args[1:] {
			if arg == "-desktop" || (arg == "-service-setup" && runtime.GOOS == "windows") {
				interactive = true
			}
		}
		if interactive && (runtime.GOOS == "windows" || os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "") {
			nativeError(err.Error())
		}
		os.Exit(1)
	}
}

var serviceStop <-chan struct{}

var serviceReady func()
