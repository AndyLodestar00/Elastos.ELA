package p2p

import (
	"fmt"
	"github.com/elastos/Elastos.ELA/crypto"
	"io"
	math "math/rand"
	"net"
	"testing"
	"time"

	"github.com/elastos/Elastos.ELA/dpos/p2p/msg"
	"github.com/elastos/Elastos.ELA/dpos/p2p/peer"

	"github.com/elastos/Elastos.ELA/p2p"
	"github.com/stretchr/testify/assert"
)

type message struct {
	pid peer.PID
}

func (msg *message) CMD() string {
	return "message"
}

func (msg *message) MaxLength() uint32 {
	return 1024
}

func (msg *message) Serialize(w io.Writer) error {
	_, err := w.Write(msg.pid[:])
	return err
}

func (msg *message) Deserialize(r io.Reader) error {
	_, err := io.ReadFull(r, msg.pid[:])
	return err
}

func mockRemotePeer(pid peer.PID, priKey []byte, port uint16,
	pc chan<- *peer.Peer, mc chan<- p2p.Message) error {

	// Configure peer to act as a simnet node that offers no services.
	cfg := &peer.Config{
		PID:             pid,
		Magic:           123123,
		ProtocolVersion: 0,
		Services:        0,
		PingInterval:    defaultPingInterval,
		SignNonce: func(nonce []byte) (signature [64]byte) {
			sign, _ := crypto.Sign(priKey, nonce)
			copy(signature[:], sign)
			return signature
		},
		PingNonce:        func(pid peer.PID) uint64 { return 0 },
		PongNonce:        func(pid peer.PID) uint64 { return 0 },
		MakeEmptyMessage: makeEmptyMessage,
	}

	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	go func() {
		for {
			conn, err := listen.Accept()
			if err != nil {
				fmt.Printf("%s can not accept, %s", listen.Addr(), err)
				return
			}

			p := peer.NewInboundPeer(cfg)
			p.AssociateConnection(conn)

			p.AddMessageFunc(func(peer *peer.Peer, m p2p.Message) {
				switch m := m.(type) {
				case *msg.VerAck:
					pc <- p

				case *message:
					mc <- m
				}
			})

			go func() {
				p.WaitForDisconnect()
				pc <- p
			}()
		}
	}()
	return nil
}

func mockInboundPeer(addr PeerAddr, priKey []byte, pc chan<- *peer.Peer,
	mc chan<- p2p.Message) error {
	// Configure peer to act as a simnet node that offers no services.
	cfg := &peer.Config{
		PID:             addr.PID,
		Magic:           123123,
		ProtocolVersion: 0,
		Services:        0,
		PingInterval:    defaultPingInterval,
		SignNonce: func(nonce []byte) (signature [64]byte) {
			sign, _ := crypto.Sign(priKey, nonce)
			copy(signature[:], sign)
			return signature
		},
		PingNonce:        func(pid peer.PID) uint64 { return 0 },
		PongNonce:        func(pid peer.PID) uint64 { return 0 },
		MakeEmptyMessage: makeEmptyMessage,
	}

	conn, err := net.Dial("tcp", "localhost:20338")
	if err != nil {
		return err
	}
	p, err := peer.NewOutboundPeer(cfg, addr.Addr)
	if err != nil {
		return err
	}
	p.AssociateConnection(conn)
	p.AddMessageFunc(func(peer *peer.Peer, m p2p.Message) {
		switch m := m.(type) {
		case *msg.VerAck:
			pc <- p

		case *message:
			mc <- m
		}
	})

	go func() {
		p.WaitForDisconnect()
		pc <- p
	}()
	return nil
}

