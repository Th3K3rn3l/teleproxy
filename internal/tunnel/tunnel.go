package tunnel

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

type PacketType byte

const (
	PacketTypeTCPConnect    PacketType = 0x01
	PacketTypeTCPData       PacketType = 0x02
	PacketTypeTCPClose      PacketType = 0x03
	PacketTypeUDPAssociate  PacketType = 0x04
	PacketTypeUDPData       PacketType = 0x05
	PacketTypeUDPClose      PacketType = 0x06
	PacketTypeError         PacketType = 0xFF
)

type Packet struct {
	Type    PacketType
	ConnID  uint32
	Payload []byte
}

func MarshalPacket(p Packet) []byte {
	headerLen := 1 + 4
	buf := make([]byte, headerLen+len(p.Payload))
	buf[0] = byte(p.Type)
	binary.BigEndian.PutUint32(buf[1:5], p.ConnID)
	copy(buf[5:], p.Payload)
	return buf
}

func UnmarshalPacket(data []byte) (Packet, error) {
	if len(data) < 5 {
		return Packet{}, fmt.Errorf("packet too short: %d", len(data))
	}
	return Packet{
		Type:    PacketType(data[0]),
		ConnID:  binary.BigEndian.Uint32(data[1:5]),
		Payload: data[5:],
	}, nil
}

type ConnTracker struct {
	mu        sync.Mutex
	conns     map[uint32]io.ReadWriteCloser
	nextID    uint32
}

func NewConnTracker() *ConnTracker {
	return &ConnTracker{
		conns:  make(map[uint32]io.ReadWriteCloser),
		nextID: 1,
	}
}

func (ct *ConnTracker) Add(conn io.ReadWriteCloser) uint32 {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	id := ct.nextID
	ct.nextID++
	ct.conns[id] = conn
	return id
}

func (ct *ConnTracker) Get(id uint32) (io.ReadWriteCloser, bool) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	conn, ok := ct.conns[id]
	return conn, ok
}

func (ct *ConnTracker) Remove(id uint32) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	if conn, ok := ct.conns[id]; ok {
		conn.Close()
		delete(ct.conns, id)
	}
}

func (ct *ConnTracker) Len() int {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return len(ct.conns)
}
