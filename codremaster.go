package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var db = struct {
	sync.RWMutex
	m map[string]int64
}{m: make(map[string]int64)}

var header string = "\xff\xff\xff\xff"

const (
	codmaster  = 20510
	codauth    = 20500
	cod2master = 20710
	cod2auth   = 20700
	cod4master = 20810
	cod4auth   = 20800
)

func main() {
	go listenMaster(cod4master)
	go listenAuth(cod4auth)
	go purge(6)
	http.HandleFunc("/getinfo", getinfoHandler)
	http.ListenAndServe(":8080", nil)
}

func listenMaster(port int) {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprint(":", port))
	if err != nil {
		log.Fatal("Could not resolve port for master.")
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal("Failed to open socket for master.")
	}

	defer conn.Close()

  fmt.Printf("Master server is listening on port %d...\n", port)
	for {
		var buf [1024]byte
		n, addr, err := conn.ReadFromUDP(buf[0:])
		if err != nil {
			return
		}
		msg := string(buf[0:n])
		endpoint := string(addr.IP.String()) + ":" + fmt.Sprint(addr.Port)

		switch {
		case strings.HasPrefix(msg[4:], "statusResponse"):
			db.Lock()
			db.m[endpoint] = time.Now().Unix()
			db.Unlock()
		case strings.HasPrefix(msg[4:], "getservers "):
			db.RLock()
			if len(db.m) == 0 {
				// db is empty
				db.RUnlock()
				continue
			}

			res := make([]byte, 0)
			innerCount := 0
			per := 20

			for k, _ := range db.m {
				current := make([]byte, 0)
				if innerCount == 0 {
					current = append(current, []byte(fmt.Sprint(header, "getserversResponse", "\n\x00\\"))...)
				}

				octets := strings.Split(k[:strings.Index(k, ":")], ".")
				for i := 0; i < 4; i++ {
					octet, err := strconv.Atoi(octets[i])
					if err != nil {
						continue
					}
					current = append(current, byte(octet))
				}

				addrport, err := strconv.Atoi(k[strings.Index(k, ":")+1:])
				if err != nil {
					continue
				}
				portbuf := &bytes.Buffer{}
				binary.Write(portbuf, binary.BigEndian, uint16(addrport))

				current = append(current, portbuf.Bytes()...)
				current = append(current, []byte("\\")...)

				res = append(res, current...)

				innerCount++
				if innerCount == per {
					res = append(res, []byte("EOT")...)
					conn.WriteToUDP(res, addr)

					// reset
					innerCount = 0
					res = make([]byte, 0)
				}
			}
			db.RUnlock()
			res = append(res, []byte("EOF")...)
			conn.WriteToUDP(res, addr)
		case msg[4:] == "heartbeat flatline":
			db.Lock()
			delete(db.m, endpoint)
			db.Unlock()
		case strings.HasPrefix(msg[4:], "heartbeat"): //server checking in to MS
			db.Lock()
			if _, ok := db.m[endpoint]; ok {
				// just checking in
				db.m[endpoint] = time.Now().Unix()
			} else {
				// new server
				nonce := generateNonce(9)
				conn.WriteToUDP([]byte(fmt.Sprint(header, "getchallenge ", nonce, "\n")), addr)
				conn.WriteToUDP([]byte(fmt.Sprint(header, "getstatus ", nonce, "\n")), addr)
			}
			db.Unlock()
		}
	}
}

// generateNonce creates a pseudorandom number.
// digits determines how long the number will be.
func generateNonce(digits int) string {
	nonce := &bytes.Buffer{}
	for i := 0; i < digits; i++ {
		nonce.WriteString(strconv.Itoa(rand.Intn(10)))
	}

	return nonce.String()
}

// listenAuth mimics the authentication server
// Packets are received but not acted upon.
func listenAuth(port int) {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprint(":", port))
	if err != nil {
		log.Fatal("Could not resolve port for auth.")
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal("Failed to open socket for auth.")
	}

	defer conn.Close()

	fmt.Printf("Authentication server is listening on port %d...\n", port)
	for {
		var buf [1024]byte
		n, _, err := conn.ReadFromUDP(buf[0:])
		if err != nil {
			return
		}

		msg := string(buf[0:n])

		if strings.HasPrefix(msg[4:], "getIpAuthorize") {
			// ignore
		}
	}
}

// purge removes inactive game servers from the database.
// A server is inactive if it has failed to send a hearbeat
// within the timeframe specified by interval in minutes.
func purge(interval int) {
	for {
		current := time.Now().Unix() - int64(interval*60)
		db.Lock()
		for k, v := range db.m {
			if v < current {
				delete(db.m, k)
			}
		}
		db.Unlock()
		time.Sleep(time.Duration(interval) * time.Minute)
	}
}

// getInfo queries the specified server.
func getInfo(addr string) (map[string]string, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	defer conn.Close()

	conn.Write([]byte("\xff\xff\xff\xffgetinfo xxx"))

	var buf [512]byte
	conn.SetReadDeadline(time.Now().Add(10000 * time.Millisecond))
	n, err := conn.Read(buf[0:])
	if err != nil {
		return nil, err
	}

	response := string(buf[0:n])
	fields := strings.Split(response[strings.Index(response, "\\")+1:], "\\")
	ir := make(map[string]string)
	for i := 0; i < len(fields)-1; i = i + 2 {
		ir[fields[i]] = fields[i+1]
	}

	return ir, nil
}

// removeColorCodes returns a copy of string s without Quake color codes.
func removeColorCodes(s string) string {
	re := regexp.MustCompile("\\^[0-7]")
	return re.ReplaceAllString(s, "")
}

func getinfoHandler(w http.ResponseWriter, r *http.Request) {
	response, err := getInfo(r.FormValue("addr"))

	if err != nil {
		fmt.Fprint(w, err)
		return
	}

	fmt.Fprint(w, response)
}
