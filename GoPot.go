package main

import (
	"bytes"
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
	fileLogger    *log.Logger // Logger for writing to the log file
	consoleLogger *log.Logger // Logger for writing to the console
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
	defer conn.Close()

	// Log connection details to console and file
	consoleLogger.Printf("Received connection on port %s from %s", port, conn.RemoteAddr())
	fileLogger.Printf("Received connection on port %s from %s", port, conn.RemoteAddr())

	// Read from the connection
	buffer := make([]byte, 1024)
	numberOfBytes, err := conn.Read(buffer)
	if err != nil {
		fileLogger.Printf("Error reading from connection on port %s: %s", port, err)
		return
	}

	// Send a predefined response and log the interaction
	_, err = conn.Write([]byte("Authentication failed."))
	if err != nil {
		fileLogger.Printf("Error writing to connection on port %s: %s", port, err)
		return
	}

	// Log the data received from the connection
	trimmedOutput := bytes.TrimRight(buffer, "\x00")
	fileLogger.Printf("Read %d bytes from %s on port %s\n%s", numberOfBytes, conn.RemoteAddr(), port, trimmedOutput)
}

// listenOnPort listens on a specified port and handles connections.
func listenOnPort(port string, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	// Start listening on the specified port
	listener, err := net.Listen("tcp", "localhost:"+port)
	if err != nil {
		fileLogger.Fatalf("Error listening on port %s: %s", port, err)
	}
	consoleLogger.Printf("Listening on port %s", port)

	// Accept and handle incoming connections
	for {
		connection, err := listener.Accept()
		if err != nil {
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
	flag.StringVar(&portsFlag, "ports", "22,80,8080", "comma-separated list of ports to listen on")
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
