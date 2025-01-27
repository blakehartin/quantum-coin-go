// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package p2p

import (
	"errors"
	"github.com/QuantumCoinProject/qc/crypto/cryptobase"
	"github.com/QuantumCoinProject/qc/crypto/signaturealgorithm"

	"github.com/QuantumCoinProject/qc/p2p/rlpx"
	"io"
	"math/rand"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/QuantumCoinProject/qc/internal/testlog"
	"github.com/QuantumCoinProject/qc/log"
	"github.com/QuantumCoinProject/qc/p2p/enode"
)

type testTransport struct {
	*rlpxTransport
	rpub     *signaturealgorithm.PublicKey
	closeErr error
}

func newTestTransport(rpub *signaturealgorithm.PublicKey, fd net.Conn, dialDest *signaturealgorithm.PublicKey) transport {
	wrapped := newRLPX(fd, dialDest, "test").(*rlpxTransport)

	dummyData := make([]byte, 32)
	secret, err := rlpx.NewSessionSecret(dummyData, dummyData)
	if err != nil {
		return nil
	}
	secret.CreateApplicationSecrets(dummyData)
	wrapped.conn.InitWithSecrets(*secret)

	return &testTransport{rpub: rpub, rlpxTransport: wrapped}
}

func (c *testTransport) doEncHandshake(prv *signaturealgorithm.PrivateKey) (*signaturealgorithm.PublicKey, error) {
	return c.rpub, nil
}

func (c *testTransport) doProtoHandshake(our *protoHandshake) (*protoHandshake, error) {

	pubkey, err := cryptobase.SigAlg.SerializePublicKey(c.rpub)
	if err != nil {
		return nil, err
	}
	return &protoHandshake{ID: pubkey, Name: "test"}, nil
}

func (c *testTransport) close(err error) {
	c.conn.Close()
	c.closeErr = err
}

func startTestServer(t *testing.T, remoteKey *signaturealgorithm.PublicKey, pf func(*Peer)) *Server {
	config := Config{
		Name:        "test",
		MaxPeers:    10,
		ListenAddr:  "127.0.0.1:0",
		NoDiscovery: true,
		PrivateKey:  newkey(),
		Logger:      testlog.Logger(t, log.LvlTrace),
	}
	server := &Server{
		Config:      config,
		newPeerHook: pf,
		newTransport: func(fd net.Conn, dialDest *signaturealgorithm.PublicKey, context string) transport {
			return newTestTransport(remoteKey, fd, dialDest)
		},
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Could not start server: %v", err)
	}
	return server
}

func TestServerListen(t *testing.T) {
	// start the test server
	connected := make(chan *Peer)
	remid := &newkey().PublicKey
	srv := startTestServer(t, remid, func(p *Peer) {

		if p.ID() != enode.PubkeyToIDV4(remid) {
			t.Error("peer func called with wrong node id")
		}
		connected <- p
	})
	defer close(connected)
	defer srv.Stop()

	// dial the test server
	conn, err := net.DialTimeout("tcp", srv.ListenAddr, 5*time.Second)
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	defer conn.Close()

	select {
	case peer := <-connected:

		if peer.LocalAddr().String() != conn.RemoteAddr().String() {
			t.Errorf("peer started with wrong conn: got %v, want %v",
				peer.LocalAddr(), conn.RemoteAddr())
		}
		peers := srv.Peers()
		if !reflect.DeepEqual(peers, []*Peer{peer}) {
			t.Errorf("Peers mismatch: got %v, want %v", peers, []*Peer{peer})
		}
	case <-time.After(1 * time.Second):
		t.Error("server did not accept within one second")
	}
}

