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

func makePeerTime(timestamp int32, port uint16, expires uint8) PeerTime {
	return PeerTime(int64(timestamp) << 32 | int64(port) << 16 | int64(expires))
}

func (peertime PeerTime) Fields() (int32, uint16, uint8) {
	return int32(peertime >> 32), uint16((peertime >> 16) & 0xffff), uint8(peertime & 0xff)
}

func (database *PeerDatabase) Add(ip net.IP, timestamp int32, port uint16, expires uint8) int32 {
	database.Mutex.Lock()
	defer database.Mutex.Unlock()
	var ipBytes [16]byte
	var timestampNew = timestamp
	copy(ipBytes[:], ip)
	peertime, ok := database.Lookup[ipBytes]
	if ok {
		prevStamp, prevPort, _ := peertime.Fields()
		if (prevPort == port) {
			timestampNew = prevStamp
		}
	}
	database.Lookup[ipBytes] = makePeerTime(timestampNew, port, expires)
	return timestampNew
}

func (database *PeerDatabase) Tick(delta uint8) {
	database.Mutex.Lock()
	defer database.Mutex.Unlock()

	for ip, peertime := range database.Lookup {
		timestamp, port, expires := peertime.Fields()
		if expires <= delta {
			log.Printf("%v expired!", ip);
			delete(database.Lookup, ip)
		} else {
			database.Lookup[ip] = makePeerTime(timestamp, port, expires - delta)
		}
	}
}

func dumpIpPortPair(ip net.IP, port uint16) string {
	if (len(ip) == 16) {
		return fmt.Sprintf("[%s]:%d", ip.String(), port)
	} else {
		return fmt.Sprintf("%s:%d", ip.String(), port)
	}
}

func (database *PeerDatabase) Dump(lower int32, portFilter uint16, subnet net.IPNet) string {
	database.Mutex.RLock()
	defer database.Mutex.RUnlock()
	var response strings.Builder
	for ipBytes, peertime := range database.Lookup {
		current, port, _ := peertime.Fields()
		ip := net.IP(ipBytes[:len(subnet.IP)])
		if current < lower || !subnet.Contains(ip) || (portFilter != 0 && portFilter != port) {
			continue
		}
		response.WriteString(fmt.Sprintf("\"%s\"", dumpIpPortPair(ip, port)))
		response.WriteString(",")
	}
	return response.String()
}

var v4Database = PeerDatabase{ Lookup: make(map[[16]byte]PeerTime) }
var v6Database = PeerDatabase{ Lookup: make(map[[16]byte]PeerTime) }
var srvEpoch = time.Now()

var expireInit = uint8(60)
var expirePeriod = uint8(10)
const portDefault = uint16(443)

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

func handleRegister(w http.ResponseWriter, r *http.Request) (int, error) {
	err := r.ParseForm()
	if err != nil {
		return http.StatusBadRequest, err
	}

	timestamp := int32(time.Now().Sub(srvEpoch) / 1_000_000_000)

	var port = portDefault
	ports := r.PostForm["port"]
	if len(ports) != 0 {
		parsed, err := strconv.ParseUint(ports[0], 10, 16)
		if err != nil {
			return http.StatusBadRequest, err
		}
		port = uint16(parsed)
	}

	ip, err := getRemoteIp(r.RemoteAddr)
	if err != nil {
		return http.StatusInternalServerError, err
	}


	ip4, expires := ip.To4(), expireInit
	var timestampNew int32
	if ip4 != nil {
		timestampNew = v4Database.Add(ip4, timestamp, port, expires)
	} else {
		timestampNew = v6Database.Add(ip, timestamp, port, expires)
	}

	fmt.Fprintf(w, "{\"peer\":\"%s\",\"timestamp\":%d,\"expires\":%d}",
		dumpIpPortPair(ip, port), timestampNew, expires)
	return http.StatusOK, nil
}

func handleQuery(w http.ResponseWriter, r *http.Request) (int, error) {
	err := r.ParseForm()
	if err != nil {
		return http.StatusBadRequest, err
	}

	timestamps, preferences := r.Form["timestamp"], r.Form["prefer"]
	subnets4, subnets6 := r.Form["subnet4"], r.Form["subnet6"]
	ports := r.Form["port"]
	now := (time.Now().Sub(srvEpoch) / 1_000_000_000)
	var _, subnet4, _ = net.ParseCIDR("0.0.0.0/0")
	var _, subnet6, _ = net.ParseCIDR("::/0")
	var port = uint16(0)
	var timestamp = int32(0)
	var prefer = "4";

	if len(ports) != 0 {
		parsed, err := strconv.ParseUint(ports[0], 10, 16)
		if err != nil {
			return http.StatusBadRequest, err
		}
		port = uint16(parsed)
	}

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

	if len(subnets4) != 0 {
		_, tmp, err := net.ParseCIDR(subnets4[0])
		if err != nil {
			return http.StatusBadRequest, err
		}
		if len(tmp.IP) != 4 {
			return http.StatusBadRequest,
			errors.New(fmt.Sprintf(
				"Specified IPv6 CIDR '%s' as IPv4 subnet",
				tmp.String()))
		}
		subnet4 = tmp
	}

	if len(subnets6) != 0 {
		_, tmp, err := net.ParseCIDR(subnets6[0])
		if err != nil {
			return http.StatusBadRequest, err
		}
		if len(tmp.IP) != 16 {
			return http.StatusBadRequest,
			errors.New(fmt.Sprintf(
				"Specified IPv4 CIDR '%s' as IPv6 subnet",
				tmp.String()))
		}
		subnet6 = tmp
	}


	var response = ""
	var peers = ""
	if prefer == "4" {
		peers = v4Database.Dump(timestamp, port, *subnet4) + v6Database.Dump(timestamp, port, *subnet6)
	} else {
		peers = v6Database.Dump(timestamp, port, *subnet6) + v4Database.Dump(timestamp, port, *subnet4)
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
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		code, err := handleRegister(w, r)
		if err != nil {
			w.WriteHeader(code)
			log.Printf("[ERROR /register]: %s", err)
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
