package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

type Listener struct {
	net.Listener
	Port int
}

func startListener(ctx context.Context, wg *sync.WaitGroup, port int, bindIP string, logger *log.Logger) (*Listener, error) {
	addr := fmt.Sprintf(":%d", port)
	if bindIP != "" {
		addr = fmt.Sprintf("%s:%d", bindIP, port)
	}
	lc := net.ListenConfig{KeepAlive: 0}
	listener, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen failed: %w", err)
	}
	l := &Listener{Listener: listener, Port: port}
	logger.Info("Port occupied successfully", "port", port)

	wg.Add(1)
	go func() {
		defer wg.Done()
		handleConnections(ctx, l, logger)
	}()

	return l, nil
}

func handleConnections(ctx context.Context, l *Listener, logger *log.Logger) {
	defer l.Close()
	for {
		select {
		case <-ctx.Done():
			logger.Debug("Stopping listener", "port", l.Port)
			return
		default:
			conn, err := l.Accept()
			if err != nil {
				if !isNetClosingError(err) {
					logger.Error("Accept error", "port", l.Port, "error", err)
				}
				continue
			}

			logger.Debug("Received connection", "port", l.Port, "remote", conn.RemoteAddr())

			go func(c net.Conn) {
				defer c.Close()
				if tcpConn, ok := c.(*net.TCPConn); ok {
					tcpConn.SetLinger(0)
				}
			}(conn)
		}
	}
}

func shutdown(ctx context.Context, listeners []*Listener, logger *log.Logger) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, l := range listeners {
		logger.Debug("Closing listener", "port", l.Port)
		if err := l.Close(); err != nil {
			logger.Error("Error closing listener", "port", l.Port, "error", err)
		}
	}
}

func isNetClosingError(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection")
}
