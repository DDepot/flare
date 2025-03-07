// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throttling

import (
	"github.com/flare-foundation/flare/ids"
)

var _ InboundMsgThrottler = &noInboundMsgThrottler{}

// Returns an InboundMsgThrottler where Acquire() always returns immediately.
func NewNoInboundThrottler() InboundMsgThrottler {
	return &noInboundMsgThrottler{}
}

// [Acquire] always returns immediately.
type noInboundMsgThrottler struct{}

func (*noInboundMsgThrottler) Acquire(uint64, ids.ShortID) {}

func (*noInboundMsgThrottler) Release(uint64, ids.ShortID) {}

func (*noInboundMsgThrottler) AddNode(ids.ShortID) {}

func (*noInboundMsgThrottler) RemoveNode(ids.ShortID) {}
