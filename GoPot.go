package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Global variables for managing logging, connections, and synchronization
var (
	fileLogger        *log.Logger // Logger for writing to the log file
	consoleLogger     *log.Logger // Logger for writing to the console
	maxConnections    int         // Maximum number of concurrent connections
	semaphore         chan struct{} // Semaphore for limiting concurrent connections
	activeConnections map[net.Conn]struct{} // Map to track active connections
	connMutex         sync.Mutex // Mutex for synchronizing access to the activeConnections map
	logFile           *os.File // Current log file for writing logs
	lastLogDate       string   // Date of the last log entry, used for rotating log files
)

// setupLoggers configures and manages log files for daily logging.
// It creates a new log file for each day and archives the previous day's log.
func setupLoggers() {
	currentTime := time.Now()
	currentDate := currentTime.Format("2006-01-02") // Current date formatted as YYYY-MM-DD

	// Check if the date has changed. If so, close the current log file and archive it.
	if lastLogDate != "" && lastLogDate != currentDate {
		if logFile != nil {
			logFile.Close() // Close the existing log file
			// Rename the current log file to include the date for archiving
			oldLogFileName := fmt.Sprintf("log-%s.txt", lastLogDate)
			os.Rename("log.txt", oldLogFileName) // Archive the log file by renaming it
		}
	}

	// Set up a new log file for the current day
	if lastLogDate != currentDate {
		var err error
		logFile, err = os.OpenFile("log.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Unable to open log file: %v", err) // Fatal error if the log file cannot be opened
		}

		fileLogger = log.New(logFile, "", log.LstdFlags) // Initialize the file logger
		lastLogDate = currentDate                        // Update the last log date to the current date
	}

	// Setup the console logger to output logs to the standard output
	consoleLogger = log.New(os.Stdout, "", log.LstdFlags) // Initialize the console logger
}

// setupSignalHandling configures handling for SIGINT and SIGTERM signals.
// It gracefully shuts down the application by closing open connections and the log file.
func setupSignalHandling() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		consoleLogger.Printf("Received signal: %s", sig)
		fileLogger.Printf("Shutting down due to signal: %s", sig)

		closeOpenConnections() // Close all open connections

		consoleLogger.Println("Application shutting down.")
		fileLogger.Println("Application shutting down.")

		if logFile != nil {
			logFile.Close() // Close the log file
		}

		os.Exit(0) // Exit the application
	}()
}

// handleConnection handles incoming connections and logs the details.
// It also manages connection timeouts and closes the connection after handling.
func handleConnection(conn net.Conn, port string) {
	setupLoggers() // Ensure loggers are up to date
	connMutex.Lock()
	activeConnections[conn] = struct{}{}
	connMutex.Unlock()
	defer func() {
		connMutex.Lock()
		delete(activeConnections, conn)
		connMutex.Unlock()
		<-semaphore // Release semaphore
		conn.Close() // Close the connection
	}()

	timeoutDuration := 15 * time.Second
	conn.SetDeadline(time.Now().Add(timeoutDuration)) // Set a timeout for the connection

	clientAddr := conn.RemoteAddr().String()
	// Log connection details to console and file
	msg := fmt.Sprintf("Received connection on port %s from %s", port, clientAddr)
	consoleLogger.Println(msg)
	fileLogger.Println(msg)

	_, err := conn.Write([]byte("Authentication failed."))
	if err != nil {
		errMsg := fmt.Sprintf("Error writing to connection on port %s: %s", port, err)
		consoleLogger.Println(errMsg)
		fileLogger.Println(errMsg)
		return
	}
}

// listenOnPort listens on a specified port and handles incoming connections.
// It acquires a semaphore before accepting a connection to limit concurrency.
func listenOnPort(port string, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		errMsg := fmt.Sprintf("Error listening on port %s: %s", port, err)
		consoleLogger.Println(errMsg)
		fileLogger.Fatalf(errMsg)
	}
	consoleLogger.Printf("Listening on port %s", port)

	for {
		semaphore <- struct{}{} // Acquire semaphore
		connection, err := listener.Accept()
		if err != nil {
			<-semaphore // Release semaphore on error
			errMsg := fmt.Sprintf("Error accepting connection on port %s: %s", port, err)
			consoleLogger.Println(errMsg)
			fileLogger.Println(errMsg)
			continue
		}
		go handleConnection(connection, port)
	}
}

// isValidPort checks if the provided port string is a valid TCP port.
func isValidPort(port string) bool {
	p, err := strconv.Atoi(port)
	if err != nil {
		return false
	}
	return p > 0 && p <= 65535
}

func main() {
	setupLoggers()
	setupSignalHandling()

	var portsFlag string
	flag.StringVar(&portsFlag, "ports", "21,23,110,135,136,137,138,139,445,995,143,993,3306,3389,5900,6379,27017,5060", "comma-separated list of ports to listen on")
	flag.Parse()

	ports := strings.Split(portsFlag, ",")
	var validPorts []string
	for _, port := range ports {
		if isValidPort(port) {
			validPorts = append(validPorts, port)
		} else {
			consoleLogger.Printf("Invalid port number: %s", port)
		}
	}

	if len(validPorts) == 0 {
		consoleLogger.Println("No valid ports provided. Exiting.")
		os.Exit(1)
	}

	var waitGroup sync.WaitGroup
	for _, port := range validPorts {
		waitGroup.Add(1)
		go listenOnPort(port, &waitGroup)
	}

	waitGroup.Wait() // Wait for all port listeners to finish
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
	consoleLogger.Println("All active connections closed.")
	fileLogger.Println("All active connections closed.")
}

func init() {
	lastLogDate = "" // Set the last log date to an empty string
	setupLoggers()   // Call setupLoggers at the beginning to initialize loggers
	maxConnections = 100
	semaphore = make(chan struct{}, maxConnections)
	activeConnections = make(map[net.Conn]struct{})
}
