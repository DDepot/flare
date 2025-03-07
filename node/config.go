// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package node

import (
	"crypto/tls"
	"time"

	"github.com/flare-foundation/flare/chains"
	"github.com/flare-foundation/flare/genesis"
	"github.com/flare-foundation/flare/ids"
	"github.com/flare-foundation/flare/nat"
	"github.com/flare-foundation/flare/network"
	"github.com/flare-foundation/flare/snow/consensus/avalanche"
	"github.com/flare-foundation/flare/snow/networking/benchlist"
	"github.com/flare-foundation/flare/snow/networking/router"
	"github.com/flare-foundation/flare/snow/networking/sender"
	"github.com/flare-foundation/flare/utils"
	"github.com/flare-foundation/flare/utils/dynamicip"
	"github.com/flare-foundation/flare/utils/logging"
	"github.com/flare-foundation/flare/utils/profiler"
	"github.com/flare-foundation/flare/utils/timer"
	"github.com/flare-foundation/flare/vms"
)

type IPCConfig struct {
	IPCAPIEnabled      bool     `json:"ipcAPIEnabled"`
	IPCPath            string   `json:"ipcPath"`
	IPCDefaultChainIDs []string `json:"ipcDefaultChainIDs"`
}

type APIAuthConfig struct {
	APIRequireAuthToken bool   `json:"apiRequireAuthToken"`
	APIAuthPassword     string `json:"-"`
}

type APIIndexerConfig struct {
	IndexAPIEnabled      bool `json:"indexAPIEnabled"`
	IndexAllowIncomplete bool `json:"indexAllowIncomplete"`
}

type HTTPConfig struct {
	APIConfig `json:"apiConfig"`
	HTTPHost  string `json:"httpHost"`
	HTTPPort  uint16 `json:"httpPort"`

	HTTPSEnabled bool   `json:"httpsEnabled"`
	HTTPSKey     []byte `json:"-"`
	HTTPSCert    []byte `json:"-"`

	APIAllowedOrigins []string `json:"apiAllowedOrigins"`

	ShutdownTimeout time.Duration `json:"shutdownTimeout"`
	ShutdownWait    time.Duration `json:"shutdownWait"`
}

type APIConfig struct {
	APIAuthConfig    `json:"authConfig"`
	APIIndexerConfig `json:"indexerConfig"`
	IPCConfig        `json:"ipcConfig"`

	// Enable/Disable APIs
	AdminAPIEnabled    bool `json:"adminAPIEnabled"`
	InfoAPIEnabled     bool `json:"infoAPIEnabled"`
	KeystoreAPIEnabled bool `json:"keystoreAPIEnabled"`
	MetricsAPIEnabled  bool `json:"metricsAPIEnabled"`
	HealthAPIEnabled   bool `json:"healthAPIEnabled"`
}

type IPConfig struct {
	IP utils.DynamicIPDesc `json:"ip"`
	// True if we attempted NAT Traversal
	AttemptedNATTraversal bool `json:"attemptedNATTraversal"`
	// Tries to perform network address translation
	Nat nat.Router `json:"-"`
	// Dynamic Update duration for IP or NAT traversal
	DynamicUpdateDuration time.Duration `json:"dynamicUpdateDuration"`
	// Tries to resolve our IP from an external source
	DynamicPublicIPResolver dynamicip.Resolver `json:"-"`
}

type StakingConfig struct {
	genesis.StakingConfig
	EnableStaking         bool            `json:"enableStaking"`
	StakingTLSCert        tls.Certificate `json:"-"`
	DisabledStakingWeight uint64          `json:"disabledStakingWeight"`
	StakingKeyPath        string          `json:"stakingKeyPath"`
	StakingCertPath       string          `json:"stakingCertPath"`
}

