// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"github.com/flare-foundation/flare/ids"
	"github.com/flare-foundation/flare/snow/validation"
)

// A Subnet is a set of validators that are validating a set of blockchains
// Each blockchain is validated by one subnet; one subnet may validate many blockchains
type Subnet interface {
	// ID returns this subnet's ID
	ID() ids.ID

	// Validators returns the validators that compose this subnet
	Validators() []validation.Validator
}
