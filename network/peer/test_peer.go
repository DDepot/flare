// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package peer

import (
	"context"
	"crypto"
	"net"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/flare-foundation/flare/ids"
	"github.com/flare-foundation/flare/message"
	"github.com/flare-foundation/flare/network/throttling"
	"github.com/flare-foundation/flare/snow/networking/router"
	"github.com/flare-foundation/flare/snow/validation"
	"github.com/flare-foundation/flare/staking"
	"github.com/flare-foundation/flare/utils"
	"github.com/flare-foundation/flare/utils/constants"
	"github.com/flare-foundation/flare/utils/logging"
	"github.com/flare-foundation/flare/version"
)

// StartTestPeer provides a simple interface to create a peer that has finished
// the p2p handshake.
//
// This function will generate a new TLS key to use when connecting to the peer.
//
// The returned peer will not throttle inbound or outbound messages.
//
// - [ctx] provides a way of canceling the connection request.
// - [ip] is the remote that will be dialed to create the connection.
// - [networkID] will be sent to the peer during the handshake. If the peer is
//   expecting a different [networkID], the handshake will fail and an error
//   will be returned.
// - [router] will be called with all non-handshake messages received by the
//   peer.
func StartTestPeer(
	ctx context.Context,
	ip utils.IPDesc,
	networkID uint32,
	router router.InboundHandler,
) (Peer, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, constants.NetworkType, ip.String())
	if err != nil {
		return nil, err
	}

	tlsCert, err := staking.NewTLSCert()
	if err != nil {
		return nil, err
	}

	tlsConfg := TLSConfig(*tlsCert)
	clientUpgrader := NewTLSClientUpgrader(tlsConfg)

	peerID, conn, cert, err := clientUpgrader.Upgrade(conn)
	if err != nil {
		return nil, err
	}

	mc, err := message.NewCreator(
		prometheus.NewRegistry(),
		true,
		"",
		10*time.Second,
	)
	if err != nil {
		return nil, err
	}

	metrics, err := NewMetrics(
		logging.NoLog{},
		"",
		prometheus.NewRegistry(),
	)
	if err != nil {
		return nil, err
	}

	ipDesc := utils.IPDesc{
		IP:   net.IPv6zero,
		Port: 0,
	}
	peer := Start(
		&Config{
			Metrics:              metrics,
			MessageCreator:       mc,
			Log:                  logging.NoLog{},
			InboundMsgThrottler:  throttling.NewNoInboundThrottler(),
			OutboundMsgThrottler: throttling.NewNoOutboundThrottler(),
			Network: NewTestNetwork(
				mc,
				networkID,
				ipDesc,
				version.CurrentApp,
				tlsCert.PrivateKey.(crypto.Signer),
				ids.Set{},
				100,
			),
			Router:               router,
			VersionCompatibility: version.GetCompatibility(networkID),
			VersionParser:        version.NewDefaultApplicationParser(),
			MySubnets:            ids.Set{},
			Beacons:              validation.NewSet(),
			NetworkID:            networkID,
			PingFrequency:        constants.DefaultPingFrequency,
			PongTimeout:          constants.DefaultPingPongTimeout,
			MaxClockDifference:   time.Minute,
		},
		conn,
		cert,
		peerID,
	)
	return peer, peer.AwaitReady(ctx)
}