func TestServerPeerLimits(t *testing.T) {
	srvkey := newkey()
	clientkey := newkey()
	clientnode := enode.NewV4(&clientkey.PublicKey, nil, 0)

	pubKey, err := cryptobase.SigAlg.SerializePublicKey(&clientkey.PublicKey)
	if err != nil {
		t.Fatalf("FromOQSPub")
	}
	var tp = &setupTransport{
		pubkey: &clientkey.PublicKey,
		phs: protoHandshake{
			ID: pubKey[:],
			// Force "DiscUselessPeer" due to unmatching caps
			// Caps: []Cap{discard.cap()},
		},
	}

	srv := &Server{
		Config: Config{
			PrivateKey:  srvkey,
			MaxPeers:    0,
			NoDial:      true,
			NoDiscovery: true,
			Protocols:   []Protocol{discard},
			Logger:      testlog.Logger(t, log.LvlTrace),
		},
		newTransport: func(fd net.Conn, dialDest *signaturealgorithm.PublicKey, context string) transport { return tp },
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("couldn't start server: %v", err)
	}
	defer srv.Stop()

	// Check that server is full (MaxPeers=0)
	flags := dynDialedConn
	dialDest := clientnode
	conn, _ := net.Pipe()
	srv.SetupConn(conn, flags, dialDest)
	if tp.closeErr != DiscTooManyPeers {
		t.Errorf("unexpected close error: %q", tp.closeErr)
	}
	conn.Close()

	srv.AddTrustedPeer(clientnode)

	// Check that server allows a trusted peer despite being full.
	conn, _ = net.Pipe()
	srv.SetupConn(conn, flags, dialDest)
	if tp.closeErr == DiscTooManyPeers {
		t.Errorf("failed to bypass MaxPeers with trusted node: %q", tp.closeErr)
	}

	if tp.closeErr != DiscUselessPeer {
		t.Errorf("unexpected close error: %q", tp.closeErr)
	}
	conn.Close()

	srv.RemoveTrustedPeer(clientnode)

	// Check that server is full again.
	conn, _ = net.Pipe()
	srv.SetupConn(conn, flags, dialDest)
	if tp.closeErr != DiscTooManyPeers {
		t.Errorf("unexpected close error: %q", tp.closeErr)
	}
	conn.Close()
}

func TestServerSetupConn(t *testing.T) {
	var (
		clientkey, srvkey = newkey(), newkey()
		clientpub         = &clientkey.PublicKey
		srvpub            = &srvkey.PublicKey
	)

	srvPub, err := cryptobase.SigAlg.SerializePublicKey(srvpub)
	if err != nil {
		t.Fatalf("FromOQSPub")
	}

	cPub, err := cryptobase.SigAlg.SerializePublicKey(clientpub)
	if err != nil {
		t.Fatalf("FromOQSPub")
	}

	tests := []struct {
		dontstart bool
		tt        *setupTransport
		flags     connFlag
		dialDest  *enode.Node

		wantCloseErr error
		wantCalls    string
	}{
		{
			dontstart:    true,
			tt:           &setupTransport{pubkey: clientpub},
			wantCalls:    "close,",
			wantCloseErr: errServerStopped,
		},
		{
			tt:           &setupTransport{pubkey: clientpub, encHandshakeErr: errors.New("read error")},
			flags:        inboundConn,
			wantCalls:    "doEncHandshake,close,",
			wantCloseErr: errors.New("read error"),
		},
		{
			tt:           &setupTransport{pubkey: clientpub, phs: protoHandshake{ID: randomID().Bytes()}},
			dialDest:     enode.NewV4(clientpub, nil, 0),
			flags:        dynDialedConn,
			wantCalls:    "doEncHandshake,doProtoHandshake,close,",
			wantCloseErr: DiscUnexpectedIdentity,
		},
		{
			tt:           &setupTransport{pubkey: clientpub, protoHandshakeErr: errors.New("foo")},
			dialDest:     enode.NewV4(clientpub, nil, 0),
			flags:        dynDialedConn,
			wantCalls:    "doEncHandshake,doProtoHandshake,close,",
			wantCloseErr: errors.New("foo"),
		},
		{
			tt:           &setupTransport{pubkey: srvpub, phs: protoHandshake{ID: srvPub[:]}},
			flags:        inboundConn,
			wantCalls:    "doEncHandshake,close,",
			wantCloseErr: DiscSelf,
		},
		{
			tt:           &setupTransport{pubkey: clientpub, phs: protoHandshake{ID: cPub[:]}},
			flags:        inboundConn,
			wantCalls:    "doEncHandshake,doProtoHandshake,close,",
			wantCloseErr: DiscUselessPeer,
		},
	}

	for i, test := range tests {
		t.Run(test.wantCalls, func(t *testing.T) {
			cfg := Config{
				PrivateKey:  srvkey,
				MaxPeers:    10,
				NoDial:      true,
				NoDiscovery: true,
				Protocols:   []Protocol{discard},
				Logger:      testlog.Logger(t, log.LvlTrace),
			}
			srv := &Server{
				Config:       cfg,
				newTransport: func(fd net.Conn, dialDest *signaturealgorithm.PublicKey, context string) transport { return test.tt },
				log:          cfg.Logger,
			}
			if !test.dontstart {
				if err := srv.Start(); err != nil {
					t.Fatalf("couldn't start server: %v", err)
				}
				defer srv.Stop()
			}
			p1, _ := net.Pipe()
			srv.SetupConn(p1, test.flags, test.dialDest)
			if !reflect.DeepEqual(test.tt.closeErr, test.wantCloseErr) {
				t.Errorf("test %d: close error mismatch: got %q, want %q", i, test.tt.closeErr, test.wantCloseErr)
			}
			if test.tt.calls != test.wantCalls {
				t.Errorf("test %d: calls mismatch: got %q, want %q", i, test.tt.calls, test.wantCalls)
			}
		})
	}
}

