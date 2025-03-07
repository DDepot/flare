// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"github.com/flare-foundation/flare/ids"
	"github.com/flare-foundation/flare/snow/networking/router"
	"github.com/flare-foundation/flare/version"
)

var _ router.ExternalHandler = &testHandler{}

type testHandler struct {
	router.InboundHandler
	ConnectedF    func(nodeID ids.ShortID, nodeVersion version.Application)
	DisconnectedF func(nodeID ids.ShortID)
}

func (h *testHandler) Connected(id ids.ShortID, nodeVersion version.Application) {
	if h.ConnectedF != nil {
		h.ConnectedF(id, nodeVersion)
	}
}

func (h *testHandler) Disconnected(id ids.ShortID) {
	if h.DisconnectedF != nil {
		h.DisconnectedF(id)
	}
}
