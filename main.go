package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/pion/turn/v2"
	"github.com/soheilhy/cmux"
)

// Example implementing Long-Term Credential Mechanism (RFC5389-10.2: https://tools.ietf.org/search/rfc5389#section-10.2)
func main() {
	publicIP := flag.String("public-ip", "", "IP Address that TURN can be contacted by.")
	port := flag.Int("port", 3478, "Listening port.")
	authSecret := flag.String("static-auth-secret", "", "Long-term auth secret")
	httpServer := flag.String("http-server", "", "Split regular HTTP traffic to a different server")
	realm := flag.String("realm", "pion.ly", "Realm (defaults to \"pion.ly\")")
	flag.Parse()

	if len(*publicIP) == 0 {
		log.Fatalf("'public-ip' is required")
	} else if len(*authSecret) == 0 {
		log.Fatalf("'static-auth-secret' is required")
	}

	// https://github.com/soheilhy/cmux

	// Create a UDP listener to pass into pion/turn
	// pion/turn itself doesn't allocate any UDP sockets, but lets the user pass them in
	// this allows us to add logging, storage or modify inbound/outbound traffic
	udpListener, err := net.ListenPacket("udp4", "0.0.0.0:"+strconv.Itoa(*port))
	if err != nil {
		log.Panicf("Failed to create TURN server listener: %s", err)
	}

	// Create a TCP listener to pass into pion/turn
	// pion/turn itself doesn't allocate any TCP listeners, but lets the user pass them in
	// this allows us to add logging, storage or modify inbound/outbound traffic
	tcpListener, err := net.Listen("tcp4", "0.0.0.0:"+strconv.Itoa(*port))
	if err != nil {
		log.Panicf("Failed to create TURN server listener: %s", err)
	}

	// Cmux: split HTTP traffic to an external server
	if len(*httpServer) != 0 {
		remoteURL, err := url.Parse(*httpServer)
		if err != nil {
			log.Panicf("Failed to parse -httpServer reverse proxy attribute")
		}
		m := cmux.New(tcpListener)
		httpL := m.Match(cmux.HTTP1Fast()) // HTTP1
		http2L := m.Match(cmux.HTTP2())    // HTTP2
		tcpListener = m.Match(cmux.Any())  // Any means anything that is not yet matched.
		// Serve HTTP traffic with a SingleHostReverseProxy
		httpServer := &http.Server{Handler: httputil.NewSingleHostReverseProxy(remoteURL)}
		go httpServer.Serve(httpL)
		go httpServer.Serve(http2L)
	}

	// Cache -users flag for easy lookup later
	// If passwords are stored they should be saved to your DB hashed using turn.GenerateAuthKey
	// usersMap := map[string][]byte{}
	// for _, kv := range regexp.MustCompile(`(\w+)=(\w+)`).FindAllStringSubmatch(*users, -1) {
	// 	usersMap[kv[1]] = turn.GenerateAuthKey(kv[1], *realm, kv[2])
	// }

	s, err := turn.NewServer(turn.ServerConfig{
		Realm: *realm,
		// Set AuthHandler callback
		// This is called everytime a user tries to authenticate with the TURN server
		// Return the key for that user, or false when no user is found
		AuthHandler: func(username string, realm string, srcAddr net.Addr) ([]byte, bool) {
			log.Printf("Authentication username=%q realm=%q srcAddr=%v\n", username, realm, srcAddr)
			// if key, ok := usersMap[username]; ok {
			// 	return key, true
			// }
			return nil, false
		},
		// PacketConnConfigs is a list of UDP Listeners and the configuration around them
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: udpListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP(*publicIP), // Claim that we are listening on IP passed by user (This should be your Public IP)
					Address:      "0.0.0.0",              // But actually be listening on every interface
				},
			},
		},
		// ListenerConfigs is a list of TCP Listeners and the configuration around them
		ListenerConfigs: []turn.ListenerConfig{
			{
				Listener: tcpListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP(*publicIP), // Claim that we are listening on IP passed by user (This should be your Public IP)
					Address:      "0.0.0.0",              // But actually be listening on every interface
				},
			},
		},
	})
	if err != nil {
		log.Panic(err)
	}

	// Block until user sends SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	if err = s.Close(); err != nil {
		log.Panic(err)
	}
}
