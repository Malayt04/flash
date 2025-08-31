# Flash: High-Speed File Transfer

Flash is a command-line tool for high-speed, long-distance file transfers. It is built with Go and uses a custom reliable UDP (RUDP) protocol to achieve better performance over networks with high latency and packet loss compared to traditional TCP-based methods.

## Features

- **High-Speed Transfers:** Utilizes UDP for low-overhead data transmission.
- **Reliability:** Implements a custom RUDP protocol with NACK (Negative Acknowledgement)-based retransmissions to ensure data integrity.
- **Congestion Control:** (Future implementation) Will include congestion control to adapt to network conditions.
- **Cross-Platform:** Written in Go, it can be compiled for various operating systems.
- **Easy to Use:** Simple command-line interface for sending and receiving files.

## How It Works

Flash works by breaking a file into small packets and sending them over UDP. The reliability layer is handled by a custom protocol:

1.  **Handshake:** The client initiates the transfer by sending a `FileInfoPacket` to the server. This packet contains the filename, size, and a SHA256 hash of the file.
2.  **Data Transfer:** The client streams the file data in `DataPacket`s. Each packet has a sequence number.
3.  **Reliability (NACKs):** The server keeps track of the received packets. If it detects missing packets (based on sequence numbers), it sends a `NackPacket` to the client, requesting retransmission of the specific missing packets. This is more efficient than the ACK-based approach of TCP in high-latency environments.
4.  **Completion:** Once the client has sent all data packets, it sends a `CompletePacket`. The server then reassembles the file in the correct order.
5.  **Verification:** The server calculates the SHA256 hash of the received file and compares it with the hash received in the initial `FileInfoPacket`. If the hashes match, the transfer is successful. The server sends a final `CompletePacket` to the client to confirm.

## Getting Started

### Prerequisites

- [Go](https://golang.org/doc/install) (version 1.15 or later)

### Installation

1.  Clone the repository:
    ```bash
    git clone https://github.com/your-username/flash.git
    cd flash
    ```

2.  Build the application:
    ```bash
    go build
    ```

### Usage

Flash has two main commands: `send` and `receive`.

#### Receiving a File

To start a server and wait for a file, use the `receive` command.

```bash
./flash receive
```

By default, the server listens on port `8080`. You can specify a different port with the `-p` or `--port` flag:

```bash
./flash receive -p 9000
```

The received file will be saved in the current directory.

#### Sending a File

To send a file, use the `send` command, specifying the path to the file and the server's address (including the port).

```bash
./flash send [file_path] [server_address:port]
```

**Example:**

```bash
./flash send my_large_file.zip 192.168.1.100:8080
```

## Protocol Details

The custom RUDP protocol uses several packet types:

-   `FileInfoPacket`: Carries metadata about the file (name, size, hash).
-   `DataPacket`: Contains a chunk of the file data and a sequence number.
-   `NackPacket`: Sent by the server to request retransmission of missing packets.
-   `CompletePacket`: Signals the end of the transfer from the client side and serves as a final acknowledgment from the server.

All packets include a checksum to protect against data corruption during transit.

## Future Development

-   **Congestion Control:** Implement a congestion control mechanism to avoid overwhelming the network.
-   **Encryption:** Add support for encrypting the data in transit.
--   **Directory Transfers:** Add the ability to transfer entire directories.
-   **Web Interface:** A simple web interface for easier file transfers.
