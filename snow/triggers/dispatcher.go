// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package triggers

import (
	"fmt"
	"sync"

	"github.com/flare-foundation/flare/ids"
	"github.com/flare-foundation/flare/snow"
	"github.com/flare-foundation/flare/utils/logging"
)

var _ snow.EventDispatcher = &EventDispatcher{}

type handler struct {
	// Must implement at least one of Acceptor, Rejector, Issuer
	handlerFunc interface{}
	// If true and [handlerFunc] returns an error during a call to Accept,
	// the chain this handler corresponds to will stop.
	dieOnError bool
}

// EventDispatcher receives events from consensus and dispatches the events to triggers
type EventDispatcher struct {
	lock sync.Mutex
	log  logging.Logger
	// Chain ID --> Identifier --> handler
	chainHandlers map[ids.ID]map[string]handler
}

func New(log logging.Logger) *EventDispatcher {
	return &EventDispatcher{
		log:           log,
		chainHandlers: make(map[ids.ID]map[string]handler),
	}
}

// Accept is called when a transaction or block is accepted.
// If the returned error is non-nil, the chain associated with [ctx] should shut
// down and not commit [container] or any other container to its database as accepted.
func (ed *EventDispatcher) Accept(ctx *snow.ConsensusContext, containerID ids.ID, container []byte) error {
	ed.lock.Lock()
	defer ed.lock.Unlock()

	events, exist := ed.chainHandlers[ctx.ChainID]
	if !exist {
		return nil
	}
	for id, handler := range events {
		handlerFunc, ok := handler.handlerFunc.(snow.Acceptor)
		if !ok {
			continue
		}

		if err := handlerFunc.Accept(ctx, containerID, container); err != nil {
			ed.log.Error("handler %s on chain %s errored while accepting %s: %s", id, ctx.ChainID, containerID, err)
			if handler.dieOnError {
				return fmt.Errorf("handler %s on chain %s errored while accepting %s: %w", id, ctx.ChainID, containerID, err)
			}
		}
	}
	return nil
}

// Reject is called when a transaction or block is rejected
func (ed *EventDispatcher) Reject(ctx *snow.ConsensusContext, containerID ids.ID, container []byte) error {
	ed.lock.Lock()
	defer ed.lock.Unlock()

	events, exist := ed.chainHandlers[ctx.ChainID]
	if !exist {
		return nil
	}
	for id, handler := range events {
		handler, ok := handler.handlerFunc.(snow.Rejector)
		if !ok {
			continue
		}

		if err := handler.Reject(ctx, containerID, container); err != nil {
			ed.log.Error("unable to Reject on %s for chainID %s: %s", id, ctx.ChainID, err)
		}
	}
	return nil
}

// Issue is called when a transaction or block is issued
func (ed *EventDispatcher) Issue(ctx *snow.ConsensusContext, containerID ids.ID, container []byte) error {
	ed.lock.Lock()
	defer ed.lock.Unlock()

	events, exist := ed.chainHandlers[ctx.ChainID]
	if !exist {
		return nil
	}
	for id, handler := range events {
		handler, ok := handler.handlerFunc.(snow.Issuer)
		if !ok {
			continue
		}

		if err := handler.Issue(ctx, containerID, container); err != nil {
			ed.log.Error("unable to Issue on %s for chainID %s: %s", id, ctx.ChainID, err)
		}
	}
	return nil
}

// RegisterChain causes [handlerFunc] to be invoked every time a container is issued, accepted or rejected on chain [chainID].
// [handlerFunc] should implement at least one of Acceptor, Rejector, Issuer.
// If [dieOnError], chain [chainID] stops if [handler].Accept is invoked and returns a non-nil error.
func (ed *EventDispatcher) RegisterChain(chainID ids.ID, identifier string, handlerFunc interface{}, dieOnError bool) error {
	ed.lock.Lock()
	defer ed.lock.Unlock()

	events, exist := ed.chainHandlers[chainID]
	if !exist {
		events = make(map[string]handler)
		ed.chainHandlers[chainID] = events
	}

	if _, ok := events[identifier]; ok {
		return fmt.Errorf("handler %s already exists on chain %s", identifier, chainID)
	}

	events[identifier] = handler{
		handlerFunc: handlerFunc,
		dieOnError:  dieOnError,
	}
	return nil
}

// DeregisterChain removes a chain handler from the system
func (ed *EventDispatcher) DeregisterChain(chainID ids.ID, identifier string) error {
	ed.lock.Lock()
	defer ed.lock.Unlock()

	events, exist := ed.chainHandlers[chainID]
	if !exist {
		return fmt.Errorf("chain %s has no handlers", chainID)
	}

	if _, ok := events[identifier]; !ok {
		return fmt.Errorf("handler %s does not exist on chain %s", identifier, chainID)
	}

	if len(events) == 1 {
		delete(ed.chainHandlers, chainID)
	} else {
		delete(events, identifier)
	}
	return nil
}
