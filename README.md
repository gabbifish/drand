[![Build Status](https://travis-ci.org/dedis/drand.svg?branch=master)](https://travis-ci.org/dedis/drand)

# Drand - A Distributed Randomness Beacon Daemon

Drand (pronounced "dee-rand") is a distributed randomness beacon daemon written
in [Golang](https://golang.org/). Servers running drand can be linked with each
other to produce collective, publicly verifiable, unbiasable, unpredictable
random values at fixed intervals using bilinear pairings and threshold cryptography.
Drand nodes can also serve locally-generated private randomness to clients.

### Disclaimer

**This software is considered experimental and has NOT received a third-party
audit yet. Therefore, DO NOT USE it in production or for anything security
critical at this point.**

## Quickstart

To run drand locally make sure that you have a working
[Docker installation](https://docs.docker.com/engine/installation/).
Then execute (might need root privileges to run Docker on some systems):
```bash
git clone https://github.com/dedis/drand
cd drand
./run_local.sh
```

The script spins up six local drand nodes using Docker and produces fresh
randomness every few seconds.

## Overview

### Public Randomness

Generating public randomness is the primary functionality of drand. Public
randomness is generated collectively by drand nodes and publicly available. The
main challenge in generating good randomness is that no party involved in the
randomness generation process should be able to predict or bias the final
output. Additionally, the final result has to be third-party verifiable to make
it actually useful for applications like lotteries, sharding, or parameter
generation in security protocols.  

A drand randomness beacon is composed of a distributed set of nodes and has two
phases:

- **Setup:** Each node first generates a *long-term public/private key pair*.
    Then all of the public keys are written to a *group file* together with some
    further metadata required to operate the beacon. After this group file has
    been distributed, the nodes perform a *distributed key generation* (DKG) protocol
    to create the collective public key and one private key share per server. The
    participants NEVER see/use the actual (distributed) private key explicitly but
    instead utilize their respective private key shares for the generation of public
    randomness.

- **Generation:** After the setup, the nodes switch to the randomness
    generation mode. Any of the nodes can initiate a randomness generation round
    by broadcasting a message which all the other participants sign using a t-of-n
    threshold version of the *Boneh-Lynn-Shacham* (BLS) signature scheme and their
    respective private key shares. Once any node (or third-party observer) has
    gathered t partial signatures, it can reconstruct the full BLS
    signature (using Lagrange interpolation) which corresponds to the collective
    random value. This random beacon / full BLS signature can be verified against
    the collective public key.

### Private Randomness

Private randomness generation is the secondary functionality of drand. Clients
can request private randomness from some or all of the drand nodes which extract
it locally from their entropy pools and send it back in encrypted form. This
can be useful to gather randomness from different entropy sources, for example
in embedded devices.

In this mode we assume that a client has a private/public key pair and
encapsulates its public key towards the server's public key using the ECIES
encryption scheme. After receiving a request, the drand node produces 32 random
bytes locally (using Go's `crypto/rand` interface), encrypts them using the
received public key and sends it back to the client.

**Note:** Assuming that clients without good local entropy sources (such
as embedded devices) use this process to gather high entropy randomness to
bootstrap their local PRNGs, we emphasize that the initial client key pair has
to be provided by a trusted source (such as the device manufacturer). Otherwise
we run into the chicken-and-egg problem of how to produce on the client's side a
secure ephemeral key pair for ECIES encryption without a good (local) source of
randomness.

## Installation

Drand can be installed via [Golang](https://golang.org/) or
[Docker](https://www.docker.com/). By default, drand saves the configuration
files such as the long-term key pair, the group file, and the collective public
key in the directory `$HOME/.drand/`.

### Via Golang

Make sure that you have a working [Golang
installation](https://golang.org/doc/install) and that your
[GOPATH](https://golang.org/doc/code.html#GOPATH) is set.  
Then install drand via:
```bash
go get -u github.com/dedis/drand
```
### Via Docker

Make sure that you have a working [Docker installation](https://docs.docker.com/engine/installation/).

## Usage

This section explains in details the workflow to have a working group of drand
nodes generate randomness. On a high-level, the workflow looks like this:
+ **Setup**: generation of individual longterm key-pair and the group file and
  starting the drand daemon.
+ **Distributed Key Generation**: each drand node collectively participates in
  the DKG.
+ **Randomness Generation**: the randomness beacon automatically starts as soon as
the DKG protocol is finished.

### Setup

The setup process for a drand node consists of two steps:
1. Generate the long-term key pair for each node
2. Setup the group configuration file

#### Long-Term Key

To generate the long-term key pair `drand_id.{secret,public}` of the drand daemon, execute
```
drand generate-keypair <address>
```
where `<address>` is the address from which your drand daemon is reachable. The
address must be reachable over a TLS connection. In case you need non-secured
channel, you can pass the `--tls-disable` flag.

#### Group Configuration

All informations regarding a group of drand nodes necessary for drand to function properly are located inside a group.toml configuration file. To run a DKG protocol, one needs to generate this group configuration file from all individual longterm keys generated in the previous step. One can do so with:
```
drand group <pk1> <pk2> ... <pkn>
```
where `<pki>` is the public key file `drand_id.public` of the i-th participant.
The group file is generated in the current directory under `group.toml`.
**NOTE:** At this stage, this group file MUST be distributed to all participants !

##### Randomness Beacon Period

drand updates the configuration file after the DKG protocol finishes,
with the distributed public key and automatically starts running the randomness
beacon. By default, a randomness beacon has a period of 1mn,i.e. new randomness
is generated every minute. If you wish to change the period, you must include
that information **inside** the group configuration file. You can do by
appending a flag to the command such as :
```
drand group --period 2m <pk1> <pk2> ... <pkn>
```

Or simply by editing manually the group file afterwards: it's a TOML
configuration file.
The period must be readable by the [time](https://golang.org/pkg/time/#ParseDuration) package.

### Starting drand daemon

The daemon does not go automatically in background, so you must run it with ` &
` in your terminal, within a screen / tmux session, or with the `-d` option
enabled for the docker commands. Once the daemon is running, the way to issue
commands to the daemon is to use the control functionalities.  The control
client has to run on the same server as the drand daemon, so only drand
administrators can issue command to their drand daemons.

There are two ways to run a drand daemon: using TLS or using plain old regular
un-encrypted connections. Drand by default tries to use TLS connections.

#### With TLS

Drand nodes attempt to communicate by default over TLS-protected connections.
Therefore, you need to point your node to the TLS certificate chain and
corresponding private key you wish to use via:
```bash
drand start \
    --tls-cert <fullchain.pem> \
    --tls-key <privkey.pem>
```

To get TLS certificates for free you can use, for example, [Let's
Encrypt](https://letsencrypt.org/) with its official CLI tool [EFF's
certbot](https://certbot.eff.org/).

#### Without TLS

Although we **do not recommend** it, you can always disable TLS in drand via:
```bash
drand start --tls-disable
```

#### With Docker
If you run drand in Docker, **always** use the following template

```bash
docker run \
    --rm \
    --name drand \
    -p <port>:<port> \
    --volume $HOME/.drand/:/root/.drand/ \
    dedis/drand <command>
```

where `<port>` specifies the port through which your drand daemon is reachable
and `<command>` has to be substituted by one of the respective drand
commands below. You must add the corresponding volumes pointing to your TLS
private key and certificate in case you are using TLS (recommended).

### Distributed Key Generation

After running all drand daemons, each operator needs to issue a command to start
the DKG protocol, using the group file generated before. One can do so using the
control client with:
```
drand share <group-file>  --timeout 10s
```

One of the nodes has to function as the leader to initiate the DKG protocol (no
additional trust assumptions), he can do so with:
```
drand share --leader <group-file> 
```

Once running, the leader initiates the distributed key generation protocol to
compute the distributed public key (`dist_key.public`) and the private key
shares (`dist_key.private`) together with the participants specified in
`drand_group.toml`. Once the DKG has finished, the keys are stored as
`$HOME/.drand/groups/dist_key.{public,private}`.

The timeout is an optional parameter indicating the maximum timeout the DKG
protocol will wait. If there are some failed nodes during the DKG, then the DKG will finish only after the given timeout. The default value is set to 10s (see
[`core/constants.go`](https://github.com/dedis/drand/blob/master/core/constants.go) file).

**Group File**: Once the DKG phase is done, the group file is updated with the
newly created distributed public key. That updated group file needed by drand to
securely contact drand nodes on their public interface to gather private or
public randomness. A drand administrator can get the updated group file  it via
the following:
```bash
drand show group
```
It will print the group file in its regular TOML format. If you want to save it to
a file, append the `--out <file>` flag.

**Distributed Public Key**: More generally, for third party implementation of
randomness beacon verification, one only needs the distributed public key. If
you are an administrator of a drand node, you can use the control port as the
following:
```bash
drand show cokey
```

Otherwise, you can contact an external drand node to ask him for its current
distributed public key:
```bash
drand get cokey --nodes <address> <group.toml>
```
where `<group.toml>` is the group file identity file of a drand node. You can
use the flag `--nodes <address(es)>` to indicate which node you want to contact
specifically (it is a whitespace-separated list).
Use the`--tls-cert` flag to specify the server's certificate if
needed. The group toml does not need to be updated with the collective key.

**NOTE**: Using the last method (`get cokey`), a drand node *can* lie about the
key if no out-of-band verification is performed. That information is usually
best gathered from a trusted drand operator and then embedded in any
applications using drand.

### Randomness Generation

After a successful setup, drand switches automatically to the randomness
generation mode, where each node broadcasts randomness shares at regular
intervals. Once a node has collected a threshold of shares in the current phase,
it computes the public random value and stores it in its local instance of
[BoltDB](https://github.com/coreos/bbolt).

The default interval is one minute. If you wish to change that, you need to
do so while generating the group file before the DKG.

### Control Functionalities

Drand's local administrator interface provides further functionality, e.g., to
update group details or retrieve secret information. By default, the daemon
listens on `127.0.0.1:8888`, but you can specify another control port when starting
the daemon with:
```bash
drand start --control 1234
```
In that case, you need to specify the control port for each of the following
commands.

#### Long-Term Private Key

To retrieve the long-term private key of our node, run:
```bash
drand show private
```

#### Long-Term Public Key

To retrieve the long-term public key of our node, run:
```bash
drand show public
```

#### Private Key Share

To retrieve the private key share of our node, as determined during the DKG, run the following command:
```bash
drand show share
```
The JSON-formatted output has the following form:
```json
{
  "index" : 1,
  "share" : {
    "gid": 22,
    "scalar": "764f6e3eecdc4aba8b2f0119e7b2fd8c35948bf2be3f87ebb5823150c6065764"
  }
}
```

The "gid" simply indicates which group the data belongs to. It is present for
scalar and points on the curve, even though scalars are the same on the three
groups of BN256. The field is present already to be able to accommodate
different curves later on.


#### Distributed Key

To retrieve the collective key of the drand beacon our node is involved in, run:
```bash
drand show cokey
```

### Using Drand

A drand beacon provides several public services to clients. A drand node exposes
its public services on a gRPC endpoint as well as a REST JSON endpoint, on the
same port. The latter is especially useful if one wishes to retrieve randomness
from a Javascript application.  Communication is protected through TLS by
default. If the contacted node is using a self-signed certificate, the client
can use the `--tls-cert` flag to specify the server's certificate.

### Fetching Public Randomness

To get the latest public random value, run
```bash
drand get public --round <i> <group.toml>
```
where `<group.toml>` is the group identity file of a drand node. You can specify
the round number when the public randomness has been generated. If not
specified, this command returns the most recent random beacon.

The JSON-formatted output produced by drand is of the following form:
```json
{
    "round": 2,
    "previous": "5e59b03c65a82c9f2be39a7fd23e8e8249fd356c4fd7d146700fc428ac80ec3f7a2
d8a74d4d3b3664a90409f7ec575f7211f06502001561b00e036d0fbd42d2b",
    "randomness": {
        "gid": 21,
        "point": "357562670af7e67f3534f5a5a6e01269f3f9e86a7b833591b0ec2a51faa7c11111
2a1dc1baea73926c1822bc5135469cc1c304adc6ccc942dac7c3a52977a342"
    }
}
```

Here `randomness` is the latest random value, which is a threshold BLS signature
on the previous random value `Previous` and the round number. The field `Round`
specifies the index of `Randomness` in the sequence of all random values
produced by this drand instance. The **message signed** is therefore the
concatenation of the round number treated as a `uint64` and the previous
randomness.The gid is an indicator of the group this point belongs to. At the
moment, we are only using BLS signatures on the BN256 curves and the signature
is made over G1.

#### Fetching Private Randomness

To get a private random value, run the following:

```bash
drand get private group.toml
```
The JSON-formatted output produced by drand should look like the following:

```bash
{
    "Randomness": "764f6e3eecdc4aba8b2f0119e7b2fd8c35948bf2be3f87ebb5823150c6065764"
}
```

The command outputs a 32-byte hex-encoded random value generated from the local
randomness engine of the contacted server. If the encryption is not correct, the
command outputs an error instead.

#### Using HTTP endpoints

One may want get the distributed key or public randomness by issuing a GET to a
HTTP endpoint instead of using a gRPC client. Here is a basic example on how to
do so with curl.

To get the distributed key, you can use:
```bash
curl <address>/api/info/distkey
```

Similarly, to get the latest round of randomness from the drand beacon, you can use
```bash
curl <address>/api/public
```

**All the REST endpoints are specified in the `protobuf/drand/client.proto` file.**

**NOTE**: At the moment, the REST endpoints return base-64 encoded values, whereas
the drand cli tool returns hexadecimal encoded value ([issue](https://github.com/dedis/drand/issues/85)).


### Updating Drand Group

Drand allows for "semi-dynamic" group update with a *resharing* protocol that
offers the following:
+ new nodes can join an existing group and get new shares. Note that, in fact,
  all nodes get *new* shares after running the resharing protocol.
+ nodes can leave their current group. It may be necessary for nodes that do not
  wish to operate drand anymore.
+ nodes can update the threshold associated with their current distributed
  public key.

The main advantage of this method is that the distributed public key stays the
*same* even with new nodes coming in. That can be useful when the distributed
public key is embedded inside the application using drand, and hence is
difficult to update.

Updating is simple in drand, it uses the same command as for the DKG:
```bash
drand share --from old-group.toml new-group.toml
```
for new nodes joining the system. The old group toml is fetched as shown above,
and the new group toml is created the usual way (`drand group ....`).

For nodes already in current the group, there is actually a shortcut (the
previous command works also) where there is no need to specify the old group:
```bash
drand share <newGroup.toml>
```

As usual, a leader must start the protocol by indicating the `--leader` flag.

After the protocol is finished, each node listed in the new-group.toml file,
will have a new share corresponding to the same distributed public key. The
randomness generation starts immediately after the resharing protocol using the
new shares.

Here `rnd` is the 32-byte base64-encoded private random value produced by the
contacted drand node. If the encryption is not correct, the command outputs an
error instead.


## Cryptography Background

You can learn more about drand, its motivations and how does it work on these
public [slides](https://docs.google.com/presentation/d/1t2ysit78w0lsySwVbQOyWcSDnYxdOBPzY7K2P9UE1Ac/edit?usp=sharing).

Drand relies on the following cryptographic constructions:

- [Pairing-based cryptography](https://en.wikipedia.org/wiki/Pairing-based_cryptography) and [Barreto-Naehrig curves](https://github.com/dfinity/bn).
- [Pedersen's distributed key generation protocol](https://link.springer.com/article/10.1007/s00145-006-0347-3) for the setup.
- Threshold [BLS signatures](https://www.iacr.org/archive/asiacrypt2001/22480516.pdf) for the generation of public randomness.
- [ECIES](https://en.wikipedia.org/wiki/Integrated_Encryption_Scheme) for the encryption of private randomness.

For our previous work on public randomness, see our academical paper
[Scalable Bias-Resistant Distributed Randomness](https://eprint.iacr.org/2016/1067.pdf).

## DrandJS

To facilitate the use of drand's randomness in JavaScript-based applications, we provide
[DrandJS](https://github.com/PizzaWhisperer/drandjs). The main method `fetchAndVerify`
of this JavaScript library fetches from a drand node the latest random beacon generated and then
verifies it against the distributed key.
For more details on the procedure and instructions on how to use it,
refer to the [readme](https://github.com/PizzaWhisperer/drandjs/blob/master/README.md).
As it is compiled from Go, DrandJS stays experimental and is used as proof-of-concept.
Our longterm objective is to have a library written in pure JavaScript.

## What's Next?

Although being already functional, drand is still at an early development stage
and there is a lot left to be done. The list of opened
[issues](https://github.com/dedis/drand/issues) is a good place to start. On top
of this, drand would benefit from higher-level enhancements such as the
following: 

+ Add more unit tests
+ Reduce size of Docker
+ Add a systemd unit file
+ Support multiple drand instances within one node
+ Implement a more [failure-resilient DKG protocol](https://eprint.iacr.org/2012/377.pdf)

Feel free to submit feature requests or, even better, pull requests. ;)

## License

The drand source code is released under MIT license, see the file
[LICENSE](https://github.com/dedis/drand/blob/master/LICENSE) for the full text.

## Contributors

Here's the list of people that contributed to drand:

- Nicolas Gailly ([@nikkolasg1](https://twitter.com/nikkolasg1))
- Philipp Jovanovic ([@daeinar](https://twitter.com/daeinar))
- Mathilde Raynal ([@PizzaWhisperer](https://github.com/PizzaWhisperer))
- Gabbi Fisher ([@gabbifish](https://github.com/gabbifish))
- Linus Gasser ([@ineiti](https://github.com/ineiti))
- Jeff Allen ([@jeffallen](https://github.com/jeffallen))

## Acknowledgments

Thanks to [@herumi](https://github.com/herumi) for providing support on his
optimized pairing-based cryptographic library used in the first version.

Thanks to Apostol Vassilev for its interest in drand and the extensive and
helpful discussions on the drand design.

Thanks to [@Bren2010](https://github.com/Bren2010) and
[@grittygrease](https://github.com/grittygrease) for providing the native Golang
bn256 implementation and for their help in the design of drand and future ideas.
