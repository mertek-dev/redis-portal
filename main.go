package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var logger *slog.Logger

func init() {
	// Create a custom handler with better formatting
	handler := &PrettyHandler{
		Handler: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}),
	}
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// PrettyHandler wraps a slog.Handler to provide better formatting
type PrettyHandler struct {
	slog.Handler
}

func (h *PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	// Add visual separators and better formatting
	switch r.Level {
	case slog.LevelInfo:
		if r.Message == "Starting TCP Portal" {
			fmt.Println("\n" + strings.Repeat("=", 60))
			fmt.Printf("🚀 %s\n", r.Message)
			fmt.Println(strings.Repeat("=", 60))
		} else if r.Message == "Environment Detection" {
			fmt.Println("\n📋 Environment Detection")
			fmt.Println(strings.Repeat("-", 30))
		} else if r.Message == "Network Information" {
			fmt.Println("\n🌐 Network Information")
			fmt.Println(strings.Repeat("-", 30))
		} else if strings.Contains(r.Message, "connection") {
			fmt.Printf("🔗 %s\n", r.Message)
		} else if strings.Contains(r.Message, "authentication") {
			fmt.Printf("🔐 %s\n", r.Message)
		} else {
			fmt.Printf("ℹ️  %s\n", r.Message)
		}
	case slog.LevelWarn:
		fmt.Printf("⚠️  %s\n", r.Message)
	case slog.LevelError:
		fmt.Printf("❌ %s\n", r.Message)
	}

	// Add key-value pairs with better formatting
	r.Attrs(func(a slog.Attr) bool {
		if a.Key != "" && a.Value.Kind() != slog.KindAny {
			fmt.Printf("   %s: %v\n", a.Key, a.Value)
		}
		return true
	})

	return nil
}

func isRunningInDocker() bool {
	// Method 1: Check for .dockerenv file
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Method 2: Check cgroup for docker
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") || strings.Contains(content, "containerd") {
			return true
		}
	}

	// Method 3: Check if we're PID 1 (common in containers)
	if os.Getpid() == 1 {
		// Additional check: look for container-specific environment
		if os.Getenv("container") != "" || os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
			return true
		}
	}

	return false
}

func validateListenAddress(listenAddr string) {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		fmt.Printf("⚠️  Warning: Could not parse listen address: %v\n", err)
		return
	}

	isDocker := isRunningInDocker()

	if isDocker {
		fmt.Println("🐳 Runtime: Docker container detected")
	} else {
		fmt.Println("🖥️  Runtime: Native/host environment")
	}

	// Check for problematic configurations
	if isDocker {
		switch host {
		case "127.0.0.1", "localhost":
			fmt.Printf("⚠️  WARNING: Listening on %s inside Docker!\n", host)
			fmt.Println("   This will NOT be accessible from outside the container!")
			fmt.Printf("   Consider using -listen 0.0.0.0:%s or -listen :%s\n", port, port)
		case "0.0.0.0", "":
			fmt.Println("✅ Good: Listening on all interfaces (accessible from outside container)")
		default:
			fmt.Printf("ℹ️  INFO: Listening on specific interface: %s\n", host)
			fmt.Println("   Make sure this interface is accessible from outside the container")
		}

		// Docker port mapping reminder
		fmt.Printf("💡 Don't forget Docker port mapping: -p HOST_PORT:%s\n", port)
	} else {
		switch host {
		case "127.0.0.1", "localhost":
			fmt.Println("ℹ️  Listening on localhost only (not accessible from other machines)")
		case "0.0.0.0", "":
			fmt.Println("✅ Listening on all interfaces (accessible from network)")
		default:
			fmt.Printf("ℹ️  Listening on specific interface: %s\n", host)
		}
	}
}

func getOutboundIP() string {
	// Method 1: Connect to a remote address to determine local IP
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		slog.Warn("Could not determine outbound IP via UDP method", "error", err)
	} else {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP.String()
	}

	// Method 2: Get all network interfaces and find the first non-loopback IP
	interfaces, err := net.Interfaces()
	if err != nil {
		slog.Warn("Could not get network interfaces", "error", err)
		return "unknown"
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			if ip.To4() != nil {
				return ip.String()
			}
		}
	}

	return "unknown"
}

