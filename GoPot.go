package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	fileLogger     *log.Logger // Logger for writing to the log file
	consoleLogger  *log.Logger // Logger for writing to the console
	maxConnections int
	semaphore      chan struct{}
)

// setupLoggers initializes both file and console loggers.
func setupLoggers() {
	// Set up file logger
	currentTime := time.Now()
	logFileName := fmt.Sprintf("log-%s.txt", currentTime.Format("2006-01-02"))
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Unable to open log file: %v", err)
	}
	fileLogger = log.New(logFile, "", log.LstdFlags)

	// Set up console logger
	consoleLogger = log.New(os.Stdout, "", log.LstdFlags)
}

// handleConnection handles incoming connections and logs the details.
func handleConnection(conn net.Conn, port string) {
	defer func() {
		<-semaphore // Rilascia il semaforo
		conn.Close()
	}()

	timeoutDuration := 15 * time.Second
	conn.SetDeadline(time.Now().Add(timeoutDuration))

	clientAddr := conn.RemoteAddr().String()

	// Log connection details to console and file
	msg := fmt.Sprintf("Received connection on port %s from %s", port, clientAddr)
	consoleLogger.Println(msg)
	fileLogger.Println(msg)

	_, err := conn.Write([]byte("Authentication failed."))
	if err != nil {
		errMsg := fmt.Sprintf("Error writing to connection on port %s: %s", port, err)
		fileLogger.Println(errMsg)
		return
	}
}

// listenOnPort listens on a specified port and handles connections.
func listenOnPort(port string, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fileLogger.Fatalf("Error listening on port %s: %s", port, err)
	}
	consoleLogger.Printf("Listening on port %s", port)

	for {
		semaphore <- struct{}{} // Acquisisce il semaforo
		connection, err := listener.Accept()
		if err != nil {
			<-semaphore // Rilascia il semaforo in caso di errore
			fileLogger.Printf("Error accepting connection on port %s: %s", port, err)
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
	// Initialize loggers
	setupLoggers()

	// Parse command-line flags for ports
	var portsFlag string
	flag.StringVar(&portsFlag, "ports", "21,23,110,135,136,137,138,139,445,995,143,993,3306,3389,5900,6379,27017,5060", "comma-separated list of ports to listen on")
	flag.Parse()

	// Validate and filter ports
	ports := strings.Split(portsFlag, ",")
	var validPorts []string
	for _, port := range ports {
		if isValidPort(port) {
			validPorts = append(validPorts, port)
		} else {
			consoleLogger.Printf("Invalid port number: %s", port)
		}
	}

	// Check if there are any valid ports to listen on
	if len(validPorts) == 0 {
		consoleLogger.Println("No valid ports provided. Exiting.")
		os.Exit(1)
	}

	// Listen on each valid port
	var waitGroup sync.WaitGroup
	for _, port := range validPorts {
		waitGroup.Add(1)
		go listenOnPort(port, &waitGroup)
	}

	// Wait for all listening routines to complete
	waitGroup.Wait()
}

func init() {
	maxConnections = 100 // Set Max concurrent connections
	semaphore = make(chan struct{}, maxConnections)
}
