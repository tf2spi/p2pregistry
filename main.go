package main

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"net"
	"net/http"
	"strconv"
	"time"
	"reflect"
	"strings"
)

type PeerDatabase struct {
	Mutex sync.RWMutex
	Ip []net.IP
	Timestamp []int64
	Expires []uint8
};

func peerUrlEncode(ip net.IP, timestamp int64, expires uint8) string {
	return fmt.Sprintf("ip=%s&timestamp=%d&expires=%d", ip.String(), timestamp, expires);
}

func (database *PeerDatabase) ContainsKey(ip net.IP) bool {
	database.Mutex.RLock()
	defer database.Mutex.RUnlock()
	for _, dataip := range database.Ip {
		if reflect.DeepEqual(dataip, ip) {
			log.Print("IP already in database\n")
			return true
		}
	}
	return false
}

func (database *PeerDatabase) Add(ip net.IP, timestamp int64, expires uint8) {
	if (database.ContainsKey(ip)) {
		return
	}
	database.Mutex.Lock()
	defer database.Mutex.Unlock()
	database.Ip = append(database.Ip, ip)
	database.Timestamp = append(database.Timestamp, timestamp)
	database.Expires = append(database.Expires, expires)
}

func (database *PeerDatabase) Dump(timestamp int64) string {
	database.Mutex.RLock()
	defer database.Mutex.RUnlock()
	var response strings.Builder
	for i := range database.Ip {
		current := database.Timestamp[i]
		if current < timestamp {
			continue
		}
		response.WriteString(peerUrlEncode(
			database.Ip[i],
			current,
			database.Expires[i]))
		response.WriteString("\n")
	}
	return response.String()
}

var v4database = PeerDatabase{};
var v6database = PeerDatabase{};

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

	timestamps := r.PostForm["timestamp"]
	var timestamp = int64(0)

	if len(timestamps) != 0 {
		parsed, err := strconv.ParseInt(timestamps[0], 10, 64)
		if err != nil {
			return http.StatusBadRequest, err
		}
		timestamp = parsed
	} else {
		timestamp = time.Now().Unix()
	}

	ip, err := getRemoteIp(r.RemoteAddr)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	ip4, expires := ip.To4(), uint8(60)
	if ip4 != nil {
		v4database.Add(ip4, timestamp, expires)
	} else {
		v6database.Add(ip, timestamp, expires)
	}

	fmt.Fprintf(w, peerUrlEncode(ip, timestamp, expires))
	return http.StatusOK, nil
}

func handleQuery(w http.ResponseWriter, r *http.Request) (int, error) {
	err := r.ParseForm()
	if err != nil {
		return http.StatusBadRequest, err
	}

	timestamps, preferences := r.PostForm["timestamp"], r.PostForm["prefer"]
	var timestamp = int64(0)
	var prefer = "4";

	if len(timestamps) != 0 {
		parsed, err := strconv.ParseInt(timestamps[0], 10, 64)
		if err != nil {
			return http.StatusBadRequest, err
		}
		timestamp = parsed
	}

	if len(preferences) != 0 {
		prefer = preferences[0]
	}
	if prefer != "4" && prefer != "6" {
		return http.StatusBadRequest,
		errors.New(fmt.Sprintf(
			"Expected preference of '4' or '6', got '%s'",
			prefer))
	}

	var response = ""
	if prefer == "4" {
		response = v4database.Dump(timestamp) + v6database.Dump(timestamp)
	} else {
		response = v6database.Dump(timestamp) + v4database.Dump(timestamp)
	}
	log.Print(response)
	fmt.Fprint(w, response)
	return http.StatusOK, nil
}

func main() {
	http.HandleFunc("/broadcast", func(w http.ResponseWriter, r *http.Request) {
		code, err := handleBroadcast(w, r)
		if err != nil {
			w.WriteHeader(code)
			log.Printf("[ERROR /broadcast]: %s", err)
			fmt.Fprint(w, err)
		}
	})
	http.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		code, err := handleQuery(w, r)
		if err != nil {
			w.WriteHeader(code)
			log.Printf("[ERROR /query]: %s", err)
			fmt.Fprint(w, err)
		}
	})
	log.Print("Listening on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
