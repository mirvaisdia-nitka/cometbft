package p2p

// func TestHandshake(t *testing.T) {
// 	ln, err := net.Listen("tcp", "127.0.0.1:0")
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	var (
// 		peerPV       = ed25519.GenPrivKey()
// 		peerNodeInfo = testNodeInfo(key.PubKeyToID(peerPV.PubKey()), defaultNodeName)
// 	)

// 	go func() {
// 		c, err := net.Dial(ln.Addr().Network(), ln.Addr().String())
// 		if err != nil {
// 			t.Error(err)
// 			return
// 		}

// 		go func(c net.Conn) {
// 			_, err := protoio.NewDelimitedWriter(c).WriteMsg(peerNodeInfo.(DefaultNodeInfo).ToProto())
// 			if err != nil {
// 				t.Error(err)
// 			}
// 		}(c)
// 		go func(c net.Conn) {
// 			// ni   DefaultNodeInfo
// 			var pbni tmp2p.DefaultNodeInfo

// 			protoReader := protoio.NewDelimitedReader(c, ni.MaxNodeInfoSize())
// 			_, err := protoReader.ReadMsg(&pbni)
// 			if err != nil {
// 				t.Error(err)
// 			}

// 			_, err = ni.DefaultNodeInfoFromToProto(&pbni)
// 			if err != nil {
// 				t.Error(err)
// 			}
// 		}(c)
// 	}()

// 	_, err = ln.Accept()
// 	require.NoError(t, err)

// 	// ni, err := handshake(c, 20*time.Millisecond, emptyNodeInfo())
// 	// if err != nil {
// 	// 	t.Fatal(err)
// 	// }

// 	// if have, want := ni, peerNodeInfo; !reflect.DeepEqual(have, want) {
// 	// 	t.Errorf("have %v, want %v", have, want)
// 	// }
// }

// func TestTransportMultiplexValidateNodeInfo(t *testing.T) {
// 	mt := testSetupMultiplexTransport(t)

// 	errc := make(chan error)

// 	go func() {
// 		var (
// 			pv     = ed25519.GenPrivKey()
// 			dialer = newMultiplexTransport(
// 				nodekey.NodeKey{
// 					PrivKey: pv,
// 				},
// 			)
// 		)

// 		addr := na.NewNetAddress(mt.nodeKey.ID(), mt.listener.Addr())

// 		_, err := dialer.Dial(*addr)
// 		if err != nil {
// 			errc <- err
// 			return
// 		}

// 		close(errc)
// 	}()

// 	if err := <-errc; err != nil {
// 		t.Errorf("connection failed: %v", err)
// 	}

// 	_, _, err := mt.Accept()
// 	if e, ok := err.(ErrRejected); ok {
// 		if !e.IsNodeInfoInvalid() {
// 			t.Errorf("expected NodeInfo to be invalid, got %v", err)
// 		}
// 	} else {
// 		t.Errorf("expected ErrRejected, got %v", err)
// 	}
// }

// func TestTransportMultiplexRejectSelf(t *testing.T) {
// 	mt := testSetupMultiplexTransport(t)

// 	errc := make(chan error)

// 	go func() {
// 		addr := na.NewNetAddress(mt.nodeKey.ID(), mt.listener.Addr())

// 		_, err := mt.Dial(*addr)
// 		if err != nil {
// 			errc <- err
// 			return
// 		}

// 		close(errc)
// 	}()

// 	if err := <-errc; err != nil {
// 		if e, ok := err.(ErrRejected); ok {
// 			if !e.IsSelf() {
// 				t.Errorf("expected to reject self, got: %v", e)
// 			}
// 		} else {
// 			t.Errorf("expected ErrRejected, got %v", err)
// 		}
// 	} else {
// 		t.Errorf("expected connection failure")
// 	}

// 	_, _, err := mt.Accept()
// 	if err, ok := err.(ErrRejected); ok {
// 		if !err.IsSelf() {
// 			t.Errorf("expected to reject self, got: %v", err)
// 		}
// 	} else {
// 		t.Errorf("expected ErrRejected, got %v", nil)
// 	}
// }

// func TestTransportMultiplexRejectMissmatchID(t *testing.T) {
// 	mt := testSetupMultiplexTransport(t)

// 	errc := make(chan error)

// 	go func() {
// 		dialer := newMultiplexTransport(
// 			nodekey.NodeKey{
// 				PrivKey: ed25519.GenPrivKey(),
// 			},
// 		)
// 		addr := na.NewNetAddress(mt.nodeKey.ID(), mt.listener.Addr())

// 		_, err := dialer.Dial(*addr)
// 		if err != nil {
// 			errc <- err
// 			return
// 		}

// 		close(errc)
// 	}()

// 	if err := <-errc; err != nil {
// 		t.Errorf("connection failed: %v", err)
// 	}

// 	_, _, err := mt.Accept()
// 	if e, ok := err.(ErrRejected); ok {
// 		if !e.IsAuthFailure() {
// 			t.Errorf("expected auth failure, got %v", e)
// 		}
// 	} else {
// 		t.Errorf("expected ErrRejected, got %v", err)
// 	}
// }

// func TestTransportMultiplexRejectIncompatible(t *testing.T) {
// 	mt := testSetupMultiplexTransport(t)

// 	errc := make(chan error)

// 	go func() {
// 		var (
// 			pv     = ed25519.GenPrivKey()
// 			dialer = newMultiplexTransport(
// 				nodekey.NodeKey{
// 					PrivKey: pv,
// 				},
// 			)
// 		)
// 		addr := na.NewNetAddress(mt.nodeKey.ID(), mt.listener.Addr())

// 		_, err := dialer.Dial(*addr)
// 		if err != nil {
// 			errc <- err
// 			return
// 		}

// 		close(errc)
// 	}()

// 	_, _, err := mt.Accept()
// 	if e, ok := err.(ErrRejected); ok {
// 		if !e.IsIncompatible() {
// 			t.Errorf("expected to reject incompatible, got %v", e)
// 		}
// 	} else {
// 		t.Errorf("expected ErrRejected, got %v", err)
// 	}
// }
