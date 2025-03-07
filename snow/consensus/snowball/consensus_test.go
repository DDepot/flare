// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snowball

import (
	"testing"

	"github.com/flare-foundation/flare/ids"
)

var _ Consensus = &Byzantine{}

// Byzantine is a naive implementation of a multi-choice snowball instance
type Byzantine struct {
	// params contains all the configurations of a snowball instance
	params Parameters

	// Hardcode the preference
	preference ids.ID
}

func (b *Byzantine) Initialize(params Parameters, choice ids.ID) {
	b.params = params
	b.preference = choice
}

func (b *Byzantine) Parameters() Parameters   { return b.params }
func (b *Byzantine) Add(choice ids.ID)        {}
func (b *Byzantine) Preference() ids.ID       { return b.preference }
func (b *Byzantine) RecordPoll(votes ids.Bag) {}
func (b *Byzantine) RecordUnsuccessfulPoll()  {}
func (b *Byzantine) Finalized() bool          { return true }
func (b *Byzantine) String() string           { return b.preference.String() }

var (
	Red   = ids.Empty.Prefix(0)
	Blue  = ids.Empty.Prefix(1)
	Green = ids.Empty.Prefix(2)
)

func ParamsTest(t *testing.T, factory Factory) {
	sb := factory.New()

	params := Parameters{
		K: 2, Alpha: 2, BetaVirtuous: 1, BetaRogue: 2, ConcurrentRepolls: 1,
	}
	sb.Initialize(params, Red)

	p := sb.Parameters()
	switch {
	case p.K != params.K:
		t.Fatalf("Wrong K parameter")
	case p.Alpha != params.Alpha:
		t.Fatalf("Wrong Alpha parameter")
	case p.BetaVirtuous != params.BetaVirtuous:
		t.Fatalf("Wrong Beta1 parameter")
	case p.BetaRogue != params.BetaRogue:
		t.Fatalf("Wrong Beta2 parameter")
	case p.ConcurrentRepolls != params.ConcurrentRepolls:
		t.Fatalf("Wrong Repoll parameter")
	}
}
