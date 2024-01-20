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
	"strings"
)

type PeerTime int64

// I'd like to use generics to optimize IPv4 addresses
// to be [4]byte when specified but Go gives me a hard
// time with that right now...
type PeerDatabase struct {
	Lookup map[[16]byte]PeerTime
	Mutex sync.RWMutex
};

func makePeerTime(timestamp int32, expires uint8) PeerTime {
	return PeerTime(int64(timestamp) << 32 | int64(expires))
}

func (peertime PeerTime) Fields() (int32, uint8) {
	return int32(peertime >> 32), uint8(peertime & 0xff)
}

func (database *PeerDatabase) Add(ip net.IP, timestamp int32, expires uint8) {
	database.Mutex.Lock()
	defer database.Mutex.Unlock()
	var ipbytes [16]byte
	copy(ipbytes[:], ip)
	peertime, ok := database.Lookup[ipbytes]
	if ok {
		prevstamp, _ := peertime.Fields()
		database.Lookup[ipbytes] = makePeerTime(prevstamp, expires)
	} else {
		database.Lookup[ipbytes] = makePeerTime(timestamp, expires)
	}
}

func (database *PeerDatabase) Tick(delta uint8) {
	database.Mutex.Lock()
	defer database.Mutex.Unlock()

	for ip, peertime := range database.Lookup {
		timestamp, expires := peertime.Fields()
		if expires <= delta {
			log.Printf("%v expired!", ip);
			delete(database.Lookup, ip)
		} else {
			database.Lookup[ip] = makePeerTime(timestamp, expires - delta)
		}
	}
}

func (database *PeerDatabase) Dump(lower int32, iplen int) string {
	database.Mutex.RLock()
	defer database.Mutex.RUnlock()
	var response strings.Builder
	for ip, peertime := range database.Lookup {
		current, _ := peertime.Fields()
		if current < lower {
			continue
		}
		response.WriteString(fmt.Sprintf("\"%s\"", net.IP(ip[:iplen]).String()))
		response.WriteString(",")
	}
	return response.String()
}

var v4Database = PeerDatabase{ Lookup: make(map[[16]byte]PeerTime) };
var v6Database = PeerDatabase{ Lookup: make(map[[16]byte]PeerTime) };
var srvEpoch = time.Now();

var expireInit = uint8(60)
var expirePeriod = uint8(10);

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

	timestamp := int32(time.Now().Sub(srvEpoch) / 1_000_000_000)

	ip, err := getRemoteIp(r.RemoteAddr)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	ip4, expires := ip.To4(), expireInit
	if ip4 != nil {
		v4Database.Add(ip4, timestamp, expires)
	} else {
		v6Database.Add(ip, timestamp, expires)
	}

	fmt.Fprintf(w, "{\"ip\":\"%s\",\"timestamp\":%d,\"expires\":%d}",
		ip, timestamp, expires)
	return http.StatusOK, nil
}

func handleQuery(w http.ResponseWriter, r *http.Request) (int, error) {
	err := r.ParseForm()
	if err != nil {
		return http.StatusBadRequest, err
	}

	timestamps, preferences := r.Form["timestamp"], r.Form["prefer"]
	now := (time.Now().Sub(srvEpoch) / 1_000_000_000)
	var timestamp = int32(0)
	var prefer = "4";

	if len(timestamps) != 0 {
		parsed, err := strconv.ParseInt(timestamps[0], 10, 32)
		if err != nil {
			return http.StatusBadRequest, err
		}
		timestamp = int32(parsed)
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
	var peers = ""
	if prefer == "4" {
		peers = v4Database.Dump(timestamp, 4) + v6Database.Dump(timestamp, 16)
	} else {
		peers = v6Database.Dump(timestamp, 16) + v4Database.Dump(timestamp, 4)
	}
	if len(peers) != 0 {
		peers = peers[:len(peers) - 1]
	}

	response = fmt.Sprintf("{\"now\":%d,\"peers\":[%s]}", now, peers)
	log.Print(response)
	fmt.Fprint(w, response)
	return http.StatusOK, nil
}

func main() {
	go func() {
		period := time.Duration(expirePeriod) * time.Second
		timer := time.NewTimer(period)
		for {
			<-timer.C
			log.Print("Expire period elapsed!\n")
			v4Database.Tick(expirePeriod)
			v6Database.Tick(expirePeriod)
			timer.Reset(period)
		}

	}();
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