func getPublicIP() string {
	services := []string{
		"https://api.ipify.org",
		"https://icanhazip.com",
		"https://ifconfig.me",
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for _, service := range services {
		resp, err := client.Get(service)
		if err != nil {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			continue
		}

		ip := strings.TrimSpace(string(body))
		// Validate it looks like an IP
		if net.ParseIP(ip) != nil {
			return ip
		}
	}

	return "unknown"
}

func displayNetworkInfo(listenAddr string) {
	// Parse listen address to get port
	_, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		port = "8080" // default fallback
	}

	fmt.Printf("Localhost: 127.0.0.1:%s\n", port)

	localIP := getOutboundIP()
	if localIP != "unknown" {
		fmt.Printf("Local Network: %s:%s\n", localIP, port)
	}

	// Try to get public IP (with timeout to avoid blocking startup)
	go func() {
		publicIP := getPublicIP()
		if publicIP != "unknown" {
			fmt.Printf("External Access: %s:%s\n", publicIP, port)
		} else {
			fmt.Println("External Access: Could not determine (check firewall/NAT)")
		}
	}()
}

func GenerateSecurePassword(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	password := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))

	for i := range length {
		randomIndex, _ := rand.Int(rand.Reader, charsetLen)
		password[i] = charset[randomIndex.Int64()]
	}
	return string(password)
}

type Config struct {
	ListenAddr     string
	TargetAddr     string
	Username       string
	Password       string
	TargetUsername string
	TargetPassword string
}

type Server struct {
	listener       net.Listener
	config         *Config
	activeConns    sync.WaitGroup
	shutdown       chan struct{}
	clientConns    sync.Map // map[net.Conn]bool to track active client connections
	shutdownSignal chan struct{}
}

func NewServer(config *Config) *Server {
	return &Server{
		config:         config,
		shutdown:       make(chan struct{}),
		shutdownSignal: make(chan struct{}),
	}
}

func (s *Server) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("error listening on %s: %v", s.config.ListenAddr, err)
	}

	go s.acceptConnections()
	return nil
}

func (s *Server) acceptConnections() {
	for {
		select {
		case <-s.shutdown:
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				select {
				case <-s.shutdown:
					return
				default:
					slog.Error("Error accepting connection", "error", err)
					continue
				}
			}

			s.activeConns.Add(1)
			go func() {
				defer s.activeConns.Done()
				handleConnection(conn, s.config, s)
			}()
		}
	}
}

func (s *Server) Shutdown() {
	slog.Info("Shutting down TCP Portal")

	close(s.shutdown)

	if s.listener != nil {
		s.listener.Close()
	}

	// Notify all active clients that server is shutting down
	slog.Info("Notifying active clients of server shutdown")
	s.clientConns.Range(func(key, value interface{}) bool {
		if conn, ok := key.(net.Conn); ok {
			// Send Redis error response indicating server shutdown
			sendRedisError(conn, "Server is shutting down")
			conn.Close()
		}
		return true
	})

	// Wait for connections to close with timeout
	shutdownTimeout := 10 * time.Second
	slog.Info("Waiting for active connections to close", "timeout", shutdownTimeout)

	done := make(chan struct{})
	go func() {
		s.activeConns.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("All connections closed gracefully. Server stopped.")
	case <-time.After(shutdownTimeout):
		slog.Warn("Shutdown timeout reached. Force closing remaining connections.")
		// Force close any remaining connections
		s.clientConns.Range(func(key, value interface{}) bool {
			if conn, ok := key.(net.Conn); ok {
				conn.Close()
			}
			return true
		})
		slog.Info("Server stopped (forced shutdown).")
	}
}

func parseArgs() *Config {
	var config Config

	flag.StringVar(&config.ListenAddr, "listen", ":8080", "Local address to listen on (e.g., :8080)")
	flag.StringVar(&config.ListenAddr, "l", ":8080", "Local address to listen on (e.g., :8080)")
	flag.StringVar(&config.TargetAddr, "target", "", "Target address to proxy to (e.g., redis:6379)")
	flag.StringVar(&config.TargetAddr, "t", "", "Target address to proxy to (e.g., redis:6379)")
	flag.StringVar(&config.Username, "username", "", "Override auth username. Default: ''")
	flag.StringVar(&config.Username, "u", "", "Override auth username. Default: ''")
	flag.StringVar(&config.Password, "password", "", "Override auth password. Default is random")
	flag.StringVar(&config.Password, "p", "", "Override auth password. Default is random")
	flag.StringVar(&config.TargetUsername, "target-username", "", "Target Redis username for authentication")
	flag.StringVar(&config.TargetUsername, "tu", "", "Target Redis username for authentication")
	flag.StringVar(&config.TargetPassword, "target-password", "", "Target Redis password for authentication")
	flag.StringVar(&config.TargetPassword, "tp", "", "Target Redis password for authentication")

	flag.Parse()

	if config.TargetAddr == "" {
		fmt.Fprintf(os.Stderr, "Error: target address is required\n")
		flag.Usage()
		os.Exit(1)
	}

	if config.Password == "" {
		config.Password = GenerateSecurePassword(16)
	}

	return &config
}

