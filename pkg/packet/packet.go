package packet

import (
    "bytes"
    "crypto/sha256"
    "encoding/binary"
    "fmt"
    "hash/crc32"
)

type PacketType uint8

const (
    DataPacket PacketType = iota
    AckPacket
    NackPacket
    FileInfoPacket
    CompletePacket
)

const MaxDataSize = 1400 // Safe UDP payload size

type Packet struct {
    Type     PacketType
    SeqNum   uint32
    Checksum uint32
    DataSize uint16
    Data     []byte
}

func NewDataPacket(seqNum uint32, data []byte) *Packet {
    if len(data) > MaxDataSize {
        data = data[:MaxDataSize]
    }
    
    packet := &Packet{
        Type:     DataPacket,
        SeqNum:   seqNum,
        DataSize: uint16(len(data)),
        Data:     make([]byte, len(data)),
    }
    copy(packet.Data, data)
    packet.Checksum = packet.calculateChecksum()
    return packet
}

func NewNackPacket(seqNums []uint32) *Packet {
    buf := new(bytes.Buffer)
    for _, seq := range seqNums {
        binary.Write(buf, binary.BigEndian, seq)
    }
    
    packet := &Packet{
        Type:     NackPacket,
        SeqNum:   0,
        DataSize: uint16(buf.Len()),
        Data:     buf.Bytes(),
    }
    packet.Checksum = packet.calculateChecksum()
    return packet
}

func NewFileInfoPacket(filename string, fileSize int64, fileHash []byte) *Packet {
    buf := new(bytes.Buffer)
    
    // Write filename length and filename
    binary.Write(buf, binary.BigEndian, uint16(len(filename)))
    buf.WriteString(filename)
    
    // Write file size
    binary.Write(buf, binary.BigEndian, fileSize)
    
    // Write file hash
    buf.Write(fileHash)
    
    packet := &Packet{
        Type:     FileInfoPacket,
        SeqNum:   0,
        DataSize: uint16(buf.Len()),
        Data:     buf.Bytes(),
    }
    packet.Checksum = packet.calculateChecksum()
    return packet
}

func NewCompletePacket() *Packet {
    packet := &Packet{
        Type:     CompletePacket,
        SeqNum:   0,
        DataSize: 0,
        Data:     []byte{},
    }
    packet.Checksum = packet.calculateChecksum()
    return packet
}

func (p *Packet) calculateChecksum() uint32 {
    h := crc32.NewIEEE()
    binary.Write(h, binary.BigEndian, p.Type)
    binary.Write(h, binary.BigEndian, p.SeqNum)
    binary.Write(h, binary.BigEndian, p.DataSize)
    h.Write(p.Data)
    return h.Sum32()
}

func (p *Packet) Verify() bool {
    return p.Checksum == p.calculateChecksum()
}

func (p *Packet) Serialize() []byte {
    buf := new(bytes.Buffer)
    binary.Write(buf, binary.BigEndian, p.Type)
    binary.Write(buf, binary.BigEndian, p.SeqNum)
    binary.Write(buf, binary.BigEndian, p.Checksum)
    binary.Write(buf, binary.BigEndian, p.DataSize)
    buf.Write(p.Data)
    return buf.Bytes()
}

func Deserialize(data []byte) (*Packet, error) {
    if len(data) < 11 { // Minimum packet size
        return nil, fmt.Errorf("packet too small")
    }
    
    buf := bytes.NewReader(data)
    packet := &Packet{}
    
    if err := binary.Read(buf, binary.BigEndian, &packet.Type); err != nil {
        return nil, err
    }
    if err := binary.Read(buf, binary.BigEndian, &packet.SeqNum); err != nil {
        return nil, err
    }
    if err := binary.Read(buf, binary.BigEndian, &packet.Checksum); err != nil {
        return nil, err
    }
    if err := binary.Read(buf, binary.BigEndian, &packet.DataSize); err != nil {
        return nil, err
    }
    
    packet.Data = make([]byte, packet.DataSize)
    if _, err := buf.Read(packet.Data); err != nil {
        return nil, err
    }
    
    return packet, nil
}

func (p *Packet) ExtractNackSeqNums() []uint32 {
    if p.Type != NackPacket {
        return nil
    }
    
    buf := bytes.NewReader(p.Data)
    var seqNums []uint32
    
    for buf.Len() >= 4 {
        var seq uint32
        if err := binary.Read(buf, binary.BigEndian, &seq); err != nil {
            break
        }
        seqNums = append(seqNums, seq)
    }
    
    return seqNums
}

func (p *Packet) ExtractFileInfo() (string, int64, []byte, error) {
    if p.Type != FileInfoPacket {
        return "", 0, nil, fmt.Errorf("not a file info packet")
    }
    
    buf := bytes.NewReader(p.Data)
    
    var filenameLen uint16
    if err := binary.Read(buf, binary.BigEndian, &filenameLen); err != nil {
        return "", 0, nil, err
    }
    
    filename := make([]byte, filenameLen)
    if _, err := buf.Read(filename); err != nil {
        return "", 0, nil, err
    }
    
    var fileSize int64
    if err := binary.Read(buf, binary.BigEndian, &fileSize); err != nil {
        return "", 0, nil, err
    }
    
    fileHash := make([]byte, buf.Len())
    buf.Read(fileHash)
    
    return string(filename), fileSize, fileHash, nil
}

func CalculateFileHash(data []byte) []byte {
    hash := sha256.Sum256(data)
    return hash[:]
}