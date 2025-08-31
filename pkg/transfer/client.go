package transfer

import (
	"crypto/sha256"
	"fmt"

	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Malayt04/flash/pkg/packet"
	"github.com/schollz/progressbar/v3"
)

type Client struct {
    conn       *net.UDPConn
    serverAddr *net.UDPAddr
    
    // Transfer state
    file           *os.File
    fileSize       int64
    fileName       string
    fileHash       []byte
    totalPackets   uint32
    sentPackets    map[uint32]bool
    retransmitChan chan uint32
    
    // Statistics
    bytesSent    int64
    packetsRetransmitted int
    
    // Control
    done chan struct{}
    wg   sync.WaitGroup
    mu   sync.RWMutex
}

func NewClient(serverAddr string) (*Client, error) {
    addr, err := net.ResolveUDPAddr("udp", serverAddr)
    if err != nil {
        return nil, err
    }
    
    conn, err := net.DialUDP("udp", nil, addr)
    if err != nil {
        return nil, err
    }
    
    return &Client{
        conn:           conn,
        serverAddr:     addr,
        sentPackets:    make(map[uint32]bool),
        retransmitChan: make(chan uint32, 1000),
        done:           make(chan struct{}),
    }, nil
}

func (c *Client) SendFile(filePath string) error {
    file, err := os.Open(filePath)
    if err != nil {
        return fmt.Errorf("failed to open file: %v", err)
    }
    defer file.Close()
    
    fileInfo, err := file.Stat()
    if err != nil {
        return fmt.Errorf("failed to get file info: %v", err)
    }
    
    c.file = file
    c.fileSize = fileInfo.Size()
    c.fileName = filepath.Base(filePath)
    c.totalPackets = uint32((c.fileSize + packet.MaxDataSize - 1) / packet.MaxDataSize)
    
    // Calculate file hash
    file.Seek(0, 0)
    hash := sha256.New()
    if _, err := io.Copy(hash, file); err != nil {
        return fmt.Errorf("failed to calculate file hash: %v", err)
    }
    c.fileHash = hash.Sum(nil)
    file.Seek(0, 0)
    
    fmt.Printf("Sending file: %s (%d bytes, %d packets)\n", c.fileName, c.fileSize, c.totalPackets)
    
    // Send file info
    if err := c.sendFileInfo(); err != nil {
        return fmt.Errorf("failed to send file info: %v", err)
    }
    
    // Start NACK listener
    c.wg.Add(1)
    go c.nackListener()
    
    // Start retransmission handler
    c.wg.Add(1)
    go c.retransmissionHandler()
    
    // Send file data
    if err := c.sendFileData(); err != nil {
        return fmt.Errorf("failed to send file data: %v", err)
    }
    
    // Wait for completion or timeout
    select {
    case <-c.waitForCompletion():
        fmt.Println("\nFile transfer completed successfully!")
    case <-time.After(5 * time.Minute):
        return fmt.Errorf("transfer timeout")
    }
    
    close(c.done)
    c.wg.Wait()
    
    fmt.Printf("Transfer Statistics:\n")
    fmt.Printf("  Bytes sent: %d\n", c.bytesSent)
    fmt.Printf("  Packets retransmitted: %d\n", c.packetsRetransmitted)
    
    return nil
}

func (c *Client) sendFileInfo() error {
    infoPacket := packet.NewFileInfoPacket(c.fileName, c.fileSize, c.fileHash)
    data := infoPacket.Serialize()
    
    _, err := c.conn.Write(data)
    return err
}

func (c *Client) sendFileData() error {
    buffer := make([]byte, packet.MaxDataSize)
    bar := progressbar.DefaultBytes(c.fileSize, "Sending")
    
    var seqNum uint32 = 1
    for {
        n, err := c.file.Read(buffer)
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
        
        dataPacket := packet.NewDataPacket(seqNum, buffer[:n])
        data := dataPacket.Serialize()
        
        if _, err := c.conn.Write(data); err != nil {
            return err
        }
        
        c.mu.Lock()
        c.sentPackets[seqNum] = true
        c.bytesSent += int64(n)
        c.mu.Unlock()
        
        bar.Add(n)
        seqNum++
        
        // Small delay to prevent overwhelming the network
        time.Sleep(10 * time.Microsecond)
    }
    
    bar.Finish()
    
    // Send completion packet
    completePacket := packet.NewCompletePacket()
    data := completePacket.Serialize()
    c.conn.Write(data)
    
    return nil
}

func (c *Client) nackListener() {
    defer c.wg.Done()
    
    buffer := make([]byte, 2048)
    for {
        select {
        case <-c.done:
            return
        default:
        }
        
        c.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
        n, err := c.conn.Read(buffer)
        if err != nil {
            if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                continue
            }
            return
        }
        
        pkt, err := packet.Deserialize(buffer[:n])
        if err != nil {
            continue
        }
        
        if !pkt.Verify() {
            continue
        }
        
        if pkt.Type == packet.NackPacket {
            seqNums := pkt.ExtractNackSeqNums()
            for _, seq := range seqNums {
                select {
                case c.retransmitChan <- seq:
                case <-c.done:
                    return
                }
            }
        }
    }
}

func (c *Client) retransmissionHandler() {
    defer c.wg.Done()
    
    buffer := make([]byte, packet.MaxDataSize)
    
    for {
        select {
        case seqNum := <-c.retransmitChan:
            // Calculate file position for this sequence number
            offset := int64(seqNum-1) * packet.MaxDataSize
            
            c.file.Seek(offset, 0)
            n, err := c.file.Read(buffer)
            if err != nil && err != io.EOF {
                continue
            }
            
            retransmitPacket := packet.NewDataPacket(seqNum, buffer[:n])
            data := retransmitPacket.Serialize()
            
            c.conn.Write(data)
            c.packetsRetransmitted++
            
        case <-c.done:
            return
        }
    }
}

func (c *Client) waitForCompletion() <-chan struct{} {
    completeChan := make(chan struct{})
    
    go func() {
        buffer := make([]byte, 1024)
        for {
            c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
            n, err := c.conn.Read(buffer)
            if err != nil {
                continue
            }
            
            pkt, err := packet.Deserialize(buffer[:n])
            if err != nil {
                continue
            }
            
            if pkt.Type == packet.CompletePacket && pkt.Verify() {
                close(completeChan)
                return
            }
        }
    }()
    
    return completeChan
}

func (c *Client) Close() error {
    return c.conn.Close()
}