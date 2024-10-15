package mock

import (
	"net"

	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/libs/service"
	"github.com/cometbft/cometbft/p2p"
	na "github.com/cometbft/cometbft/p2p/netaddress"
	ni "github.com/cometbft/cometbft/p2p/nodeinfo"
	"github.com/cometbft/cometbft/p2p/transport/tcp/conn"
)

type Peer struct {
	*service.BaseService
	ip                   net.IP
	id                   nodekey.ID
	addr                 *na.NetAddress
	kv                   map[string]any
	Outbound, Persistent bool
}

// NewPeer creates and starts a new mock peer. If the ip
// is nil, random routable address is used.
func NewPeer(ip net.IP) *Peer {
	var netAddr *na.NetAddress
	if ip == nil {
		_, netAddr = na.CreateRoutableAddr()
	} else {
		netAddr = na.NewNetAddressIPPort(ip, 26656)
	}
	nodeKey := nodekey.NodeKey{PrivKey: ed25519.GenPrivKey()}
	netAddr.ID = nodenodekey.ID()
	mp := &Peer{
		ip:   ip,
		id:   nodenodekey.ID(),
		addr: netAddr,
		kv:   make(map[string]any),
	}
	mp.BaseService = service.NewBaseService(nil, "MockPeer", mp)
	if err := mp.Start(); err != nil {
		panic(err)
	}
	return mp
}

func (mp *Peer) FlushStop()               { mp.Stop() } //nolint:errcheck //ignore error
func (*Peer) HasChannel(_ byte) bool      { return true }
func (*Peer) TrySend(_ p2p.Envelope) bool { return true }
func (*Peer) Send(_ p2p.Envelope) bool    { return true }
func (mp *Peer) NodeInfo() ni.NodeInfo {
	return ni.DefaultNodeInfo{
		DefaultNodeID: mp.addr.ID,
		ListenAddr:    mp.addr.DialString(),
	}
}
func (*Peer) Status() conn.ConnectionStatus { return conn.ConnectionStatus{} }
func (mp *Peer) ID() nodekey.ID             { return mp.id }
func (mp *Peer) IsOutbound() bool           { return mp.Outbound }
func (mp *Peer) IsPersistent() bool         { return mp.Persistent }
func (mp *Peer) Get(key string) any {
	if value, ok := mp.kv[key]; ok {
		return value
	}
	return nil
}

func (mp *Peer) Set(key string, value any) {
	mp.kv[key] = value
}
func (mp *Peer) RemoteIP() net.IP           { return mp.ip }
func (mp *Peer) SocketAddr() *na.NetAddress { return mp.addr }
func (mp *Peer) RemoteAddr() net.Addr       { return &net.TCPAddr{IP: mp.ip, Port: 8800} }
func (*Peer) CloseConn() error              { return nil }
func (*Peer) SetRemovalFailed()             {}
func (*Peer) GetRemovalFailed() bool        { return false }