func parseRedisCommand(reader *bufio.Reader) ([]string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "*") {
		return nil, fmt.Errorf("invalid redis command format")
	}

	numArgsStr := line[1:]
	numArgs, err := strconv.Atoi(numArgsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid number of arguments: %v", err)
	}

	args := make([]string, numArgs)
	for i := range numArgs {
		lengthLine, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		lengthLine = strings.TrimSpace(lengthLine)
		if !strings.HasPrefix(lengthLine, "$") {
			return nil, fmt.Errorf("invalid argument length format")
		}

		length, err := strconv.Atoi(lengthLine[1:])
		if err != nil {
			return nil, fmt.Errorf("invalid argument length: %v", err)
		}

		argData := make([]byte, length)
		_, err = io.ReadFull(reader, argData)
		if err != nil {
			return nil, err
		}

		_, err = reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		args[i] = string(argData)
	}

	return args, nil
}

func sendRedisResponse(conn net.Conn, response string) error {
	_, err := conn.Write([]byte(response))
	return err
}

func sendRedisOK(conn net.Conn) error {
	return sendRedisResponse(conn, "+OK\r\n")
}

func sendRedisError(conn net.Conn, message string) error {
	return sendRedisResponse(conn, fmt.Sprintf("-ERR %s\r\n", message))
}

func authenticateRedisClient(conn net.Conn, username, password string) (bool, []byte) {
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	defer conn.SetReadDeadline(time.Time{})

	reader := bufio.Reader{}
	reader.Reset(conn)

	for {
		args, err := parseRedisCommand(&reader)
		if err != nil {
			slog.Error("Error parsing Redis command", "client", conn.RemoteAddr(), "error", err)
			sendRedisError(conn, "Protocol error")
			return false, nil
		}

		if len(args) == 0 {
			sendRedisError(conn, "Protocol error")
			return false, nil
		}

		command := strings.ToUpper(args[0])

		switch command {
		case "AUTH":
			if len(args) < 2 {
				sendRedisError(conn, "wrong number of arguments for 'auth' command")
				continue
			}

			var providedUsername, providedPassword string
			if len(args) == 2 {
				// AUTH password (Redis < 6.0 format)
				providedPassword = args[1]
				providedUsername = username
			} else if len(args) == 3 {
				// AUTH username password (Redis 6.0+ format)
				providedUsername = args[1]
				providedPassword = args[2]
			} else {
				sendRedisError(conn, "wrong number of arguments for 'auth' command")
				continue
			}

			// Use constant time comparison to prevent timing attacks
			usernameMatch := subtle.ConstantTimeCompare([]byte(providedUsername), []byte(username)) == 1
			passwordMatch := subtle.ConstantTimeCompare([]byte(providedPassword), []byte(password)) == 1

			if usernameMatch && passwordMatch {
				slog.Info("Redis authentication successful", "client", conn.RemoteAddr())
				sendRedisOK(conn)
				return true, nil
			} else {
				slog.Warn("Redis authentication failed", "client", conn.RemoteAddr())
				sendRedisError(conn, "WRONGPASS invalid username-password pair")
				return false, nil
			}

		case "PING":
			// Allow PING before authentication
			if len(args) == 1 {
				sendRedisResponse(conn, "+PONG\r\n")
			} else {
				// PING with message
				msg := args[1]
				sendRedisResponse(conn, fmt.Sprintf("$%d\r\n%s\r\n", len(msg), msg))
			}

		case "QUIT":
			sendRedisOK(conn)
			return false, nil

		default:
			// Any other command requires authentication first
			sendRedisError(conn, "NOAUTH Authentication required")
			return false, nil
		}
	}
}

func authenticateWithTargetRedis(targetConn net.Conn, username, password string) error {
	if username == "" && password == "" {
		return nil
	}

	reader := bufio.NewReader(targetConn)

	var authCommand string
	if username != "" {
		authCommand = fmt.Sprintf("*3\r\n$4\r\nAUTH\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n",
			len(username), username, len(password), password)
	} else {
		authCommand = fmt.Sprintf("*2\r\n$4\r\nAUTH\r\n$%d\r\n%s\r\n",
			len(password), password)
	}

	_, err := targetConn.Write([]byte(authCommand))
	if err != nil {
		return fmt.Errorf("failed to send AUTH command: %v", err)
	}

	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read AUTH response: %v", err)
	}

	if !strings.HasPrefix(response, "+OK") {
		return fmt.Errorf("target Redis authentication failed: %s", strings.TrimSpace(response))
	}

	return nil
}

