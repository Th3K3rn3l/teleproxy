package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/teleproxy/internal/goloom"
	"github.com/teleproxy/internal/proxy"
)

func main() {
	conferenceURL := flag.String("url", "", "Conference URL from telemost.yandex.ru")
	displayName := flag.String("name", "ProxyServer", "Display name in conference")
	flag.Parse()

	if *conferenceURL == "" {
		log.Fatal("-url is required (paste the Telemost conference link)")
	}

	cfg := goloom.SessionConfig{
		DisplayName:   *displayName,
		ConferenceURI: *conferenceURL,
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("Connecting to conference...\n")

	for attempt := 1; ; attempt++ {
		disconnected := make(chan struct{})
		stopped := make(chan struct{})

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
			select {
			case <-disconnected:
			default:
				close(disconnected)
			}
		})

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			select {
			case <-sig:
				cancel()
				session.Close()
			case <-disconnected:
			}
			close(stopped)
		}()

		if err := session.Start(ctx); err != nil {
			log.Printf("Attempt %d failed: %v", attempt, err)
			cancel()
			<-stopped
			time.Sleep(3 * time.Second)
			continue
		}

		handler = proxy.NewHandler(func(data []byte) error {
			return session.Send(data)
		})

		log.Printf("Waiting for data channel (attempt %d)...", attempt)
		if err := session.WaitForDataChannel(ctx); err != nil {
			log.Printf("Data channel timeout (attempt %d): %v", attempt, err)
			session.Close()
			<-stopped
			time.Sleep(3 * time.Second)
			continue
		}
		fmt.Println("Proxy server ready.")

		<-stopped

		select {
		case <-sig:
			os.Exit(0)
		default:
		}
		log.Println("Reconnecting in 3 seconds...")
		time.Sleep(3 * time.Second)
	}
}
