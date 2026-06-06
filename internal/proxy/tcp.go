package proxy

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/teleproxy/internal/tunnel"
)

type Handler struct {
	tracker  *tunnel.ConnTracker
	mu       sync.Mutex
	sendFunc func([]byte) error
}

func NewHandler(sendFunc func([]byte) error) *Handler {
	return &Handler{
		tracker:  tunnel.NewConnTracker(),
		sendFunc: sendFunc,
	}
}

func (h *Handler) HandlePacket(data []byte) {
	pkt, err := tunnel.UnmarshalPacket(data)
	if err != nil {
		return
	}

	switch pkt.Type {
	case tunnel.PacketTypeTCPConnect:
		h.handleTCPConnect(pkt)
	case tunnel.PacketTypeTCPData:
		h.handleTCPData(pkt)
	case tunnel.PacketTypeTCPClose:
		h.handleTCPClose(pkt)
	case tunnel.PacketTypeUDPData:
		h.handleUDPData(pkt)
	}
}

func (h *Handler) handleTCPConnect(pkt tunnel.Packet) {
	addr := string(pkt.Payload)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		h.sendPacket(tunnel.Packet{
			Type:    tunnel.PacketTypeError,
			ConnID:  pkt.ConnID,
			Payload: []byte(err.Error()),
		})
		return
	}

	h.tracker.Add(conn)

	go h.readRemote(conn, pkt.ConnID)
}

func (h *Handler) handleTCPData(pkt tunnel.Packet) {
	conn, ok := h.tracker.Get(pkt.ConnID)
	if !ok {
		return
	}
	tcpConn, ok := conn.(net.Conn)
	if !ok {
		return
	}
	if _, err := tcpConn.Write(pkt.Payload); err != nil {
		h.handleTCPClose(pkt)
	}
}

func (h *Handler) handleTCPClose(pkt tunnel.Packet) {
	h.tracker.Remove(pkt.ConnID)
}

func (h *Handler) handleUDPData(pkt tunnel.Packet) {
	addr := string(pkt.Payload)
	parts := strings.SplitN(addr, ":", 2)
	if len(parts) != 2 {
		return
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return
	}

	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return
	}
	defer udpConn.Close()

	if len(pkt.Payload) > 0 {
		// First n bytes are the data after the address
	}

	// Actually, for UDP we need to re-think. Let me re-examine the format
}

func (h *Handler) readRemote(conn net.Conn, connID uint32) {
	buf := make([]byte, 65535)
	defer h.tracker.Remove(connID)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			h.sendPacket(tunnel.Packet{
				Type:   tunnel.PacketTypeTCPClose,
				ConnID: connID,
			})
			return
		}
		if n == 0 {
			continue
		}

		data := make([]byte, n)
		copy(data, buf[:n])
		h.sendPacket(tunnel.Packet{
			Type:    tunnel.PacketTypeTCPData,
			ConnID:  connID,
			Payload: data,
		})
	}
}

func (h *Handler) sendPacket(pkt tunnel.Packet) error {
	data := tunnel.MarshalPacket(pkt)
	return h.sendFunc(data)
}

func dialTCP(addr string) (net.Conn, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("split host port: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("parse port: %w", err)
	}
	return net.DialTCP("tcp", nil, &net.TCPAddr{
		IP:   net.ParseIP(host),
		Port: port,
	})
}

func dialUDP(addr string) (*net.UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	return net.DialUDP("udp", nil, udpAddr)
}

var _ = io.Discard
var _ = dialUDP
var _ = dialTCP