func copyData(dst net.Conn, src net.Conn, clientAddr string, direction string, connectionClosed *bool) {
	defer func() {
		dst.Close()
		src.Close()
		if !*connectionClosed {
			*connectionClosed = true
			slog.Info("Connection closed", "client", clientAddr, "direction", direction)
		}
	}()

	_, err := io.Copy(dst, src)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			slog.Warn("Connection timed out", "client", clientAddr, "direction", direction)
		} else {
			slog.Error("Connection error", "client", clientAddr, "direction", direction, "error", err)
		}
	}
}

func copyDataWithShutdown(dst net.Conn, src net.Conn, clientAddr string, direction string, connectionClosed *bool, shutdown chan struct{}) {
	defer func() {
		dst.Close()
		src.Close()
		if !*connectionClosed {
			*connectionClosed = true
			slog.Info("Connection closed", "client", clientAddr, "direction", direction)
		}
	}()

	// Use a channel to signal when copy operation is done
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(dst, src)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				slog.Warn("Connection timed out", "client", clientAddr, "direction", direction)
			} else {
				slog.Error("Connection error", "client", clientAddr, "direction", direction, "error", err)
			}
		}
	case <-shutdown:
		slog.Info("Shutdown signal received, closing connection", "client", clientAddr, "direction", direction)
		return
	}
}

func handleConnection(clientConn net.Conn, config *Config, server *Server) {
	clientAddr := clientConn.RemoteAddr().String()
	connectionClosed := false

	// Track this connection
	server.clientConns.Store(clientConn, true)
	defer func() {
		server.clientConns.Delete(clientConn)
		clientConn.Close()
		if !connectionClosed {
			connectionClosed = true
			slog.Info("Client disconnected", "client", clientAddr)
		}
	}()

	slog.Info("New connection", "client", clientAddr)

	authenticated, bufferedData := authenticateRedisClient(clientConn, config.Username, config.Password)
	if !authenticated {
		slog.Warn("Client authentication failed", "client", clientAddr)
		return
	}

	targetConn, err := net.Dial("tcp", config.TargetAddr)
	if err != nil {
		slog.Error("Error connecting to target", "target", config.TargetAddr, "client", clientAddr, "error", err)
		sendRedisError(clientConn, "Cannot connect to target server")
		return
	}
	defer targetConn.Close()

	if err := authenticateWithTargetRedis(targetConn, config.TargetUsername, config.TargetPassword); err != nil {
		slog.Error("Error authenticating with target", "target", config.TargetAddr, "client", clientAddr, "error", err)
		sendRedisError(clientConn, "Target authentication failed")
		return
	}

	slog.Info("Client connected and authenticated to target", "client", clientAddr, "target", config.TargetAddr)

	// Forward any buffered data to target if needed
	if len(bufferedData) > 0 {
		_, err = targetConn.Write(bufferedData)
		if err != nil {
			slog.Error("Error writing buffered data to target", "client", clientAddr, "error", err)
			return
		}
	}

	// Start bidirectional data forwarding with shutdown signal handling
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		copyDataWithShutdown(targetConn, clientConn, clientAddr, "client->target", &connectionClosed, server.shutdown)
	}()

	go func() {
		defer wg.Done()
		copyDataWithShutdown(clientConn, targetConn, clientAddr, "target->client", &connectionClosed, server.shutdown)
	}()

	// Wait for both directions to complete
	wg.Wait()
}

func main() {
	config := parseArgs()

	slog.Info("Starting TCP Portal",
		"listen", config.ListenAddr,
		"target", config.TargetAddr)

	fmt.Println("\n🔐 Client Authentication")
	fmt.Println(strings.Repeat("-", 25))
	if config.Username != "" || config.Password != "" {
		fmt.Printf("Username: '%s'\n", config.Username)
		fmt.Printf("Password: '%s'\n", config.Password)
	} else {
		fmt.Println("No authentication required")
	}

	fmt.Println("\n🎯 Target Authentication")
	fmt.Println(strings.Repeat("-", 25))
	if config.TargetUsername != "" || config.TargetPassword != "" {
		fmt.Printf("Username: '%s'\n", config.TargetUsername)
		fmt.Printf("Password: '%s'\n", config.TargetPassword)
	} else {
		fmt.Println("No authentication required")
	}

	validateListenAddress(config.ListenAddr)
	displayNetworkInfo(config.ListenAddr)

	server := NewServer(config)

	if err := server.Start(); err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	slog.Info("Received shutdown signal")

	server.Shutdown()
}
