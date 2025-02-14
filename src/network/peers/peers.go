package peers

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"time"

	"../conn"
	"../localip"
)

type Peer struct {
	Id       string
	Ip       string
	TcpPort  int
	lastSeen time.Time
}
type PeerUpdate struct {
	Peers         []Peer
	TCPconnUpdate bool
	//New   Peer
	//Lost  []Peer
}

const interval = 10 * time.Millisecond
const timeout = 1000 * time.Millisecond

func Transmitter(udpPort int, id string, tcpPort int) {

	conn := conn.DialBroadcastUDP(udpPort)
	addr, _ := net.ResolveUDPAddr("udp4", fmt.Sprintf("255.255.255.255:%d", udpPort))
	var localIP string
	//Dont start transmitter until we get our IP, in case no network on startup
	for {
		var err error
		localIP, err = localip.LocalIP()
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	msgPeer := Peer{id, localIP, tcpPort, time.Now()}
	jsonMsg, _ := json.Marshal(msgPeer)

	for {
		select {
		case <-time.After(interval):
			conn.WriteTo(jsonMsg, addr)
		}
	}
}

func Receiver(udpPort int, peerUpdateCh chan<- PeerUpdate) {
	var buf [1024]byte
	var p Peer
	var pUpdate PeerUpdate
	pUpdate.TCPconnUpdate = false
	lastSeen := make(map[string]Peer)
	conn := conn.DialBroadcastUDP(udpPort)

	for {
		//fmt.Println("Peers:", lastSeen)
		updated := false

		conn.SetReadDeadline(time.Now().Add(interval))
		n, _, _ := conn.ReadFrom(buf[0:])
		err := json.Unmarshal(buf[:n], &p)
		// Adding new connection, check if new peer
		if err == nil {
			if _, idExists := lastSeen[p.Id]; !idExists {
				updated = true
				p.lastSeen = time.Now()
				lastSeen[p.Id] = p

			} else {
				p.lastSeen = time.Now()
				lastSeen[p.Id] = p

			}
		}
		// Removing dead connection
		for k, v := range lastSeen {
			if time.Since(v.lastSeen) > timeout {
				updated = true
				delete(lastSeen, k)
			}
		}

		// Sending update, send at interval to synchronize UDP and TCP connection loss
		if updated {
			pUpdate.Peers = make([]Peer, 0, len(lastSeen))

			for _, v := range lastSeen {
				pUpdate.Peers = append(pUpdate.Peers, v)
			}
			sort.Slice(pUpdate.Peers, func(i, j int) bool {
				return pUpdate.Peers[i].Id > pUpdate.Peers[j].Id
			})
			fmt.Println("PeerUpdate! Peers: ", pUpdate.Peers)
			peerUpdateCh <- pUpdate
		}
	}

}
