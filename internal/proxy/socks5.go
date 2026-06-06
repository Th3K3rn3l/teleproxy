package proxy

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/teleproxy/internal/tunnel"
)

const (
	socksVer5        = 5
	cmdConnect       = 1
	cmdUDPAssociate  = 3
	atypIPv4         = 1
	atypDomainName   = 3
	atypIPv6         = 4

	repSuccess = 0
)

type TunnelWriter interface {
	Send(data []byte) error
}

type SOCKS5Server struct {
	listenAddr string
	tunnel     TunnelWriter
	tracker    *tunnel.ConnTracker
	udpConns   *tunnel.ConnTracker
}

func NewSOCKS5Server(addr string, t TunnelWriter, tracker *tunnel.ConnTracker) *SOCKS5Server {
	return &SOCKS5Server{
		listenAddr: addr,
		tunnel:     t,
		tracker:    tracker,
		udpConns:   tunnel.NewConnTracker(),
	}
}

func (s *SOCKS5Server) Start() error {
	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			go s.handleConn(conn)
		}
	}()

	return nil
}

func (s *SOCKS5Server) handleConn(client net.Conn) {
	defer client.Close()

	buf := make([]byte, 256)

	if _, err := io.ReadFull(client, buf[:2]); err != nil {
		return
	}
	nMethods := int(buf[1])
	if _, err := io.ReadFull(client, buf[:nMethods]); err != nil {
		return
	}

	client.Write([]byte{socksVer5, 0})

	if _, err := io.ReadFull(client, buf[:4]); err != nil {
		return
	}

	cmd := buf[1]
	addr, port, err := parseAddr(buf, client)
	if err != nil {
		client.Write([]byte{socksVer5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}

	switch cmd {
	case cmdConnect:
		s.handleTCP(client, addr, port)
	case cmdUDPAssociate:
		s.handleUDP(client, addr, port)
	default:
		client.Write([]byte{socksVer5, 7, 0, 1, 0, 0, 0, 0, 0, 0})
	}
}

func (s *SOCKS5Server) handleTCP(client net.Conn, addr string, port int) {
	connID := s.tracker.Add(client)

	pkt := tunnel.MarshalPacket(tunnel.Packet{
		Type:   tunnel.PacketTypeTCPConnect,
		ConnID: connID,
		Payload: []byte(fmt.Sprintf("%s:%d", addr, port)),
	})
	if err := s.tunnel.Send(pkt); err != nil {
		s.tracker.Remove(connID)
		return
	}

	reply := []byte{socksVer5, repSuccess, 0, 1, 0, 0, 0, 0, 0, 0}
	client.Write(reply)

	buf := make([]byte, 65535)
	for {
		n, err := client.Read(buf)
		if err != nil {
			break
		}
		dataPkt := tunnel.MarshalPacket(tunnel.Packet{
			Type:    tunnel.PacketTypeTCPData,
			ConnID:  connID,
			Payload: buf[:n],
		})
		if err := s.tunnel.Send(dataPkt); err != nil {
			break
		}
	}

	closePkt := tunnel.MarshalPacket(tunnel.Packet{
		Type:   tunnel.PacketTypeTCPClose,
		ConnID: connID,
	})
	s.tunnel.Send(closePkt)
	s.tracker.Remove(connID)
}

func (s *SOCKS5Server) handleUDP(client net.Conn, addr string, port int) {
	localAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	udpConn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		client.Write([]byte{socksVer5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	defer udpConn.Close()

	bindPort := udpConn.LocalAddr().(*net.UDPAddr).Port
	bindAddr := net.ParseIP("0.0.0.0").To4()
	reply := []byte{socksVer5, repSuccess, 0, 1}
	reply = append(reply, bindAddr...)
	reply = append(reply, byte(bindPort>>8), byte(bindPort))
	client.Write(reply)

	go s.relayUDP(udpConn)

	buf := make([]byte, 1)
	for {
		_, err := client.Read(buf)
		if err != nil {
			break
		}
	}
}

func (s *SOCKS5Server) relayUDP(conn *net.UDPConn) {
	buf := make([]byte, 65535)
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		data := buf[:n]
		if len(data) < 4 {
			continue
		}
		_ = data[2]
		addrStr, port, _ := parseUDPAddr(data[3:])
		payload := data[3+addrLen(data[3:])+2:]
		_ = addr
		_ = addrStr
		_ = port

		connID := s.udpConns.Add(nil)
		pkt := tunnel.MarshalPacket(tunnel.Packet{
			Type:    tunnel.PacketTypeUDPData,
			ConnID:  connID,
			Payload: payload,
		})
		s.tunnel.Send(pkt)
		s.udpConns.Remove(connID)
	}
}

func parseAddr(buf []byte, r io.Reader) (string, int, error) {
	atyp := buf[3]
	switch atyp {
	case atypIPv4:
		addr := make([]byte, 4)
		if _, err := io.ReadFull(r, addr); err != nil {
			return "", 0, err
		}
		port := make([]byte, 2)
		if _, err := io.ReadFull(r, port); err != nil {
			return "", 0, err
		}
		return net.IP(addr).String(), int(port[0])<<8 | int(port[1]), nil
	case atypDomainName:
		n := make([]byte, 1)
		if _, err := io.ReadFull(r, n); err != nil {
			return "", 0, err
		}
		domain := make([]byte, n[0])
		if _, err := io.ReadFull(r, domain); err != nil {
			return "", 0, err
		}
		port := make([]byte, 2)
		if _, err := io.ReadFull(r, port); err != nil {
			return "", 0, err
		}
		return string(domain), int(port[0])<<8 | int(port[1]), nil
	case atypIPv6:
		addr := make([]byte, 16)
		if _, err := io.ReadFull(r, addr); err != nil {
			return "", 0, err
		}
		port := make([]byte, 2)
		if _, err := io.ReadFull(r, port); err != nil {
			return "", 0, err
		}
		return net.IP(addr).String(), int(port[0])<<8 | int(port[1]), nil
	}
	return "", 0, fmt.Errorf("unsupported address type: %d", atyp)
}

func parseUDPAddr(data []byte) (string, int, int) {
	if len(data) < 1 {
		return "", 0, 0
	}
	atyp := data[0]
	switch atyp {
	case atypIPv4:
		if len(data) < 7 {
			return "", 0, 0
		}
		ip := net.IP(data[1:5]).String()
		port := int(data[5])<<8 | int(data[6])
		return ip, port, 7
	case atypDomainName:
		if len(data) < 2 {
			return "", 0, 0
		}
		n := int(data[1])
		if len(data) < 2+n+2 {
			return "", 0, 0
		}
		domain := string(data[2 : 2+n])
		port := int(data[2+n])<<8 | int(data[2+n+1])
		return domain, port, 2 + n + 2
	case atypIPv6:
		if len(data) < 19 {
			return "", 0, 0
		}
		ip := net.IP(data[1:17]).String()
		port := int(data[17])<<8 | int(data[18])
		return ip, port, 19
	}
	return "", 0, 0
}

func addrLen(data []byte) int {
	if len(data) < 1 {
		return 0
	}
	switch data[0] {
	case atypIPv4:
		return 4
	case atypDomainName:
		if len(data) < 2 {
			return 0
		}
		return 1 + int(data[1])
	case atypIPv6:
		return 16
	}
	return 0
}

func parseSOCKSAddr(address string) ([]byte, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(host)
	if ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			return append([]byte{atypIPv4}, ip4...), nil
		}
		return append([]byte{atypIPv6}, ip.To16()...), nil
	}

	if len(host) > 255 {
		return nil, fmt.Errorf("domain too long")
	}
	buf := []byte{atypDomainName, byte(len(host))}
	buf = append(buf, []byte(host)...)
	_ = port
	return buf, nil
}

func writeBoundAddr(port int) []byte {
	return []byte{0, 0, 0, 0, 0, 0, byte(port >> 8), byte(port)}
}

func socksAddrToString(data []byte) string {
	if len(data) < 1 {
		return ""
	}
	atyp := data[0]
	var host string
	var offset int
	switch atyp {
	case atypIPv4:
		if len(data) < 5 {
			return ""
		}
		host = net.IP(data[1:5]).String()
		offset = 5
	case atypDomainName:
		if len(data) < 2 {
			return ""
		}
		n := int(data[1])
		if len(data) < 2+n {
			return ""
		}
		host = string(data[2 : 2+n])
		offset = 2 + n
	case atypIPv6:
		if len(data) < 17 {
			return ""
		}
		host = net.IP(data[1:17]).String()
		offset = 17
	default:
		return ""
	}
	if len(data) < offset+2 {
		return host
	}
	port := int(data[offset])<<8 | int(data[offset+1])
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func init() {
	_ = strings.TrimSpace
	_ = strconv.Itoa
}
