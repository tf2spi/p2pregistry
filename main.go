package main

import (
	"errors"
	"time"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
)

type Peer struct {
	Ip net.IP
	Protocol string
	Timestamp int64
	Port int
	Expires int
}

// Table of peers for IPV4 and IPV6 should both be keyed by only ip address.
// Servers should create reverse proxies if they want multiple discovery ports.
var database = []Peer{}

func getRemoteIp(addr string) (net.IP, error) {
	var ip []byte
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return ip, err
	}
	ipParsed := net.ParseIP(host)
	if ipParsed != nil {
		ip = ipParsed
		return ip, nil
	}
	return ip, errors.New(fmt.Sprintf("IP of remote addr '%s' not found!", addr))
}

func handleBroadcast(w http.ResponseWriter, r *http.Request) (int, string, error) {
	err := r.ParseForm()
	if err != nil {
		return http.StatusBadRequest, "", err
	}
	ports, protocols := r.Form["port"], r.Form["protocol"]
	if len(ports) != 1 || len(protocols) != 1 {
		return http.StatusBadRequest,
		"",
		errors.New(fmt.Sprintf(
			"Expected exactly 1 protocol and port, got %d protocols and %d ports",
			len(protocols), len(ports)))
	}
	port, err := strconv.Atoi(ports[0])
	if err == nil && (port <= 0 || port > 65535) {
		err = errors.New(fmt.Sprintf(
			"Port number overflow! ('%d' not in [1,65535])",
			port))
	}
	if err != nil {
		return http.StatusBadRequest, "", err
	}
	protocol := protocols[0]
	if protocol != "dtls" && protocol != "tls" && protocol != "tcp" && protocol != "udp" {
		return http.StatusBadRequest,
		"",
		errors.New(fmt.Sprintf(
			"Expected 'dtls', 'tls', 'tcp', or 'udp' protocol, got '%s'",
			protocol))
	}
	ip, err := getRemoteIp(r.RemoteAddr)
	if err != nil {
		return http.StatusInternalServerError, "", err
	}
	peer := Peer{
		Ip: ip,
		Protocol: protocol,
		Port: port,
		Timestamp: time.Now().Unix(),
		Expires: 86400,
	}
	database = append(database, peer)
	log.Printf("Your address: %s\n", ip.String())
	log.Printf("%d : %s\n", port, protocol)
	log.Printf("Will save these in a database eventually...\n")
	log.Printf("Peers:\n")
	for _, peer := range database {
		log.Printf("%v\n", peer)
	}
	return http.StatusOK, fmt.Sprintf("%s:%d", ip.String(), port), nil
}

func handleQuery(w http.ResponseWriter, r *http.Request) (int, string, error) {
	log.Print("We'll handle queries eventually here...")
	// Filter ideas:
	// * Filter by timestamp so only newer entries are shown
	// * Filter to return only ipv6 or ipv4 addresses
	// * Filter based on ip address, port, and transport layer protocol
	response := ""
	for _, peer := range database {
		response += fmt.Sprintf("%v\n", peer)
	}
	return http.StatusOK, response, nil
}

func main() {
	http.HandleFunc("/broadcast", func(w http.ResponseWriter, r *http.Request) {
		code, response, err := handleBroadcast(w, r)
		w.WriteHeader(code)
		if err != nil {
			log.Printf("[ERROR /broadcast]: %s", err)
			fmt.Fprint(w, err)
		}
		fmt.Fprintf(w, response)
	})
	http.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		code, response, err := handleQuery(w, r)
		w.WriteHeader(code)
		if err != nil {
			log.Printf("[ERROR /query]: %s", err)
			fmt.Fprint(w, err)
		}
		fmt.Fprintf(w, response)
	})
	log.Print("Listening on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