// Test multiple servers connect to each other.
func TestServerConnections(t *testing.T) {
	// Mock 71 server config and addresses.  Why 71 ? because 71[servers]*
	// ((71-1)*2)[inbound and outbound connections] = 9940, and terminal ulimit
	// parameter is 10240, so 71 is the maximum servers can mock on my computer.
	servers := 71
	cfgs := make([]Config, 0, servers)
	addrList := make([]PeerAddr, 0, servers)
	pid := peer.PID{}
	for i := 0; i < servers; i++ {
		priKey, pubKey, _ := crypto.GenerateKeyPair()
		ePubKey, _ := pubKey.EncodePoint(true)
		copy(pid[:], ePubKey)

		port := 40000 + i

		cfgs = append(cfgs, Config{
			PID:             pid,
			MagicNumber:     123123,
			ProtocolVersion: 0,
			Services:        0,
			DefaultPort:     uint16(port),
			SignNonce: func(nonce []byte) (signature [64]byte) {
				sign, _ := crypto.Sign(priKey, nonce)
				copy(signature[:], sign)
				return signature
			},
			MakeEmptyMessage: makeEmptyMessage,
		})

		addrList = append(addrList, PeerAddr{
			PID:  pid,
			Addr: fmt.Sprintf("localhost:%d", port),
		})
	}

	// Start 71 servers.
	serverChan := make(chan *server, servers)
	doneChan := make(chan struct{})
	for _, cfg := range cfgs {
		s, err := NewServer(&cfg)
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		s.Start()
		s.ConnectPeers(addrList)
		serverChan <- s

		// There will be 70 outbound connections and 70 inbound connections
		// for each server
		go func() {
			ticker := time.NewTicker(time.Millisecond * 100)
			defer ticker.Stop()

		out:
			for {
				select {
				case <-ticker.C:
					connected := s.ConnectedCount()
					if connected >= int32(servers-1)*2 {
						break out
					}
				case <-time.After(time.Second * 10):
					t.Fatal("Server connection timeout")
				}
			}
			// Notify server connect peers completed.
			doneChan <- struct{}{}
		}()
	}
	for i := 0; i < servers; i++ {
		select {
		case <-doneChan:
		case <-time.After(time.Second * 10):
			t.Fatal("Server connect to peers timeout")
		}
	}

cleanup:
	for {
		select {
		case s := <-serverChan:
			s.Stop()
		default:
			break cleanup
		}
	}
}

