package beacon

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/dedis/drand/key"
	"github.com/dedis/drand/net"
	"github.com/dedis/drand/protobuf/crypto"
	"github.com/dedis/drand/protobuf/drand"
	"github.com/dedis/drand/test"
	"github.com/nikkolasg/slog"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/kyber/v3/sign/tbls"
	"go.dedis.ch/kyber/v3/util/random"
)

// testBeaconServer implements a barebone service to be plugged in a net.DefaultService
type testBeaconServer struct {
	h *Handler
}

func (t *testBeaconServer) NewBeacon(c context.Context, in *drand.BeaconRequest) (*drand.BeaconResponse, error) {
	return t.h.ProcessBeacon(c, in)
}

func dkgShares(n, t int) ([]*key.Share, kyber.Point) {
	var priPoly *share.PriPoly
	var pubPoly *share.PubPoly
	var err error
	for i := 0; i < n; i++ {
		pri := share.NewPriPoly(key.G2, t, key.G2.Scalar().Pick(random.New()), random.New())
		pub := pri.Commit(key.G2.Point().Base())
		if priPoly == nil {
			priPoly = pri
			pubPoly = pub
			continue
		}
		priPoly, err = priPoly.Add(pri)
		if err != nil {
			panic(err)
		}
		pubPoly, err = pubPoly.Add(pub)
		if err != nil {
			panic(err)
		}
	}
	shares := priPoly.Shares(n)
	secret, err := share.RecoverSecret(key.G2, shares, t, n)
	if err != nil {
		panic(err)
	}
	if !secret.Equal(priPoly.Secret()) {
		panic("secret not equal")
	}
	msg := []byte("Hello world")
	sigs := make([][]byte, n, n)
	_, commits := pubPoly.Info()
	dkgShares := make([]*key.Share, n, n)
	for i := 0; i < n; i++ {
		sigs[i], err = tbls.Sign(key.Pairing, shares[i], msg)
		if err != nil {
			panic(err)
		}
		dkgShares[i] = &key.Share{
			Share:   shares[i],
			Commits: commits,
		}
	}
	sig, err := tbls.Recover(key.Pairing, pubPoly, msg, sigs, t, n)
	if err != nil {
		panic(err)
	}
	if err := bls.Verify(key.Pairing, pubPoly.Commit(), msg, sig); err != nil {
		panic(err)
	}
	//fmt.Println(pubPoly.Commit())
	return dkgShares, pubPoly.Commit()
}

