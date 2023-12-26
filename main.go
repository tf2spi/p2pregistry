package main

import (
	"errors"
	"net"
	"fmt"
	"log"
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

func main() {
	http.HandleFunc("/broadcast", func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Print(err)
			fmt.Fprint(w, err)
			return
		}
		ports, protocols := r.Form["port"], r.Form["protocol"]
		if len(ports) != 1 || len(protocols) != 1 {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Expected exactly 1 protocol and port, got %d protocols and %d ports",
				len(protocols), len(ports))
			return
		}
		port, err := strconv.Atoi(ports[0])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Unable to parse port as integer: %s", err)
			return
		}
		protocol := protocols[0]
		if protocol != "tcp" && protocol != "udp" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Expected 'tcp' or 'udp' protocol, got '%s'", protocol)
			return
		}
		ip, err := getRemoteIp(r.RemoteAddr)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w,err)
			return
		}
		fmt.Fprintf(w, "Your address: %s\n", ip.String())
		fmt.Fprintf(w, "%d : %s\n", port, protocol)
		fmt.Fprintf(w, "Will save these in a database eventually...\n")
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
