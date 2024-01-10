# Simple Multi-Port Honeypot in Go

This project is a simple, yet effective multi-port honeypot written in Go. It's designed to monitor and analyze potential malicious activities on various TCP ports. By logging detailed information about incoming network connections, it serves as a valuable tool for network security analysis.

## Features

- **Multi-Port Listening**: Capable of listening on multiple ports simultaneously, configurable via command-line flags.
- **Detailed Connection Logging**: Logs detailed information about each connection including the port number, remote address, and the data received.
- **Console Feedback**: Provides real-time feedback in the console for high-level activities like starting port listening and detecting new connections.
- **Port Validation**: Ensures that only valid TCP port numbers are listened on.
- **Dual Logging System**: Uses separate loggers for writing detailed logs to files and high-level information to the console.

## Getting Started

### Prerequisites

- Go (version 1.15 or later)

### Installing

1. Clone the repository:
   ```git clone https://github.com/jackyes/GoPot.git```
2. Navigate to the cloned directory:
   ```cd GoPot/```
   
### Usage

Run the program with the following command, specifying the ports to listen on using the `-ports` flag:

```go run GoPot.go -ports=22,80,8080```


This will start the honeypot and listen on ports 22, 80, and 8080.
Default ports are: ```21,23,110,135,136,137,138,139,445,995,143,993,3306,3389,5900,6379,27017,5060```  
Note: Ports must be not used by other programs  

## Logs

Logs are written to files named in the format `log-YYYY-MM-DD.txt`, making it easy to track and analyze data over specific time periods.

## Contributing

Contributions to this project are welcome! Feel free to fork the repository, make changes, and submit pull requests.