func TestBeacon(t *testing.T) {
	slog.Level = slog.LevelDebug
	n := 5
	thr := 5/2 + 1
	//thr := 5
	// how many generated beacons should we wait from each beacon handler
	nbRound := 3
	dialTimeout := time.Duration(200) * time.Millisecond

	tmp := path.Join(os.TempDir(), "drandtest")
	paths := make([]string, n, n)
	for i := 0; i < n; i++ {
		paths[i] = path.Join(tmp, fmt.Sprintf("drand-%d", i))
		require.NoError(t, os.MkdirAll(paths[i], 0755))
	}
	defer func() {
		for i := 0; i < n; i++ {
			os.RemoveAll(paths[i])
		}
	}()

	shares, public := dkgShares(n, thr)
	privs, group := test.BatchIdentities(n)
	group.Threshold = thr

	listeners := make([]net.Listener, n)
	handlers := make([]*Handler, n)

	seed := []byte("Sunshine in a bottle")
	period := time.Duration(600) * time.Millisecond
	group.Period = period

	// storing beacons from all nodes indexed per round
	genBeacons := make(map[uint64][]*Beacon)
	var l sync.Mutex
	// this is just to signal we received a new beacon
	newBeacon := make(chan int, n*nbRound)
	// launchBeacon will launch the beacon at the given index. Each time a new
	// beacon is ready from that node, it saves the beacon and the node index
	// into the map
	gid, exists := crypto.GroupToID(key.G1)
	require.True(t, exists)
	launchBeacon := func(i int, catchup bool) {
		myCb := func(b *Beacon) {
			err := bls.Verify(key.Pairing, public, Message(b.PreviousRand, b.Round), b.Randomness)
			require.NoError(t, err)
			require.Equal(t, b.Gid, gid)
			l.Lock()
			genBeacons[b.Round] = append(genBeacons[b.Round], b)
			l.Unlock()
			newBeacon <- i
		}
		store, err := NewBoltStore(paths[i], nil)
		require.NoError(t, err)
		store = NewCallbackStore(store, myCb)
		conf := &Config{Group: group, Private: privs[i], Share: shares[i], Seed: seed}
		handlers[i], err = NewHandler(net.NewGrpcClientWithTimeout(dialTimeout), store, conf)
		require.NoError(t, err)
		beaconServer := testBeaconServer{h: handlers[i]}
		listeners[i] = net.NewTCPGrpcListener(privs[i].Public.Addr, &net.DefaultService{B: &beaconServer})
		go listeners[i].Start()
		go handlers[i].Run(period, catchup)
	}

	for i := 0; i < n-1; i++ {
		launchBeacon(i, false)
	}

	defer func() {
		for i := 0; i < n-1; i++ {
			handlers[i].Stop()
			listeners[i].Stop()
		}
	}()

	/* displayInfo := func() {*/
	//l.Lock()
	//defer l.Unlock()
	//for round, beacons := range genBeacons {
	//fmt.Printf("round %d = %d beacons.", round, len(beacons))
	//}
	//fmt.Printf("\n")
	/*}*/
	//expected := nbRound * n
	done := make(chan bool)
	// test how many beacons are generated per each handler, except the last
	// handler that will start later
	countGenBeacons := func(rounds, beaconPerRound int, doneCh chan bool) {
		for {
			<-newBeacon
			l.Lock()
			// do we have enough rounds made
			if len(genBeacons) < rounds {
				l.Unlock()
				continue
			} else {
				// do we have enough beacons for enough rounds
				// we want at least <rounds> rounds with at least
				// <beaconPerRound> beacons in each
				fullRounds := 0
				for _, beacons := range genBeacons {
					if len(beacons) >= beaconPerRound {
						fullRounds++
					}
				}
				if fullRounds < rounds {
					l.Unlock()
					continue
				}
			}
			l.Unlock()
			//displayInfo()
			l.Lock()
			// let's check if they are all equal
			for round, beacons := range genBeacons {
				original := beacons[0]
				for i, beacon := range beacons[1:] {
					if !bytes.Equal(beacon.Randomness, original.Randomness) {
						// randomness is not equal we return false
						l.Unlock()
						fmt.Printf("round %d: original %x vs (%d) %x\n", round, original.Randomness, i+1, beacon.Randomness)
						doneCh <- false
						return
					}
				}
			}
			l.Unlock()
			doneCh <- true
			return
		}
	}
	go countGenBeacons(nbRound, n-1, done)

	checkSuccess := func() {
		select {
		case success := <-done:
			if !success {
				t.Fatal("Not all equal")
			}
			// erase the map
			l.Lock()
			for i := range genBeacons {
				delete(genBeacons, i)
			}
			l.Unlock()
			//case <-time.After(period * time.Duration(nbRound*20)):
			//t.Fatal("not in time")
		}
	}

	checkSuccess()

	// start the last node that needs to catchup
	launchBeacon(n-1, true)
	defer handlers[n-1].Stop()
	defer listeners[n-1].Stop()

	go countGenBeacons(nbRound, n, done)
	checkSuccess()
}