func TestServer_ConnectPeers(t *testing.T) {
	// Start peer-to-peer server
	pid := peer.PID{}
	priKey, pubKey, _ := crypto.GenerateKeyPair()
	ePubKey, _ := pubKey.EncodePoint(true)
	copy(pid[:], ePubKey)
	server, err := NewServer(&Config{
		PID:             pid,
		MagicNumber:     123123,
		ProtocolVersion: 0,
		Services:        0,
		DefaultPort:     20338,
		SignNonce: func(nonce []byte) (signature [64]byte) {
			sign, _ := crypto.Sign(priKey, nonce)
			copy(signature[:], sign)
			return signature
		},
		MakeEmptyMessage: makeEmptyMessage,
	})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer server.Stop()
	server.Start()

	peerChan := make(chan *peer.Peer)
	msgChan := make(chan p2p.Message)

	// Mock 100 remote peers and addresses.
	portBase := uint16(50000)
	addrList := make([]PeerAddr, 0, 100)
	priKeys := make([][]byte, 0, 100)
	connectPeers := make(map[peer.PID]PeerAddr)
	for i := uint16(0); i < 100; i++ {
		priKey, pubKey, _ := crypto.GenerateKeyPair()
		priKeys = append(priKeys, priKey)
		ePubKey, _ := pubKey.EncodePoint(true)
		copy(pid[:], ePubKey)
		port := portBase + i
		addr := PeerAddr{PID: pid, Addr: fmt.Sprintf("localhost:%d", port)}
		addrList = append(addrList, addr)
		connectPeers[pid] = addr
		err := mockRemotePeer(pid, priKey, port, peerChan, msgChan)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
	}

	// Connect 50 peers
	server.ConnectPeers(addrList[:50])
	for i := 0; i < 50; i++ {
		select {
		case <-peerChan:
		case <-time.After(time.Second):
			t.Fatalf("Connect peers timeout")
		}
	}

	connectedPeers := server.ConnectedPeers()
	if !assert.Equal(t, 50, len(connectedPeers)) {
		t.FailNow()
	}

	for _, p := range connectedPeers {
		index := p.ToPeer().NA().Port % portBase
		if !p.PID().Equal(addrList[index].PID) {
			t.Errorf("Connect peer PID not match, expect %s get %s",
				addrList[index].PID, p.PID())
		}
	}
	for i := 0; i < 100; i++ {
		index := math.Intn(100)
		pid := addrList[index].PID
		err := server.SendMessageToPeer(pid, &message{pid: pid})
		if index < 50 {
			assert.NoError(t, err)
			<-msgChan
		} else {
			if !assert.Equal(t, err, ErrSendMessageFailed) {
				t.Errorf("Send message to %d>50 succeed", index)
			}
		}
	}

	// Change connect peers
	server.ConnectPeers(addrList[50:])
	for i := 0; i < 100; i++ { // 50 disconnect peers, 50 connected peers.
		select {
		case <-peerChan:
		case <-time.After(time.Second):
			t.Fatalf("Connect peers timeout")
		}
	}

	connectedPeers = server.ConnectedPeers()
	if !assert.Equal(t, 50, len(connectedPeers)) {
		t.FailNow()
	}
	for _, p := range connectedPeers {
		index := p.ToPeer().NA().Port % portBase
		if !p.PID().Equal(addrList[index].PID) {
			t.Errorf("Connect peer PID not match, expect %s got %s",
				addrList[index].PID, p.PID())
		}
	}

	for i := 0; i < 100; i++ {
		index := math.Intn(100)
		pid := addrList[index].PID
		err := server.SendMessageToPeer(pid, &message{pid: pid})
		if index >= 50 {
			assert.NoError(t, err)
			<-msgChan
		} else {
			if !assert.Equal(t, err, ErrSendMessageFailed) {
				t.Errorf("Send message to %d<50 succeed", index)
			}
		}
	}

	// Connect all peers
	server.ConnectPeers(addrList)
	for i := 0; i < 50; i++ { // 50 new peers will connected.
		select {
		case <-peerChan:
		case <-time.After(time.Second):
			t.Fatalf("Connect peers timeout")
		}
	}

	connectedPeers = server.ConnectedPeers()
	if !assert.Equal(t, 100, len(connectedPeers)) {
		t.FailNow()
	}
	for _, p := range connectedPeers {
		index := p.ToPeer().NA().Port % portBase
		if !p.PID().Equal(addrList[index].PID) {
			t.Errorf("Connect peer PID not match, expect %s got %s",
				addrList[index].PID, p.PID())
		}
	}

	// Mock 50 inbound peers with PID in connect list.
	for i, addr := range addrList[:50] {
		err := mockInboundPeer(addr, priKeys[:50][i], peerChan, msgChan)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
	}
	for i := 0; i < 50; i++ {
		select {
		case <-peerChan:
		case <-time.After(time.Second):
			t.Fatalf("Connect peers timeout")
		}
	}

	connectedPeers = server.ConnectedPeers()
	if !assert.Equal(t, 150, len(connectedPeers)) {
		t.FailNow()
	}

	connectedPIDs := make(map[peer.PID]struct{})
	for _, p := range connectedPeers {
		_, ok := connectPeers[p.PID()]
		if !assert.Equal(t, true, ok) {
			t.Errorf("Connect peer PID %s not in addr list", p.PID())
			t.FailNow()
		}
		connectedPIDs[p.PID()] = struct{}{}
	}
	if !assert.Equal(t, 100, len(connectedPIDs)) {
		t.FailNow()
	}

	// Now there are 50 inbound peers are the same PID with outbound peers,
	// only one of the peer with the PID will receive message.
	for _, addr := range addrList[:50] {
		err := server.SendMessageToPeer(addr.PID, &message{addr.PID})
		if !assert.NoError(t, err) {
			t.FailNow()
		}
	}
	for i := 0; i < 50; i++ { // 50 received messages.
		select {
		case <-msgChan:
		case <-time.After(time.Second):
			t.Fatalf("Receive message timeout")
		}
	}

	// Change connect peers, there are 50 inbound peers with the same PID as
	// 50 outbound peers, so 100 peers will be disconnected.
	server.ConnectPeers(addrList[50:])
	for i := 0; i < 100; i++ {
		select {
		case <-peerChan:
		case <-time.After(time.Second):
			t.Fatalf("Connect peers timeout")
		}
	}

	connectedPeers = server.ConnectedPeers()
	if !assert.Equal(t, 50, len(connectedPeers)) {
		t.FailNow()
	}
}

