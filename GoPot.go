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

func setupSignalHandling() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		consoleLogger.Printf("Received signal: %s", sig)
		fileLogger.Printf("Shutting down due to signal: %s", sig)

		// Chiusura delle connessioni aperte (esempio)
		// closeOpenConnections()

		// Registrazione del messaggio di chiusura
		consoleLogger.Println("Application shutting down.")
		fileLogger.Println("Application shutting down.")

		// Altre operazioni di pulizia se necessario
		// ...

		os.Exit(0)
	}()
}

// handleConnection handles incoming connections and logs the details.
func handleConnection(conn net.Conn, port string) {
	defer func() {
		<-semaphore // Release semaphore
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
		consoleLogger.Println(errMsg)
		fileLogger.Println(errMsg)
		return
	}
}

// listenOnPort listens on a specified port and handles connections.
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

	waitGroup.Wait()
}

func init() {
	maxConnections = 100
	semaphore = make(chan struct{}, maxConnections)
}
