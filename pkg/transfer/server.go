package transfer

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/Malayt04/flash/pkg/packet"
	"github.com/schollz/progressbar/v3"
)

type Server struct {
    conn     *net.UDPConn
    
    // Transfer state
    receivedPackets map[uint32][]byte
    expectedSeqNum  uint32
    maxSeqNum       uint32
    fileName        string
    fileSize        int64
    fileHash        []byte
    outputFile      *os.File
    clientAddr      *net.UDPAddr
    
    // Statistics
    bytesReceived int64
    packetsReceived int64
    nacksSent      int
    
    // Control
    done chan struct{}
    mu   sync.RWMutex
}

func NewServer(listenAddr string) (*Server, error) {
    addr, err := net.ResolveUDPAddr("udp", listenAddr)
    if err != nil {
        return nil, err
    }
    
    conn, err := net.ListenUDP("udp", addr)
    if err != nil {
        return nil, err
    }
    
    return &Server{
        conn:            conn,
        receivedPackets: make(map[uint32][]byte),
        done:            make(chan struct{}),
    }, nil
}

func (s *Server) Listen() error {
    fmt.Printf("Server listening on %s\\n", s.conn.LocalAddr())
    
    buffer := make([]byte, 2048)
    var bar *progressbar.ProgressBar
    
    // Start NACK sender
    go s.nackSender()
    
    for {
        n, clientAddr, err := s.conn.ReadFromUDP(buffer)
        if err != nil {
            return err
        }
        
        pkt, err := packet.Deserialize(buffer[:n])
        if err != nil {
            continue
        }
        
        if !pkt.Verify() {
            continue
        }
        
        s.clientAddr = clientAddr
        
        switch pkt.Type {
        case packet.FileInfoPacket:
            fileName, fileSize, fileHash, err := pkt.ExtractFileInfo()
            if err != nil {
                continue
            }
            
            s.fileName = fileName
            s.fileSize = fileSize
            s.fileHash = fileHash
            s.maxSeqNum = uint32((fileSize + packet.MaxDataSize - 1) / packet.MaxDataSize)
            s.expectedSeqNum = 1
            
            // Create output file
            s.outputFile, err = os.Create(s.fileName)
            if err != nil {
                return fmt.Errorf("failed to create output file: %v", err)
            }
            
            fmt.Printf("Receiving file: %s (%d bytes, %d packets)\n", s.fileName, s.fileSize, s.maxSeqNum)
            bar = progressbar.DefaultBytes(s.fileSize, "Receiving")
            
        case packet.DataPacket:
            s.mu.Lock()
            s.receivedPackets[pkt.SeqNum] = pkt.Data
            s.packetsReceived++
            s.bytesReceived += int64(len(pkt.Data))
            s.mu.Unlock()
            
            if bar != nil {
                bar.Add(len(pkt.Data))
            }
            
        case packet.CompletePacket:
            if bar != nil {
                bar.Finish()
            }
            return s.finalizeTransfer()
        }
    }
}

func (s *Server) nackSender() {
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if s.maxSeqNum == 0 {
                continue
            }
            
            s.mu.RLock()
            var missingSeqNums []uint32
            
            for seq := s.expectedSeqNum; seq <= s.maxSeqNum; seq++ {
                if _, exists := s.receivedPackets[seq]; !exists {
                    missingSeqNums = append(missingSeqNums, seq)
                    if len(missingSeqNums) >= 50 { // Limit NACK size
                        break
                    }
                }
            }
            s.mu.RUnlock()
            
            if len(missingSeqNums) > 0 && s.clientAddr != nil {
                nackPacket := packet.NewNackPacket(missingSeqNums)
                data := nackPacket.Serialize()
                s.conn.WriteToUDP(data, s.clientAddr)
                s.nacksSent++
            }
            
        case <-s.done:
            return
        }
    }
}

func (s *Server) finalizeTransfer() error {
    defer s.outputFile.Close()
    close(s.done)
    
    fmt.Printf("\\nReassembling file...\\n")
    
    // Write packets in order
    for seq := uint32(1); seq <= s.maxSeqNum; seq++ {
        s.mu.RLock()
        data, exists := s.receivedPackets[seq]
        s.mu.RUnlock()
        
        if !exists {
            return fmt.Errorf("missing packet %d", seq)
        }
        
        if _, err := s.outputFile.Write(data); err != nil {
            return fmt.Errorf("failed to write packet %d: %v", seq, err)
        }
    }
    
    // Verify file integrity
    s.outputFile.Seek(0, 0)
    hash := sha256.New()
    if _, err := io.Copy(hash, s.outputFile); err != nil {
        return fmt.Errorf("failed to calculate received file hash: %v", err)
    }
    
    receivedHash := hash.Sum(nil)
    
    // Compare hashes
    if !compareHashes(s.fileHash, receivedHash) {
        return fmt.Errorf("file integrity check failed")
    }
    
    // Send completion acknowledgment
    completePacket := packet.NewCompletePacket()
    data := completePacket.Serialize()
    s.conn.WriteToUDP(data, s.clientAddr)
    
    fmt.Printf("File transfer completed successfully!\\n")
    fmt.Printf("  Bytes received: %d\\n", s.bytesReceived)
    fmt.Printf("Transfer Statistics:\\n")
    fmt.Printf("  Packets received: %d\\n", s.packetsReceived)
    fmt.Printf("  NACKs sent: %d\\n", s.nacksSent)
    
    return nil
}

func compareHashes(hash1, hash2 []byte) bool {
    if len(hash1) != len(hash2) {
        return false
    }
    for i := range hash1 {
        if hash1[i] != hash2[i] {
            return false
        }
    }
    return true
}

func (s *Server) Close() error {
    close(s.done)
    return s.conn.Close()
}