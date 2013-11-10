package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/cznic/kv"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

var (
	db     *kv.DB
	header string = "\xff\xff\xff\xff"
)

func main() {
	db, _ = kv.CreateMem(&kv.Options{})
	go listenMaster(":20810")
	go listenAuth(":20800")
	go purge(2)
	select {}
}

func listenMaster(port string) {
	udpAddr, err := net.ResolveUDPAddr("udp", port)
	if err != nil {
		log.Fatal("Could not resolve port for master.")
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal("Failed to open socket for master.")
	}

	defer conn.Close()

	fmt.Println("Master server is listening...")
	for {
		var buf [1024]byte
		n, addr, err := conn.ReadFromUDP(buf[0:])
		if err != nil {
			return
		}
		msg := string(buf[0:n])
		endpoint := string(addr.IP.String()) + ":" + fmt.Sprint(addr.Port)

		if strings.HasPrefix(msg[4:], "statusResponse") {
			db.Set([]byte(endpoint), []byte(fmt.Sprint(time.Now().Unix())))
		} else {
			switch msg[4:] {
			case "getservers 6 full empty":
				e, err := db.SeekFirst()
				if err != nil {
					// db is empty
					continue
				}

				cbuf := make([]byte, 0)
				innerCount := 0
				per := 20

				for {
					k, _, err := e.Next()

					if err != nil {
						cbuf = append(cbuf, []byte("EOF")...)
						conn.WriteToUDP(cbuf, addr)
						break
					}

					if innerCount == 0 {
						cbuf = append(cbuf, []byte(header+"getserversResponse\n\x00\\")...)
					}

					octets := strings.Split(strings.Split(string(k), ":")[0], ".")
					for i := 0; i < 4; i++ {
						ioctet, _ := strconv.Atoi(octets[i])
						cbuf = append(cbuf, byte(ioctet))
					}
					addrport, _ := strconv.Atoi(strings.Split(string(k), ":")[1])
					portbuf := &bytes.Buffer{}
					binary.Write(portbuf, binary.BigEndian, uint16(addrport))
					cbuf = append(cbuf, portbuf.Bytes()...)
					cbuf = append(cbuf, []byte("\\")...)

					innerCount++
					if innerCount == per {
						cbuf = append(cbuf, []byte("EOT")...)
						conn.WriteToUDP(cbuf, addr)

						// reset
						innerCount = 0
						cbuf = make([]byte, 20)
					}
				}

			case "heartbeat COD-4\n": //server checking in to MS
				v, _ := db.Get(nil, []byte(endpoint)) //todo err
				if v == nil {
					// new server
					nonce := generateNonce(9)
					conn.WriteToUDP([]byte(fmt.Sprint(header, "getchallenge ", nonce, "\n")), addr)
					conn.WriteToUDP([]byte(fmt.Sprint(header, "getstatus ", nonce, "\n")), addr)
				} else {
					// just checking in
					db.Set([]byte(endpoint), []byte(fmt.Sprint(time.Now().Unix())))
				}
			case "heartbeat flatline":
				db.Delete([]byte(endpoint))
			}
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
func listenAuth(port string) {
	udpAddr, err := net.ResolveUDPAddr("udp", port)
	if err != nil {
		log.Fatal("Could not resolve port for auth.")
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal("Failed to open socket for auth.")
	}

	defer conn.Close()

	fmt.Println("Authentication server is listening...")
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

// purge removes inactive game servers from the database
// A server is inactive if it has failed to send a hearbeat
// within the timeframe specified by interval in minutes.
func purge(interval int) {
	for {
		current := time.Now().Unix() - int64(interval*60)

		e, err := db.SeekFirst()
		for {
			if err != nil {
				break
			}

			k, v, err := e.Next()

			if err != nil {
				break
			}

			last, err := strconv.Atoi(string(v))
			if err != nil || last < int(current) {
				db.Delete(k)
			}
		}
		time.Sleep(time.Duration(interval) * time.Minute)
	}
}
