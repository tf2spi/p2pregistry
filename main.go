package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
)

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

func handleBroadcast(w http.ResponseWriter, r *http.Request) (int, error) {
	err := r.ParseForm()
	if err != nil {
		return http.StatusBadRequest, err
	}
	ports, protocols := r.Form["port"], r.Form["protocol"]
	if len(ports) != 1 || len(protocols) != 1 {
		return http.StatusBadRequest,
			errors.New(fmt.Sprintf("Expected exactly 1 protocol and port, got %d protocols and %d ports",
				len(protocols), len(ports)))
	}
	port, err := strconv.Atoi(ports[0])
	if err != nil {
		return http.StatusBadRequest, err
	}
	protocol := protocols[0]
	if protocol != "tls" && protocol != "tcp" && protocol != "udp" {
		return http.StatusBadRequest,
			errors.New(fmt.Sprintf("Expected 'tls', 'tcp', or 'udp' protocol, got '%s'", protocol))
	}
	ip, err := getRemoteIp(r.RemoteAddr)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	log.Printf("Your address: %s\n", ip.String())
	log.Printf("%d : %s\n", port, protocol)
	log.Printf("Will save these in a database eventually...\n")
	return http.StatusOK, nil
}

func handleQuery(w http.ResponseWriter, r *http.Request) (int, error) {
	log.Print("We'll handle queries eventually here...")
	// Filter ideas:
	// * Filter by timestamp so only newer entries are shown
	// * Filter to return only ipv6 or ipv4 addresses
	// * Filter based on ip address, port, and transport layer protocol
	return http.StatusOK, nil
}

func main() {
	http.HandleFunc("/broadcast", func(w http.ResponseWriter, r *http.Request) {
		code, err := handleBroadcast(w, r)
		w.WriteHeader(code)
		if err != nil {
			log.Printf("[ERROR /broadcast]: %s", err)
			fmt.Fprint(w, err)
		}
	})
	http.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		code, err := handleQuery(w, r)
		w.WriteHeader(code)
		if err != nil {
			log.Printf("[ERROR /query]: %s", err)
			fmt.Fprint(w, err)
		}
	})
	log.Print("Listening on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