// The peers in connect list should be reconnect when happens to disconnected.
func TestServer_PeersReconnect(t *testing.T) {
	// Start peer-to-peer server
	pid := peer.PID{}
	priKey, pubKey, _ := crypto.GenerateKeyPair()
	ePubKey, _ := pubKey.EncodePoint(true)
	copy(pid[:], ePubKey)
	server, err := NewServer(&Config{
		PID:             pid,
		MagicNumber:     123123,
		ProtocolVersion: 0,
		Services:        0,
		DefaultPort:     20338,
		SignNonce: func(nonce []byte) (signature [64]byte) {
			sign, _ := crypto.Sign(priKey, nonce)
			copy(signature[:], sign)
			return signature
		},
		MakeEmptyMessage: makeEmptyMessage,
	})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer server.Stop()
	server.Start()

	peerChan := make(chan *peer.Peer)
	msgChan := make(chan p2p.Message)

	// Mock 100 remote peers and addresses.
	addrList := make([]PeerAddr, 0, 100)
	connectPeers := make(map[peer.PID]PeerAddr)
	for i := uint16(0); i < 100; i++ {
		priKey, pubKey, _ := crypto.GenerateKeyPair()
		ePubKey, _ := pubKey.EncodePoint(true)
		copy(pid[:], ePubKey)
		port := 60000 + i
		addr := PeerAddr{PID: pid, Addr: fmt.Sprintf("localhost:%d", port)}
		addrList = append(addrList, addr)
		connectPeers[pid] = addr
		err := mockRemotePeer(pid, priKey, port, peerChan, msgChan)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
	}

	// Connect peers and disconnect them to mock unstable connection.
	server.ConnectPeers(addrList)
	for i := 0; i < 1000; i++ {
		select {
		case p := <-peerChan:
			if i < 900 { // Disconnect peers for 900 times.
				p.Disconnect()
			}

		case <-time.After(time.Minute):
			t.Fatalf("Connect peers timeout")
		}
	}
}

func TestServer_BroadcastMessage(t *testing.T) {
	// Start peer-to-peer server
	pid := peer.PID{}
	priKey, pubKey, _ := crypto.GenerateKeyPair()
	ePubKey, _ := pubKey.EncodePoint(true)
	copy(pid[:], ePubKey)
	server, err := NewServer(&Config{
		PID:             pid,
		MagicNumber:     123123,
		ProtocolVersion: 0,
		Services:        0,
		DefaultPort:     20338,
		SignNonce: func(nonce []byte) (signature [64]byte) {
			sign, _ := crypto.Sign(priKey, nonce)
			copy(signature[:], sign)
			return signature
		},
		MakeEmptyMessage: makeEmptyMessage,
	})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer server.Stop()
	server.Start()

	peerChan := make(chan *peer.Peer)
	msgChan := make(chan p2p.Message)

	// Mock 100 remote peers and addresses.
	priKeys := make([][]byte, 0, 100)
	addrList := make([]PeerAddr, 0, 100)
	connectPeers := make(map[peer.PID]PeerAddr)
	for i := uint16(0); i < 100; i++ {
		priKey, pubKey, _ := crypto.GenerateKeyPair()
		priKeys = append(priKeys, priKey)
		ePubKey, _ := pubKey.EncodePoint(true)
		copy(pid[:], ePubKey)
		port := 40000 + i
		addr := PeerAddr{PID: pid, Addr: fmt.Sprintf("localhost:%d", port)}
		addrList = append(addrList, addr)
		connectPeers[pid] = addr
		err := mockRemotePeer(pid, priKey, port, peerChan, msgChan)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
	}

	// Mock 100 outbound peers.
	server.ConnectPeers(addrList)
	for i := 0; i < 100; i++ {
		select {
		case <-peerChan:
		case <-time.After(time.Second):
			t.Fatalf("Connect peers timeout")
		}
	}
	if !assert.Equal(t, int32(100), server.ConnectedCount()) {
		t.FailNow()
	}

	// Mock 100 inbound peers.
	for i, addr := range addrList {
		err := mockInboundPeer(addr, priKeys[i], peerChan, msgChan)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
	}
	for i := 0; i < 100; i++ {
		select {
		case <-peerChan:
		case <-time.After(time.Second):
			t.Fatalf("Connect peers timeout")
		}
	}
	if !assert.Equal(t, int32(200), server.ConnectedCount()) {
		t.FailNow()
	}

	// Each PID should receive only one message.
	server.BroadcastMessage(&message{pid: pid})
	count := 0
out:
	for {
		select {
		case <-msgChan:
			count++
		case <-time.After(time.Second):
			break out
		}
	}
	if !assert.Equal(t, 100, count) {
		t.FailNow()
	}
}

func makeEmptyMessage(cmd string) (m p2p.Message, e error) {
	switch cmd {
	case "message":
		m = &message{}
	}
	return m, nil
}
