// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validators

import (
	"fmt"
	"strings"
	"sync"

	"github.com/flare-foundation/flare/ids"
	"github.com/flare-foundation/flare/utils/constants"
)

// Manager holds the validator set of each subnet
type Manager interface {
	fmt.Stringer

	// GetValidators returns the validator set for the given subnet
	// Returns false if the subnet doesn't exist
	GetValidators() (Set, bool)

	// MaskValidator hides the named validator from future samplings
	MaskValidator(vdrID ids.ShortID) error

	// RevealValidator ensures the named validator is not hidden from future
	// samplings
	RevealValidator(vdrID ids.ShortID) error

	// Contains returns true if there is a validator with the specified ID
	// currently in the set.
	Contains(vdrID ids.ShortID) bool

	// Mutate will mutate the validator set by copying it and adding the given
	// validators.
	Mutate(withs ...With) Manager
}

type With func(Set)

func WithValidator(vdr ids.ShortID, weight uint64) With {
	return func(set Set) {
		_ = set.AddWeight(vdr, weight)
	}
}

// NewManager returns a new, empty manager
func NewManager(networkID uint32, withs ...With) Manager {
	var validators Set
	switch networkID {
	case constants.CostonID:
		validators = loadCostonValidators()
	case constants.SongbirdID:
		validators = loadSongbirdValidators()
	case constants.FlareID:
		validators = loadFlareValidators()
	default:
		validators = loadCustomValidators()
	}
	for _, with := range withs {
		with(validators)
	}
	return &manager{
		networkID:  networkID,
		validators: validators,
	}
}

// manager implements Manager
type manager struct {
	lock       sync.Mutex
	networkID  uint32
	validators Set
	maskedVdrs ids.ShortSet
}

// GetValidatorSet implements the Manager interface.
func (m *manager) GetValidators() (Set, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.validators.Len() == 0 {
		return nil, false
	}
	return m.validators, true
}

// MaskValidator implements the Manager interface.
func (m *manager) MaskValidator(vdrID ids.ShortID) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.maskedVdrs.Contains(vdrID) {
		return nil
	}
	m.maskedVdrs.Add(vdrID)

	if err := m.validators.MaskValidator(vdrID); err != nil {
		return err
	}
	return nil
}

// RevealValidator implements the Manager interface.
func (m *manager) RevealValidator(vdrID ids.ShortID) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if !m.maskedVdrs.Contains(vdrID) {
		return nil
	}
	m.maskedVdrs.Remove(vdrID)

	if err := m.validators.RevealValidator(vdrID); err != nil {
		return err
	}
	return nil
}

// Contains implements the Manager interface.
func (m *manager) Contains(vdrID ids.ShortID) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	return m.validators.Contains(vdrID)
}

// Mutate will copy the underyling validator set and add the given validators.
func (m *manager) Mutate(withs ...With) Manager {
	list := m.validators.List()
	validators := NewSet()
	_ = validators.Set(list)
	for _, with := range withs {
		with(validators)
	}
	return &manager{
		networkID:  m.networkID,
		validators: validators,
	}
}

func (m *manager) String() string {
	m.lock.Lock()
	defer m.lock.Unlock()
	sb := strings.Builder{}

	sb.WriteString(fmt.Sprintf("Validator Set: (Size = %d)",
		m.validators.Len(),
	))
	sb.WriteString(fmt.Sprintf(
		"\n    Network[%d]: %s",
		m.networkID,
		m.validators.PrefixedString("    "),
	))

	return sb.String()
}
