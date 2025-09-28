package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Global variables for managing logging, connections, and synchronization
var (
	maxConnections    int                   // Maximum number of concurrent connections
	semaphore         chan struct{}         // Semaphore for limiting concurrent connections
	activeConnections map[net.Conn]struct{} // Map to track active connections
	connMutex         sync.Mutex            // Mutex for synchronizing access to the activeConnections map
	fakeBanners       map[string]string     // Map of port-specific banners
)

// setupLoggers configures structured JSON logging with log rotation
func setupLoggers() {
	// Configure lumberjack for log rotation
	logFile := &lumberjack.Logger{
		Filename:   "gopot.log",
		MaxSize:    10,   // megabytes
		MaxBackups: 3,    // number of old log files to keep
		MaxAge:     28,   // days
		Compress:   true, // compress old log files
	}

	// Create a multi-writer: console (pretty) and file (JSON)
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	multi := zerolog.MultiLevelWriter(consoleWriter, logFile)

	// Configure global logger
	log.Logger = zerolog.New(multi).With().Timestamp().Logger()
}

// setupSignalHandling creates a context that is cancelled on SIGINT or SIGTERM
func setupSignalHandling() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Info().
			Str("event", "signal_received").
			Str("signal", sig.String()).
			Msg("Received shutdown signal")

		cancel() // Cancel the context to trigger graceful shutdown
	}()

	return ctx
}

// handleConnection handles incoming connections and logs the details.
// It also manages connection timeouts and closes the connection after handling.
func handleConnection(conn net.Conn, port string) {
	// Set a 10-second timeout for any read/write operations
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	defer func() {
		connMutex.Lock()
		delete(activeConnections, conn)
		connMutex.Unlock()
		<-semaphore  // Release semaphore
		conn.Close() // Close the connection
	}()

	clientAddr := conn.RemoteAddr().String()

	// Log connection details using structured logging
	log.Info().
		Str("event", "connection_received").
		Str("remote_ip", clientAddr).
		Str("port", port).
		Msg("New connection received")

	// Send a port-specific banner if one exists
	if banner, ok := fakeBanners[port]; ok {
		_, err := conn.Write([]byte(banner))
		if err != nil {
			log.Error().
				Str("event", "write_error").
				Str("remote_ip", clientAddr).
				Str("port", port).
				Err(err).
				Msg("Error writing to connection")
			return
		}
	} else {
		// Default response for unmapped ports
		_, err := conn.Write([]byte("Authentication failed.\n"))
		if err != nil {
			log.Error().
				Str("event", "write_error").
				Str("remote_ip", clientAddr).
				Str("port", port).
				Err(err).
				Msg("Error writing to connection")
			return
		}
	}

	// Read and log client data
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		log.Error().
			Str("event", "read_error").
			Str("remote_ip", clientAddr).
			Str("port", port).
			Err(err).
			Msg("Error reading from connection")
		return
	}

	data := string(buffer[:n])
	log.Info().
		Str("event", "data_received").
		Str("remote_ip", clientAddr).
		Str("port", port).
		Str("data", data).
		Msg("Data received from client")
}

// listenOnPort listens on a specified port and handles incoming connections.
// It acquires a semaphore before accepting a connection to limit concurrency.
func listenOnPort(ctx context.Context, port string, wg *sync.WaitGroup) {
	defer wg.Done()

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Error().
			Str("event", "listen_error").
			Str("port", port).
			Err(err).
			Msg("Error listening on port")
		return
	}
	log.Info().
		Str("event", "listener_started").
		Str("port", port).
		Msg("Listening on port")

	// Start a goroutine to close the listener when context is cancelled
	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		semaphore <- struct{}{} // Acquire semaphore
		connection, err := listener.Accept()
		if err != nil {
			<-semaphore // Release semaphore on error

			// Check if the error is due to context cancellation
			select {
			case <-ctx.Done():
				log.Info().
					Str("event", "listener_stopped").
					Str("port", port).
					Msg("Stopping listener due to context cancellation")
				return
			default:
				// Other error, log and continue
				log.Error().
					Str("event", "accept_error").
					Str("port", port).
					Err(err).
					Msg("Error accepting connection")
				continue
			}
		}

		connMutex.Lock()
		activeConnections[connection] = struct{}{}
		connMutex.Unlock()

		go handleConnection(connection, port)
	}
}

// isValidPort checks if the provided port string is a valid TCP port.
func isValidPort(port string) bool {
	p, err := strconv.Atoi(port)
	return err == nil && p > 0 && p <= 65535
}

func main() {
	// Initialize Viper configuration
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal().Err(err).Msg("Error reading config file")
	}

	// Load configuration values
	ports := viper.GetStringSlice("ports")
	maxConnections = viper.GetInt("max_connections")
	fakeBanners = viper.GetStringMapString("banners")

	// Validate ports
	var validPorts []string
	for _, port := range ports {
		portStr := fmt.Sprintf("%v", port)
		if isValidPort(portStr) {
			validPorts = append(validPorts, portStr)
		} else {
			log.Warn().Str("port", portStr).Msg("Invalid port number")
		}
	}

	if len(validPorts) == 0 {
		log.Error().Msg("No valid ports provided. Exiting.")
		os.Exit(1)
	}

	setupLoggers()
	ctx := setupSignalHandling()

	semaphore = make(chan struct{}, maxConnections)
	activeConnections = make(map[net.Conn]struct{})

	var wg sync.WaitGroup
	for _, port := range validPorts {
		wg.Add(1)
		go listenOnPort(ctx, port, &wg)
	}

	// Wait for context cancellation or all listeners to finish
	<-ctx.Done()

	// Wait for all listeners to shut down gracefully
	wg.Wait()

	// Close any remaining connections
	closeOpenConnections()

	log.Info().Msg("Application shutdown complete")
}

// closeOpenConnections closes all active connections.
// It is called during graceful shutdown to ensure all resources are released.
func closeOpenConnections() {
	connMutex.Lock()
	defer connMutex.Unlock()

	for conn := range activeConnections {
		_ = conn.Close() // Close the connection and ignore the error if any
		delete(activeConnections, conn)
	}
	log.Info().
		Str("event", "connections_closed").
		Msg("All active connections closed")
}
