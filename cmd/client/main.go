package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/teleproxy/internal/goloom"
	"github.com/teleproxy/internal/proxy"
	"github.com/teleproxy/internal/tunnel"
)

func main() {
	conferenceURL := flag.String("url", "", "Conference URL from telemost.yandex.ru")
	uid := flag.String("uid", "", "Yandex UID (optional, for REST API join)")
	sessionCookie := flag.String("session", "", "Session_id cookie (optional, for REST API join)")
	displayName := flag.String("name", "ProxyClient", "Display name in conference")
	socksAddr := flag.String("socks", "127.0.0.1:1080", "SOCKS5 listen address")
	flag.Parse()

	if *conferenceURL == "" {
		log.Fatal("-url is required (paste the Telemost conference link)")
	}

	cfg := goloom.SessionConfig{
		DisplayName:   *displayName,
		ConferenceURI: *conferenceURL,
		UID:           *uid,
		SessionCookie: *sessionCookie,
	}

	session := goloom.NewSession(cfg)

	tracker := tunnel.NewConnTracker()

	session.OnData(func(data []byte) {
		pkt, err := tunnel.UnmarshalPacket(data)
		if err != nil {
			return
		}
		switch pkt.Type {
		case tunnel.PacketTypeTCPData:
			conn, ok := tracker.Get(pkt.ConnID)
			if !ok {
				return
			}
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				tcpConn.Write(pkt.Payload)
			}
		case tunnel.PacketTypeTCPClose:
			tracker.Remove(pkt.ConnID)
		case tunnel.PacketTypeError:
			log.Printf("Server error for conn %d: %s", pkt.ConnID, string(pkt.Payload))
			tracker.Remove(pkt.ConnID)
		}
	})

	session.OnError(func(err error) {
		log.Printf("Session error: %v", err)
	})

	session.OnClose(func() {
		log.Println("Session closed")
		os.Exit(1)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Printf("Connecting to conference...\n")

	if err := session.Start(ctx); err != nil {
		log.Fatalf("Failed to start session: %v", err)
	}

	socksServer := proxy.NewSOCKS5Server(*socksAddr, session, tracker)
	if err := socksServer.Start(); err != nil {
		log.Fatalf("Failed to start SOCKS5: %v", err)
	}

	log.Println("Waiting for data channel...")
	if err := session.WaitForDataChannel(ctx); err != nil {
		log.Fatalf("Failed to establish data channel: %v", err)
	}

	fmt.Printf("SOCKS5 proxy listening on %s\n", *socksAddr)
	fmt.Println("Configure your browser/OS to use this SOCKS5 proxy")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")
	session.Close()
}
