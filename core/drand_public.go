package core

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/dedis/drand/lavarand"

	"github.com/dedis/drand/beacon"
	"github.com/dedis/drand/ecies"
	"github.com/dedis/drand/key"
	"github.com/dedis/drand/protobuf/crypto"
	dkg_proto "github.com/dedis/drand/protobuf/dkg"
	"github.com/dedis/drand/protobuf/drand"
	"github.com/dedis/kyber"
	"github.com/nikkolasg/slog"
)

// Setup is the public method to call during a DKG protocol.
func (d *Drand) Setup(c context.Context, in *dkg_proto.DKGPacket) (*dkg_proto.DKGResponse, error) {
	d.state.Lock()
	defer d.state.Unlock()
	if d.dkgDone {
		return nil, errors.New("drand: dkg finished already")
	}
	if d.dkg == nil {
		return nil, errors.New("drand: no dkg running")
	}
	d.dkg.Process(c, in)
	return &dkg_proto.DKGResponse{}, nil
}

// Reshare is called when a resharing protocol is in progress
func (d *Drand) Reshare(c context.Context, in *dkg_proto.ResharePacket) (*dkg_proto.ReshareResponse, error) {
	d.state.Lock()
	defer d.state.Unlock()

	if d.nextGroupHash == "" {
		return nil, errors.New("drand: can't reshare because InitReshare has not been called")
	}

	// check that we are resharing to the new group that we expect
	if in.GroupHash != d.nextGroupHash {
		return nil, errors.New("drand: can't reshare to new group: incompatible hashes")
	}

	if in.Packet == nil {
		// indicator that we should start the DKG as we are one node in the old
		// list that should reshare its share
		go d.StartDKG()
		return &dkg_proto.ReshareResponse{}, nil
	}

	if d.dkg == nil {
		return nil, errors.New("drand: no dkg setup yet")
	}

	// we just relay to the dkg
	d.dkg.Process(c, in.Packet)
	return &dkg_proto.ReshareResponse{}, nil
}

// NewBeacon methods receives a beacon generation requests and answers
// with the partial signature from this drand node.
func (d *Drand) NewBeacon(c context.Context, in *drand.BeaconRequest) (*drand.BeaconResponse, error) {
	d.state.Lock()
	defer d.state.Unlock()
	if d.beacon == nil {
		return nil, errors.New("drand: beacon not setup yet")
	}
	return d.beacon.ProcessBeacon(c, in)
}

// Public returns a public random beacon according to the request. If the Round
// field is 0, then it returns the last one generated.
func (d *Drand) Public(c context.Context, in *drand.PublicRandRequest) (*drand.PublicRandResponse, error) {
	d.state.Lock()
	defer d.state.Unlock()
	if d.beacon == nil {
		return nil, errors.New("drand: beacon generation not started yet")
	}
	var beacon *beacon.Beacon
	var err error
	if in.GetRound() == 0 {
		beacon, err = d.beaconStore.Last()
	} else {
		beacon, err = d.beaconStore.Get(in.GetRound())
	}
	if err != nil {
		return nil, fmt.Errorf("can't retrieve beacon: %s", err)
	}

	return &drand.PublicRandResponse{
		Previous: beacon.PreviousRand,
		Round:    beacon.Round,
		Randomness: &crypto.Point{
			Point: beacon.Randomness,
			Gid:   crypto.GroupID(beacon.Gid),
		},
	}, nil
}

// Private returns an ECIES encrypted random blob of 32 bytes from /dev/urandom
func (d *Drand) Private(c context.Context, priv *drand.PrivateRandRequest) (*drand.PrivateRandResponse, error) {
	protoPoint := priv.GetRequest().GetEphemeral()
	point, err := crypto.ProtoToKyberPoint(protoPoint)
	if err != nil {
		return nil, err
	}
	groupable, ok := point.(kyber.Groupable)
	if !ok {
		return nil, errors.New("point is not on a registered curve")
	}
	if groupable.Group().String() != key.G2.String() {
		return nil, errors.New("point is not on the supported curve")
	}
	msg, err := ecies.Decrypt(key.G2, ecies.DefaultHash, d.priv.Key, priv.GetRequest())
	if err != nil {
		slog.Debugf("drand: received invalid ECIES private request: %s", err)
		return nil, errors.New("invalid ECIES request")
	}

	clientKey := key.G2.Point()
	if err := clientKey.UnmarshalBinary(msg); err != nil {
		return nil, errors.New("invalid client key")
	}
	//var randomness [32]byte
	// TODO: READ FROM LAVARAND
	randomness, err := lavarand.GetRandom(32)
	if err != nil {
		return nil, err
	}
	if n, err := rand.Read(randomness[:]); err != nil {
		return nil, errors.New("error gathering randomness")
	} else if n != 32 {
		return nil, errors.New("error gathering randomness")
	}

	obj, err := ecies.Encrypt(key.G2, ecies.DefaultHash, clientKey, randomness[:])
	return &drand.PrivateRandResponse{Response: obj}, err
}

// Home ...
func (d *Drand) Home(c context.Context, in *drand.HomeRequest) (*drand.HomeResponse, error) {
	slog.Infof("drand: home method requested")
	return &drand.HomeResponse{
		Status: fmt.Sprintf("drand up and running on %s",
			d.priv.Public.Address()),
	}, nil
}
