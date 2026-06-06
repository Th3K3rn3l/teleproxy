package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/teleproxy/internal/goloom"
	"github.com/teleproxy/internal/proxy"
)

func main() {
	uid := flag.String("uid", "", "Yandex UID")
	sessionCookie := flag.String("session", "", "Session_id cookie from yandex.ru")
	displayName := flag.String("name", "ProxyServer", "Display name in conference")
	conferenceURL := flag.String("url", "", "Conference URL to join")
	listenAddr := flag.String("listen", "", "Create conference instead of joining")
	flag.Parse()

	if *uid == "" {
		log.Fatal("-uid is required (Yandex UID)")
	}

	cfg := goloom.SessionConfig{
		UID:           *uid,
		SessionCookie: *sessionCookie,
		DisplayName:   *displayName,
	}

	if *listenAddr != "" {
		cfg.Mode = goloom.ModeCreate
		fmt.Printf("Creating conference as server...\n")
	} else if *conferenceURL != "" {
		cfg.Mode = goloom.ModeJoin
		cfg.ConferenceURI = *conferenceURL
		fmt.Printf("Joining conference %s...\n", *conferenceURL)
	} else {
		cfg.Mode = goloom.ModeCreate
		fmt.Printf("Creating conference as server...\n")
	}

	session := goloom.NewSession(cfg)

	var handler *proxy.Handler

	session.OnData(func(data []byte) {
		if handler != nil {
			handler.HandlePacket(data)
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

	if err := session.Start(ctx); err != nil {
		log.Fatalf("Failed to start session: %v", err)
	}

	handler = proxy.NewHandler(func(data []byte) error {
		return session.Send(data)
	})

	log.Println("Waiting for client to connect via DataChannel...")
	if err := session.WaitForDataChannel(ctx); err != nil {
		log.Fatalf("Failed to establish data channel: %v", err)
	}
	log.Println("DataChannel established! Server ready.")

	if *listenAddr != "" {
		fmt.Printf("Conference URL: %s\n", session.ConferenceURL())
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")
	session.Close()
}