type setupTransport struct {
	pubkey            *signaturealgorithm.PublicKey
	encHandshakeErr   error
	phs               protoHandshake
	protoHandshakeErr error

	calls    string
	closeErr error
}

func (c *setupTransport) doEncHandshake(prv *signaturealgorithm.PrivateKey) (*signaturealgorithm.PublicKey, error) {
	c.calls += "doEncHandshake,"
	return c.pubkey, c.encHandshakeErr
}

func (c *setupTransport) doProtoHandshake(our *protoHandshake) (*protoHandshake, error) {

	c.calls += "doProtoHandshake,"
	if c.protoHandshakeErr != nil {
		return nil, c.protoHandshakeErr
	}
	return &c.phs, nil
}
func (c *setupTransport) close(err error) {
	c.calls += "close,"
	c.closeErr = err
}

// setupConn shouldn't write to/read from the connection.
func (c *setupTransport) WriteMsg(Msg) error {
	panic("WriteMsg called on setupTransport")
}
func (c *setupTransport) ReadMsg() (Msg, error) {
	panic("ReadMsg called on setupTransport")
}

func newkey() *signaturealgorithm.PrivateKey {
	key, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		panic("couldn't generate key: " + err.Error())
	}
	return key
}

func randomID() (id enode.ID) {
	for i := range id {
		id[i] = byte(rand.Intn(255))
	}
	return id
}

// This test checks that inbound connections are throttled by IP.
func TestServerInboundThrottle(t *testing.T) {
	const timeout = 10 * time.Second
	newTransportCalled := make(chan struct{})
	srv := &Server{
		Config: Config{
			PrivateKey:  newkey(),
			ListenAddr:  "127.0.0.1:0",
			MaxPeers:    10,
			NoDial:      true,
			NoDiscovery: true,
			Protocols:   []Protocol{discard},
			Logger:      testlog.Logger(t, log.LvlTrace),
		},
		newTransport: func(fd net.Conn, dialDest *signaturealgorithm.PublicKey, context string) transport {
			newTransportCalled <- struct{}{}
			return newRLPX(fd, dialDest, "test")
		},
		listenFunc: func(network, laddr string) (net.Listener, error) {
			fakeAddr := &net.TCPAddr{IP: net.IP{95, 33, 21, 2}, Port: 4444}
			return listenFakeAddr(network, laddr, fakeAddr)
		},
	}
	if err := srv.Start(); err != nil {
		t.Fatal("can't start: ", err)
	}
	defer srv.Stop()

	// Dial the test server.
	conn, err := net.DialTimeout("tcp", srv.ListenAddr, timeout)
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	select {
	case <-newTransportCalled:
		// OK
	case <-time.After(timeout):
		t.Error("newTransport not called")
	}
	conn.Close()

	// Dial again. This time the server should close the connection immediately.
	connClosed := make(chan struct{}, 1)
	conn, err = net.DialTimeout("tcp", srv.ListenAddr, timeout)
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	defer conn.Close()
	go func() {
		conn.SetDeadline(time.Now().Add(timeout))
		buf := make([]byte, 10)
		if n, err := conn.Read(buf); err != io.EOF || n != 0 {
			t.Errorf("expected io.EOF and n == 0, got error %q and n == %d", err, n)
		}
		connClosed <- struct{}{}
	}()
	select {
	case <-connClosed:
		// OK
	case <-newTransportCalled:
		t.Error("newTransport called for second attempt")
	case <-time.After(timeout):
		t.Error("connection not closed within timeout")
	}
}

func listenFakeAddr(network, laddr string, remoteAddr net.Addr) (net.Listener, error) {
	l, err := net.Listen(network, laddr)
	if err == nil {
		l = &fakeAddrListener{l, remoteAddr}
	}
	return l, err
}

// fakeAddrListener is a listener that creates connections with a mocked remote address.
type fakeAddrListener struct {
	net.Listener
	remoteAddr net.Addr
}

type fakeAddrConn struct {
	net.Conn
	remoteAddr net.Addr
}

func (l *fakeAddrListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &fakeAddrConn{c, l.remoteAddr}, nil
}

func (c *fakeAddrConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func syncAddPeer(srv *Server, node *enode.Node) bool {
	var (
		ch      = make(chan *PeerEvent)
		sub     = srv.SubscribeEvents(ch)
		timeout = time.After(2 * time.Second)
	)
	defer sub.Unsubscribe()
	srv.AddPeer(node)
	for {
		select {
		case ev := <-ch:
			if ev.Type == PeerEventTypeAdd && ev.Peer == node.ID() {
				return true
			}
		case <-timeout:
			return false
		}
	}
}
