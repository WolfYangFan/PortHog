package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
)

type Listener struct {
	net.Listener
	Port int
}

const (
	envDebugMode    = "PORTHOG_DEBUG" // 环境变量名称
	defaultLogLevel = log.InfoLevel   // 默认日志级别
)

var (
	Version   = "dev"     // 默认为开发版本
	GitCommit = "unknown" // 默认我也不知道
)

func main() {
	var (
		ports    = flag.String("p", "", "Port specification (e.g. 8080,9000-9005)")
		debug    = flag.Bool("debug", false, "Enable debug mode")
		logLevel = flag.String("level", "", "Set log level (debug, info, warn, error)")
	)

	flag.Parse()

	logger := setupLogger(*debug, *logLevel)
	logger.Info("PortHog started", "version", Version, "commit", GitCommit, "pid", os.Getpid())

	if *ports == "" {
		logger.Fatal("No ports specified. Use -p to specify ports.")
	}

	portList, err := parsePorts(*ports)
	if err != nil {
		logger.Fatal("Failed to parse ports", "error", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	listeners := make([]*Listener, 0, len(portList))

	// 启动端口监听
	for _, port := range portList {
		l, err := startListener(ctx, &wg, port, logger)
		if err != nil {
			logger.Error("Failed to start listener",
				"port", port,
				"error", err)
			continue
		}
		listeners = append(listeners, l)
	}

	if len(listeners) == 0 {
		logger.Fatal("No valid ports available")
	}

	// 处理系统信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号或上下文取消
	select {
	case sig := <-sigCh:
		logger.Info("Received signal - initiating graceful shutdown...", "signal", sig)
		cancel()
	case <-ctx.Done():
	}

	// 优雅关闭
	shutdown(ctx, listeners, logger)
	wg.Wait()

	logger.Info("Shutdown completed")
}

func startListener(ctx context.Context, wg *sync.WaitGroup, port int, logger *log.Logger) (*Listener, error) {
	addr := fmt.Sprintf(":%d", port)
	lc := net.ListenConfig{
		KeepAlive: 0, // 禁用 keepalive
	}

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
					logger.Error("Accept error",
						"port", l.Port,
						"error", err)
				}
				continue
			}

			logger.Debug("Received connection",
				"port", l.Port,
				"remote", conn.RemoteAddr())

			// 立即关闭连接
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
			logger.Error("Error closing listener",
				"port", l.Port,
				"error", err)
		}
	}
}

func isNetClosingError(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection")
}

// 解析端口参数
func parsePorts(ports string) ([]int, error) {
	var portList []int

	// 按逗号分割
	parts := strings.Split(ports, ",")
	for _, part := range parts {
		if strings.Contains(part, "-") {
			// 处理端口范围
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid port range: %s", part)
			}

			start, err := strconv.Atoi(rangeParts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid start port: %s", rangeParts[0])
			}

			end, err := strconv.Atoi(rangeParts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid end port: %s", rangeParts[1])
			}

			if start > end {
				return nil, fmt.Errorf("start port must be less than or equal to end port: %s", part)
			}

			for port := start; port <= end; port++ {
				portList = append(portList, port)
			}
		} else {
			// 处理单个端口
			port, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid port: %s", part)
			}
			portList = append(portList, port)
		}
	}

	return portList, nil
}

func setupLogger(cmdDebug bool, cmdLogLevel string) *log.Logger {
	logger := log.New(os.Stdout)

	// 输出时间戳
	logger.SetReportTimestamp(true)

	// 优先级：命令行参数 > 环境变量 > 默认
	switch {
	case cmdLogLevel != "":
		setLevelFromString(logger, cmdLogLevel)
	case cmdDebug || os.Getenv(envDebugMode) != "":
		logger.SetLevel(log.DebugLevel)
	default:
		logger.SetLevel(defaultLogLevel)
	}

	if os.Getenv("NO_COLOR") != "1" {
		logger.SetFormatter(log.TextFormatter)
	} else {
		logger.SetFormatter(log.LogfmtFormatter)
	}

	return logger
}

func setLevelFromString(logger *log.Logger, level string) {
	switch strings.ToLower(level) {
	case "debug":
		logger.SetLevel(log.DebugLevel)
	case "info":
		logger.SetLevel(log.InfoLevel)
	case "warn", "warning":
		logger.SetLevel(log.WarnLevel)
	case "error":
		logger.SetLevel(log.ErrorLevel)
	default:
		logger.Warn("Invalid log level, using default", "input", level)
		logger.SetLevel(defaultLogLevel)
	}
}
