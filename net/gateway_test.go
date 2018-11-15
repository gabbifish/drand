package net

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	run "runtime"
	"testing"
	"time"

	"github.com/dedis/drand/protobuf/drand"
	"github.com/kabukky/httpscerts"
	"github.com/stretchr/testify/require"
)

type testPeer struct {
	addr string
	t    bool
}

func (t *testPeer) Address() string {
	return t.addr
}

func (t *testPeer) IsTLS() bool {
	return t.t
}

type testRandomnessServer struct {
	round uint64
}

func (t *testRandomnessServer) Public(context.Context, *drand.PublicRandRequest) (*drand.PublicRandResponse, error) {
	return &drand.PublicRandResponse{Round: t.round}, nil
}
func (t *testRandomnessServer) Private(context.Context, *drand.PrivateRandRequest) (*drand.PrivateRandResponse, error) {
	return &drand.PrivateRandResponse{}, nil
}

func TestListener(t *testing.T) {
	addr1 := "127.0.0.1:4000"
	peer1 := &testPeer{addr1, false}
	addr2 := "127.0.0.1:4100"
	peer2 := &testPeer{addr2, false}
	randServer := &testRandomnessServer{42}

	lis1 := NewTCPGrpcListener(addr1, &DefaultService{R: randServer})
	go lis1.Start()
	defer lis1.Stop()
	time.Sleep(100 * time.Millisecond)

	client := NewGrpcClient()
	resp, err := client.Public(peer1, &drand.PublicRandRequest{})
	require.Nil(t, err)
	expected := &drand.PublicRandResponse{Round: randServer.round}
	require.Equal(t, expected.GetRound(), resp.GetRound())

	rest := NewRestClient()
	resp, err = rest.Public(peer2, &drand.PublicRandRequest{})
	require.NoError(t, err)
	expected = &drand.PublicRandResponse{Round: randServer.round}
	require.Equal(t, expected.GetRound(), resp.GetRound())
}

// ref https://bbengfort.github.io/programmer/2017/03/03/secure-grpc.html
func TestListenerTLS(t *testing.T) {
	if run.GOOS == "windows" {
		fmt.Println("Skipping TestClientTLS as operating on Windows")
		t.Skip("crypto/x509: system root pool is not available on Windows")
	}
	addr1 := "127.0.0.1:4000"
	peer1 := &testPeer{addr1, true}

	tmpDir := path.Join(os.TempDir(), "drand-net")
	require.NoError(t, os.MkdirAll(tmpDir, 0766))
	defer os.RemoveAll(tmpDir)
	certPath := path.Join(tmpDir, "server.crt")
	keyPath := path.Join(tmpDir, "server.key")
	if httpscerts.Check(certPath, keyPath) != nil {
		h, _, _ := net.SplitHostPort(addr1)
		require.NoError(t, httpscerts.Generate(certPath, keyPath, h))
		//require.NoError(t, httpscerts.Generate(certPath, keyPath, addr1))
	}

	randServer := &testRandomnessServer{42}

	lis1, err := NewTLSGrpcListener(addr1, certPath, keyPath, &DefaultService{R: randServer})
	require.NoError(t, err)
	go lis1.Start()
	defer lis1.Stop()
	time.Sleep(100 * time.Millisecond)

	require.Equal(t, peer1.Address(), addr1)
	certManager := NewCertManager()
	certManager.Add(certPath)

	client := NewGrpcClientFromCertManager(certManager)
	resp, err := client.Public(peer1, &drand.PublicRandRequest{})
	require.Nil(t, err)
	expected := &drand.PublicRandResponse{Round: randServer.round}
	require.Equal(t, expected.GetRound(), resp.GetRound())

	rest := NewRestClientFromCertManager(certManager)
	resp, err = rest.Public(peer1, &drand.PublicRandRequest{})
	require.NoError(t, err)
	expected = &drand.PublicRandResponse{Round: randServer.round}
	require.Equal(t, expected.GetRound(), resp.GetRound())
}
