// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snowman

import (
	"sort"
	"time"

	"github.com/flare-foundation/flare/ids"
	"github.com/flare-foundation/flare/snow/choices"
)

var _ Block = &TestBlock{}

// TestBlock is a useful test block
type TestBlock struct {
	choices.TestDecidable

	ParentV    ids.ID
	HeightV    uint64
	TimestampV time.Time
	VerifyV    error
	BytesV     []byte
}

func (b *TestBlock) Parent() ids.ID       { return b.ParentV }
func (b *TestBlock) Height() uint64       { return b.HeightV }
func (b *TestBlock) Timestamp() time.Time { return b.TimestampV }
func (b *TestBlock) Verify() error        { return b.VerifyV }
func (b *TestBlock) Bytes() []byte        { return b.BytesV }

type sortBlocks []*TestBlock

func (sb sortBlocks) Less(i, j int) bool { return sb[i].HeightV < sb[j].HeightV }
func (sb sortBlocks) Len() int           { return len(sb) }
func (sb sortBlocks) Swap(i, j int)      { sb[j], sb[i] = sb[i], sb[j] }

// SortTestBlocks sorts the array of blocks by height
func SortTestBlocks(blocks []*TestBlock) { sort.Sort(sortBlocks(blocks)) }
