// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package poll

import (
	"testing"

	"github.com/flare-foundation/flare/ids"
)

func TestNoEarlyTermResults(t *testing.T) {
	vtxID := ids.ID{1}
	votes := []ids.ID{vtxID}

	validator1 := ids.ShortID{1} // k = 1

	validators := ids.ShortBag{}
	validators.Add(validator1)

	factory := NewNoEarlyTermFactory()
	poll := factory.New(validators)

	poll.Vote(validator1, votes)
	if !poll.Finished() {
		t.Fatalf("Poll did not terminate after receiving k votes")
	}

	result := poll.Result()
	if list := result.List(); len(list) != 1 {
		t.Fatalf("Wrong number of vertices returned")
	} else if retVtxID := list[0]; retVtxID != vtxID {
		t.Fatalf("Wrong vertex returned")
	} else if set := result.GetSet(vtxID); set.Len() != 1 {
		t.Fatalf("Wrong number of votes returned")
	}
}

func TestNoEarlyTermString(t *testing.T) {
	vtxID := ids.ID{1}
	votes := []ids.ID{vtxID}

	validator1 := ids.ShortID{1}
	validator2 := ids.ShortID{2} // k = 2

	validators := ids.ShortBag{}
	validators.Add(
		validator1,
		validator2,
	)

	factory := NewNoEarlyTermFactory()
	poll := factory.New(validators)

	poll.Vote(validator1, votes)

	expected := `waiting on Bag: (Size = 1)
    ID[BaMPFdqMUQ46BV8iRcwbVfsam55kMqcp]: Count = 1
received UniqueBag: (Size = 1)
    ID[SYXsAycDPUu4z2ZksJD5fh5nTDcH3vCFHnpcVye5XuJ2jArg]: Members = 0000000000000002`
	if result := poll.String(); expected != result {
		t.Fatalf("Poll should have returned %s but returned %s", expected, result)
	}
}

func TestNoEarlyTermDropsDuplicatedVotes(t *testing.T) {
	vtxID := ids.ID{1}
	votes := []ids.ID{vtxID}

	validator1 := ids.ShortID{1}
	validator2 := ids.ShortID{2} // k = 2

	validators := ids.ShortBag{}
	validators.Add(
		validator1,
		validator2,
	)

	factory := NewNoEarlyTermFactory()
	poll := factory.New(validators)

	poll.Vote(validator1, votes)
	if poll.Finished() {
		t.Fatalf("Poll finished after less than alpha votes")
	}
	poll.Vote(validator1, votes)
	if poll.Finished() {
		t.Fatalf("Poll finished after getting a duplicated vote")
	}
	poll.Vote(validator2, votes)
	if !poll.Finished() {
		t.Fatalf("Poll did not terminate after receiving k votes")
	}
}