func TestBeaconNEqualT(t *testing.T) {
	slog.Level = slog.LevelDebug
	n := 5
	//thr := 5/2 + 1
	thr := 5
	// how many generated beacons should we wait from each beacon handler
	nbRound := 3
	dialTimeout := time.Duration(200) * time.Millisecond

	tmp := path.Join(os.TempDir(), "drandtest")
	paths := make([]string, n, n)
	for i := 0; i < n; i++ {
		paths[i] = path.Join(tmp, fmt.Sprintf("drand-%d", i))
		require.NoError(t, os.MkdirAll(paths[i], 0755))
	}
	defer func() {
		for i := 0; i < n; i++ {
			os.RemoveAll(paths[i])
		}
	}()

	shares, public := dkgShares(n, thr)
	privs, group := test.BatchIdentities(n)
	group.Threshold = thr

	listeners := make([]net.Listener, n)
	handlers := make([]*Handler, n)

	seed := []byte("Sunshine in a bottle")
	period := time.Duration(1000) * time.Millisecond
	group.Period = period

	// storing beacons from all nodes indexed per round
	genBeacons := make(map[uint64][]*Beacon)
	var l sync.Mutex
	// this is just to signal we received a new beacon
	newBeacon := make(chan int, n*nbRound)
	// launchBeacon will launch the beacon at the given index. Each time a new
	// beacon is ready from that node, it saves the beacon and the node index
	// into the map
	launchBeacon := func(i int, catchup bool) {
		myCb := func(b *Beacon) {
			err := bls.Verify(key.Pairing, public, Message(b.PreviousRand, b.Round), b.Randomness)
			require.NoError(t, err)
			l.Lock()
			genBeacons[b.Round] = append(genBeacons[b.Round], b)
			l.Unlock()
			newBeacon <- i
		}
		store, err := NewBoltStore(paths[i], nil)
		require.NoError(t, err)
		store = NewCallbackStore(store, myCb)
		conf := &Config{Group: group, Private: privs[i], Share: shares[i], Seed: seed}
		handlers[i], err = NewHandler(net.NewGrpcClientWithTimeout(dialTimeout), store, conf)
		require.NoError(t, err)
		beaconServer := testBeaconServer{h: handlers[i]}
		listeners[i] = net.NewTCPGrpcListener(privs[i].Public.Addr, &net.DefaultService{B: &beaconServer})
		go listeners[i].Start()
		go handlers[i].Run(period, catchup)
	}

	for i := 0; i < n; i++ {
		launchBeacon(i, false)
	}

	defer func() {
		for i := 0; i < n; i++ {
			handlers[i].Stop()
			listeners[i].Stop()
		}
	}()

	/* displayInfo := func() {*/
	//l.Lock()
	//defer l.Unlock()
	//for round, beacons := range genBeacons {
	//fmt.Printf("round %d = %d beacons.", round, len(beacons))
	//}
	//fmt.Printf("\n")
	/*}*/
	//expected := nbRound * n
	done := make(chan bool)
	// test how many beacons are generated per each handler, except the last
	// handler that will start later
	countGenBeacons := func(rounds, beaconPerRound int, doneCh chan bool) {
		for {
			<-newBeacon
			l.Lock()
			// do we have enough rounds made
			if len(genBeacons) < rounds {
				l.Unlock()
				continue
			} else {
				// do we have enough beacons for enough rounds
				// we want at least <rounds> rounds with at least
				// <beaconPerRound> beacons in each
				fullRounds := 0
				for _, beacons := range genBeacons {
					if len(beacons) >= beaconPerRound {
						fullRounds++
					}
				}
				if fullRounds < rounds {
					l.Unlock()
					continue
				}
			}
			l.Unlock()
			//displayInfo()
			l.Lock()
			// let's check if they are all equal
			for round, beacons := range genBeacons {
				original := beacons[0]
				for i, beacon := range beacons[1:] {
					if !bytes.Equal(beacon.Randomness, original.Randomness) {
						// randomness is not equal we return false
						l.Unlock()
						fmt.Printf("round %d: original %x vs (%d) %x\n", round, original.Randomness, i+1, beacon.Randomness)
						doneCh <- false
						return
					}
				}
			}
			l.Unlock()
			doneCh <- true
			return
		}
	}
	go countGenBeacons(nbRound, n, done)

	checkSuccess := func() {
		select {
		case success := <-done:
			if !success {
				t.Fatal("Not all equal")
			}
			// erase the map
			l.Lock()
			for i := range genBeacons {
				delete(genBeacons, i)
			}
			l.Unlock()
			//case <-time.After(period * time.Duration(nbRound*20)):
			//t.Fatal("not in time")
		}
	}

	checkSuccess()
}
