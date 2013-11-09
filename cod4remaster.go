package main

import (
	"fmt"
	"github.com/cznic/kv"
	"net"
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

	select {}
}

func listenMaster(port string) {
	udpAddr, _ := net.ResolveUDPAddr("udp", port)
	conn, _ := net.ListenUDP("udp", udpAddr)
	defer conn.Close()

	fmt.Println("Master Server is listening...")
	for {
		var buf [1024]byte
		n, addr, err := conn.ReadFromUDP(buf[0:])
		if err != nil {
			return
		}
		msg := string(buf[0:n])
		endpoint := string(addr.IP[:]) + ":" + string(addr.Port)

		if strings.HasPrefix(msg[4:], "statusResponse") {
			db.Set([]byte(endpoint), []byte(fmt.Sprint(time.Now().Unix())))
		} else {
			switch msg[4:] {
			case "getservers 6 full empty":

				// send back all servers
				e, err := db.SeekFirst()
				if err != nil {
					// db is empty
					continue
				}

				current := ""
				innerCount := 0
				per := 20
				for {
					k, _, err := e.Next()
					if err != nil {
						current += "EOF"
						conn.WriteToUDP([]byte(current), addr)
						fmt.Println("EOF packet ", current)
						break
					}

					if innerCount == 0 {
						current = header + "getserversResponse\n\x00\\"
					}

					current += strings.Join(strings.Split(string(k), ":"), "") + "\\"

					innerCount++
					if innerCount == per {
						current += "EOT"
						conn.WriteToUDP([]byte(current), addr)
						fmt.Println("EOT packet ", current)

						// reset
						innerCount = 0
					}
				}

			case "heartbeat COD-4\n": //server checking in to MS
				//buf := make([]byte, 8)
				v, _ := db.Get(nil, []byte(endpoint)) //todo err
				if v == nil {
					// not found, new server
					conn.WriteToUDP([]byte(header+"getchallenge 123456789\n"), addr)
					conn.WriteToUDP([]byte(header+"getstatus 123456789\n"), addr)
				} else {
					db.Set([]byte(endpoint), []byte(fmt.Sprint(time.Now().Unix())))
				}
			case "heartbeat flatline": //server is shutting down, remove from list
				fmt.Println(endpoint, " flatlined.")
				db.Delete([]byte(endpoint))
			}
		}
	}
}

func listenAuth(port string) {
	udpAddr, _ := net.ResolveUDPAddr("udp", port)
	conn, _ := net.ListenUDP("udp", udpAddr)
	defer conn.Close()

	fmt.Println("Authentication Server is listening...")
	for {
		var buf [1024]byte
		n, _, err := conn.ReadFromUDP(buf[0:])
		if err != nil {
			return
		}

		msg := string(buf[0:n])

		if strings.HasPrefix(msg[4:], "getIpAuthorize") {
			// received packet to auth server
			// do nothing, Activision shut down the master server
			// after all.
			//conn.WriteToUDP([]byte(header+"received"), addr)
			fmt.Println("getIpAuthorize received. Ignore it.")
		}
	}
}