type BootstrapConfig struct {
	// Should Bootstrap be retried
	RetryBootstrap bool `json:"retryBootstrap"`

	// Max number of times to retry bootstrap before warning the node operator
	RetryBootstrapWarnFrequency int `json:"retryBootstrapWarnFrequency"`

	// Timeout when connecting to bootstrapping beacons
	BootstrapBeaconConnectionTimeout time.Duration `json:"bootstrapBeaconConnectionTimeout"`

	// Max number of containers in an ancestors message sent by this node.
	BootstrapAncestorsMaxContainersSent int `json:"bootstrapAncestorsMaxContainersSent"`

	// This node will only consider the first [AncestorsMaxContainersReceived]
	// containers in an ancestors message it receives.
	BootstrapAncestorsMaxContainersReceived int `json:"bootstrapAncestorsMaxContainersReceived"`

	// Max time to spend fetching a container and its
	// ancestors while responding to a GetAncestors message
	BootstrapMaxTimeGetAncestors time.Duration `json:"bootstrapMaxTimeGetAncestors"`

	BootstrapIDs []ids.ShortID  `json:"bootstrapIDs"`
	BootstrapIPs []utils.IPDesc `json:"bootstrapIPs"`
}

type DatabaseConfig struct {
	// Path to database
	Path string `json:"path"`

	// Name of the database type to use
	Name string `json:"name"`

	// Path to config file
	Config []byte `json:"-"`
}

// Config contains all of the configurations of an Avalanche node.
type Config struct {
	HTTPConfig          `json:"httpConfig"`
	IPConfig            `json:"ipConfig"`
	StakingConfig       `json:"stakingConfig"`
	genesis.TxFeeConfig `json:"txFeeConfig"`
	BootstrapConfig     `json:"bootstrapConfig"`
	DatabaseConfig      `json:"databaseConfig"`

	// Genesis information
	GenesisBytes []byte `json:"-"`
	AvaxAssetID  ids.ID `json:"avaxAssetID"`

	// ID of the network this node should connect to
	NetworkID uint32 `json:"networkID"`

	// Assertions configuration
	EnableAssertions bool `json:"enableAssertions"`

	// Crypto configuration
	EnableCrypto bool `json:"enableCrypto"`

	// Health
	HealthCheckFreq time.Duration `json:"healthCheckFreq"`

	// Network configuration
	NetworkConfig network.Config `json:"networkConfig"`

	GossipConfig sender.GossipConfig `json:"gossipConfig"`

	AdaptiveTimeoutConfig timer.AdaptiveTimeoutConfig `json:"adaptiveTimeoutConfig"`

	// Benchlist Configuration
	BenchlistConfig benchlist.Config `json:"benchlistConfig"`

	// Profiling configurations
	ProfilerConfig profiler.Config `json:"profilerConfig"`

	// Logging configuration
	LoggingConfig logging.Config `json:"loggingConfig"`

	// Plugin directory
	PluginDir string `json:"pluginDir"`

	// File Descriptor Limit
	FdLimit uint64 `json:"fdLimit"`

	// Consensus configuration
	ConsensusParams avalanche.Parameters `json:"consensusParams"`

	// Metrics
	MeterVMEnabled bool `json:"meterVMEnabled"`

	// Router that is used to handle incoming consensus messages
	ConsensusRouter          router.Router       `json:"-"`
	RouterHealthConfig       router.HealthConfig `json:"routerHealthConfig"`
	ConsensusShutdownTimeout time.Duration       `json:"consensusShutdownTimeout"`
	// Gossip a container in the accepted frontier every [ConsensusGossipFrequency]
	ConsensusGossipFrequency time.Duration `json:"consensusGossipFreq"`

	// Subnet Whitelist
	WhitelistedSubnets ids.Set `json:"whitelistedSubnets"`

	// SubnetConfigs
	SubnetConfigs map[ids.ID]chains.SubnetConfig `json:"subnetConfigs"`

	// ChainConfigs
	ChainConfigs map[string]chains.ChainConfig `json:"-"`

	// VM management
	VMManager vms.Manager `json:"-"`

	// Reset proposerVM height index
	ResetProposerVMHeightIndex bool `json:"resetProposerVMHeightIndex"`
}
