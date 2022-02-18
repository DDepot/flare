#Flare

Node implementation for the [Flare](https://flare.network) network.

## Installation

Flare uses a relatively lightweight consensus protocol, so the minimum computer requirements are modest.
Note that as network usage increases, hardware requirements may change.

The minimum recommended hardware specification for nodes connected to Mainnet is:

- CPU: Equivalent of 8 AWS vCPU
- RAM: 16 GiB
- Storage: 512 GiB
- OS: Ubuntu 18.04/20.04 or macOS >= 10.15 (Catalina)
- Network: Reliable IPv4 or IPv6 network connection, with an open public port.

If you plan to build AvalancheGo from source, you will also need the following software:

- [Go](https://golang.org/doc/install) version >= 1.16.8
- [gcc](https://gcc.gnu.org/)
- g++

### Native Install

Clone the Flare repository:

```sh
git clone git@github.com:flare-foundation/flare.git
cd avalanchego
```

This will clone and checkout to `master` branch.

#### Building the Avalanche Executable

Build Avalanche using the build script:

```sh
./scripts/build.sh
```

The Flare binary, named `flare`, is in the `build` directory.

## Running Flare

### Connecting to Songbird

To connect to the Songbird canary network, run:

```sh
export FBA_VALs=./scripts/configs/songbird/validators.json
./build/flare --network-id=songbird \
  --bootstrap-ips="$(curl -m 10 -sX POST --data '{ "jsonrpc":"2.0", "id":1, "method":"info.getNodeIP" }' -H 'content-type:application/json;' https://songbird.flare.network/ext/info | jq -r ".result.ip")" \
  --bootstrap-ids="$(curl -m 10 -sX POST --data '{ "jsonrpc":"2.0", "id":1, "method":"info.getNodeID" }' -H 'content-type:application/json;' https://songbird.flare.network/ext/info | jq -r ".result.nodeID")"
```

You should see some _fire_ ASCII art and log messages.

You can use `Ctrl+C` to kill the node.

Please note that you currently need to be whitelisted to connect to the beacon nodes.

### Pruning & APIs

The configuration for the chain is loaded from a configuration file, located at `{chain-config-dir}/C/config.json`:

```json
{
  "snowman-api-enabled": false,
  "coreth-admin-api-enabled": false,
  "net-api-enabled": true,
  "eth-api-enabled": true,
  "personal-api-enabled": false,
  "tx-pool-api-enabled": true,
  "debug-api-enabled": true,
  "web3-api-enabled": true,
  "local-txs-enabled": true,
  "pruning-enabled": false
}
```

The directory for configuration files defaults to `HOME/.flare/configs` and can be changed using the `--chain-config-dir` flag.

In order to disable pruning and run a full archival node, `pruning-enabled` should be set to `false`.

The various node APIs can also be enabled and disabled by setting the respective parameters.

### Launching Flare locally

To create a single node local test network, run:

```sh
./build/flare --network-id=local \
  --staking-enabled=false \
  --snow-sample-size=1 \
  --snow-quorum-size=1
```

This launches a Flare network with one node.

## Generating Code

Flare uses multiple tools to generate efficient and boilerplate code.

### Running protobuf codegen

To regenerate the protobuf go code, run `scripts/protobuf_codegen.sh` from the root of the repo.

This should only be necessary when upgrading protobuf versions or modifying .proto definition files.

To use this script, you must have [buf](https://docs.buf.build/installation) (v1.0.0-rc12), protoc-gen-go (v1.27.1) and protoc-gen-go-grpc (v1.2.0) installed.

To install the buf dependencies:

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.27.1
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2.0
```

If you have not already, you may need to add `$GOPATH/bin` to your `$PATH`:

```sh
export PATH="$PATH:$(go env GOPATH)/bin"
```

If you extract buf to ~/software/buf/bin, the following should work:

```sh
export PATH=$PATH:~/software/buf/bin/:~/go/bin
go get google.golang.org/protobuf/cmd/protoc-gen-go
go get google.golang.org/protobuf/cmd/protoc-gen-go-grpc
scripts/protobuf_codegen.sh
```

For more information, refer to the [GRPC Golang Quick Start Guide](https://grpc.io/docs/languages/go/quickstart/).        |

## Security Bugs

**We and our community welcome responsible disclosures.**

If you've discovered a security vulnerability, please report it via our [contact form](https://flare.network/contact/). Valid reports will be eligible for a reward (terms and conditions apply).
