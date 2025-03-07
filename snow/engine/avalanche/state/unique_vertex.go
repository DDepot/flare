// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/flare-foundation/flare/cache"
	"github.com/flare-foundation/flare/ids"
	"github.com/flare-foundation/flare/snow/choices"
	"github.com/flare-foundation/flare/snow/consensus/avalanche"
	"github.com/flare-foundation/flare/snow/consensus/snowstorm"
	"github.com/flare-foundation/flare/snow/engine/avalanche/vertex"
	"github.com/flare-foundation/flare/utils/formatting"
	"github.com/flare-foundation/flare/utils/hashing"
)

var (
	_ cache.Evictable  = &uniqueVertex{}
	_ avalanche.Vertex = &uniqueVertex{}
)

// uniqueVertex acts as a cache for vertices in the database.
//
// If a vertex is loaded, it will have one canonical uniqueVertex. The vertex
// will eventually be evicted from memory, when the uniqueVertex is evicted from
// the cache. If the uniqueVertex has a function called again after this
// eviction, the vertex will be re-loaded from the database.
type uniqueVertex struct {
	serializer *Serializer

	id ids.ID
	v  *vertexState
	// default to "time.Now", used for testing
	time func() time.Time
}

// newUniqueVertex returns a uniqueVertex instance from [b] by checking the cache
// and then parsing the vertex bytes on a cache miss.
func newUniqueVertex(s *Serializer, b []byte) (*uniqueVertex, error) {
	vtx := &uniqueVertex{
		id:         hashing.ComputeHash256Array(b),
		serializer: s,
	}
	vtx.shallowRefresh()

	// If the vtx exists, then the vertex is already known
	if vtx.v.vtx != nil {
		return vtx, nil
	}

	// If it wasn't in the cache parse the vertex and set it
	innerVertex, err := s.parseVertex(b)
	if err != nil {
		return nil, err
	}
	if err := innerVertex.Verify(); err != nil {
		return nil, err
	}

	unparsedTxs := innerVertex.Txs()
	txs := make([]snowstorm.Tx, len(unparsedTxs))
	for i, txBytes := range unparsedTxs {
		tx, err := vtx.serializer.VM.ParseTx(txBytes)
		if err != nil {
			return nil, err
		}
		txs[i] = tx
	}

	vtx.v.vtx = innerVertex
	vtx.v.txs = txs

	// If the vertex has already been fetched,
	// skip persisting the vertex.
	if vtx.v.status.Fetched() {
		return vtx, nil
	}

	// The vertex is newly parsed, so set the status
	// and persist it.
	vtx.v.status = choices.Processing
	return vtx, vtx.persist()
}

func (vtx *uniqueVertex) refresh() {
	vtx.shallowRefresh()

	if vtx.v.vtx == nil && vtx.v.status.Fetched() {
		vtx.v.vtx = vtx.serializer.state.Vertex(vtx.ID())
	}
}

// shallowRefresh checks the cache for the uniqueVertex and gets the
// most up-to-date status for [vtx]
// ensures that the status is up-to-date for this vertex
// inner vertex may be nil after calling shallowRefresh
func (vtx *uniqueVertex) shallowRefresh() {
	if vtx.v == nil {
		vtx.v = &vertexState{}
	}
	if vtx.v.latest {
		return
	}

	latest := vtx.serializer.state.UniqueVertex(vtx)
	prevVtx := vtx.v.vtx
	if latest == vtx {
		vtx.v.status = vtx.serializer.state.Status(vtx.ID())
		vtx.v.latest = true
	} else {
		// If someone is in the cache, they must be up-to-date
		*vtx = *latest
	}

	if vtx.v.vtx == nil {
		vtx.v.vtx = prevVtx
	}
}

func (vtx *uniqueVertex) Evict() {
	if vtx.v != nil {
		vtx.v.latest = false
		// make sure the parents can be garbage collected
		vtx.v.parents = nil
	}
}

func (vtx *uniqueVertex) setVertex(innerVtx vertex.StatelessVertex) error {
	vtx.shallowRefresh()
	vtx.v.vtx = innerVtx

	if vtx.v.status.Fetched() {
		return nil
	}

	if _, err := vtx.Txs(); err != nil {
		return err
	}

	vtx.v.status = choices.Processing
	return vtx.persist()
}

func (vtx *uniqueVertex) persist() error {
	if err := vtx.serializer.state.SetVertex(vtx.v.vtx); err != nil {
		return err
	}
	if err := vtx.serializer.state.SetStatus(vtx.ID(), vtx.v.status); err != nil {
		return err
	}
	return vtx.serializer.versionDB.Commit()
}

func (vtx *uniqueVertex) setStatus(status choices.Status) error {
	vtx.shallowRefresh()
	if vtx.v.status == status {
		return nil
	}
	vtx.v.status = status
	return vtx.serializer.state.SetStatus(vtx.ID(), status)
}

func (vtx *uniqueVertex) ID() ids.ID       { return vtx.id }
func (vtx *uniqueVertex) Key() interface{} { return vtx.id }

func (vtx *uniqueVertex) Accept() error {
	if err := vtx.setStatus(choices.Accepted); err != nil {
		return err
	}

	vtx.serializer.edge.Add(vtx.id)
	parents, err := vtx.Parents()
	if err != nil {
		return err
	}

	for _, parent := range parents {
		vtx.serializer.edge.Remove(parent.ID())
	}

	if err := vtx.serializer.state.SetEdge(vtx.serializer.Edge()); err != nil {
		return fmt.Errorf("failed to set edge while accepting vertex %s due to %w", vtx.id, err)
	}

	// Should never traverse into parents of a decided vertex. Allows for the
	// parents to be garbage collected
	vtx.v.parents = nil

	return vtx.serializer.versionDB.Commit()
}

func (vtx *uniqueVertex) Reject() error {
	if err := vtx.setStatus(choices.Rejected); err != nil {
		return err
	}

	// Should never traverse into parents of a decided vertex. Allows for the
	// parents to be garbage collected
	vtx.v.parents = nil

	return vtx.serializer.versionDB.Commit()
}

// TODO: run performance test to see if shallow refreshing
// (which will mean that refresh must be called in Bytes and Verify)
// improves performance
func (vtx *uniqueVertex) Status() choices.Status { vtx.refresh(); return vtx.v.status }

func (vtx *uniqueVertex) Parents() ([]avalanche.Vertex, error) {
	vtx.refresh()

	if vtx.v.vtx == nil {
		return nil, fmt.Errorf("failed to get parents for vertex with status: %s", vtx.v.status)
	}

	parentIDs := vtx.v.vtx.ParentIDs()
	if len(vtx.v.parents) != len(parentIDs) {
		vtx.v.parents = make([]avalanche.Vertex, len(parentIDs))
		for i, parentID := range parentIDs {
			vtx.v.parents[i] = &uniqueVertex{
				serializer: vtx.serializer,
				id:         parentID,
			}
		}
	}

	return vtx.v.parents, nil
}

var (
	errStopVertexNotAllowedTimestamp = errors.New("stop vertex not allowed timestamp")
	errStopVertexAlreadyAccepted     = errors.New("stop vertex already accepted")
	errUnexpectedEdges               = errors.New("unexpected edge, expected accepted frontier")
	errUnexpectedDependencyStopVtx   = errors.New("unexpected dependencies found in stop vertex transitive path")
)

// "uniqueVertex" itself implements "Verify" regardless of whether the underlying vertex
// is stop vertex or not. Called before issuing the vertex to the consensus.
// No vertex should ever be able to refer to a stop vertex in its transitive closure.
func (vtx *uniqueVertex) Verify() error {
	// first verify the underlying stateless vertex
	if err := vtx.v.vtx.Verify(); err != nil {
		return err
	}

	whitelistVtx := vtx.v.vtx.StopVertex()
	if whitelistVtx {
		now := time.Now()
		if vtx.time != nil {
			now = vtx.time()
		}
		allowed := vtx.serializer.XChainMigrationTime
		if now.Before(allowed) {
			return errStopVertexNotAllowedTimestamp
		}
	}

	// edge is updated in "vtx.Accept"
	// and "vtx.serializer.Edge()" is global
	acceptedEdges := ids.NewSet(0)
	acceptedEdges.Add(vtx.serializer.Edge()...)
	for id := range acceptedEdges {
		edgeVtx, err := vtx.serializer.getUniqueVertex(id)
		if err != nil {
			return err
		}
		// MUST error if stop vertex has already been accepted (can't be accepted twice)
		// regardless of whether the underlying vertex is stop vertex or not
		if edgeVtx.v.vtx.StopVertex() {
			return errStopVertexAlreadyAccepted
		}
	}
	if !whitelistVtx {
		// below are stop vertex specific verifications
		// no need to continue
		return nil
	}

	//      (accepted)           (accepted)
	//        vtx_1                vtx_2
	//    [tx_a, tx_b]          [tx_c, tx_d]
	//          ⬆      ⬉     ⬈       ⬆
	//        vtx_3                vtx_4
	//    [tx_e, tx_f]          [tx_g, tx_h]
	//                               ⬆
	//                         stop_vertex_5
	//
	// [tx_a, tx_b] transitively referenced by "stop_vertex_5"
	// has the dependent transactions [tx_e, tx_f]
	// that are not transitively referenced by "stop_vertex_5"
	// in case "tx_g" depends on "tx_e" that is not in vtx4.
	// Thus "stop_vertex_5" is invalid!
	//
	// To make sure such transitive paths of the stop vertex reach all accepted frontier:
	// 1. check the edge of the transitive paths refers to the accepted frontier
	// 2. check dependencies of all txs must be subset of transitive paths
	queue := []avalanche.Vertex{vtx}
	visitedVtx := ids.NewSet(0)

	acceptedFrontier := ids.NewSet(0)
	transitivePaths := ids.NewSet(0)
	dependencies := ids.NewSet(0)
	for len(queue) > 0 { // perform BFS
		cur := queue[0]
		queue = queue[1:]

		curID := cur.ID()
		if cur.Status() == choices.Accepted {
			// 1. check the edge of the transitive paths refers to the accepted frontier
			acceptedFrontier.Add(curID)

			// have reached the accepted frontier on the transitive closure
			// no need to continue the search on this path
			continue
		}

		if visitedVtx.Contains(curID) {
			continue
		}
		visitedVtx.Add(curID)
		transitivePaths.Add(curID)

		txs, err := cur.Txs()
		if err != nil {
			return err
		}
		for _, tx := range txs {
			transitivePaths.Add(tx.ID())
			deps, err := tx.Dependencies()
			if err != nil {
				return err
			}
			for _, dep := range deps {
				// only add non-accepted dependencies
				if dep.Status() != choices.Accepted {
					dependencies.Add(dep.ID())
				}
			}
		}

		parents, err := cur.Parents()
		if err != nil {
			return err
		}
		queue = append(queue, parents...)
	}

	// stop vertex should be able to reach all IDs
	// that are returned by the "Edge"
	if !acceptedFrontier.Equals(acceptedEdges) {
		return errUnexpectedEdges
	}

	// 2. check dependencies of all txs must be subset of transitive paths
	prev := transitivePaths.Len()
	transitivePaths.Union(dependencies)
	if prev != transitivePaths.Len() {
		return errUnexpectedDependencyStopVtx
	}

	return nil
}

func (vtx *uniqueVertex) HasWhitelist() bool {
	return vtx.v.vtx.StopVertex()
}

// "uniqueVertex" itself implements "Whitelist" traversal iff its underlying
// "vertex.StatelessVertex" is marked as a stop vertex.
func (vtx *uniqueVertex) Whitelist() (ids.Set, error) {
	if !vtx.v.vtx.StopVertex() {
		return nil, nil
	}

	// perform BFS on transitive paths until reaching the accepted frontier
	// represents all processing transaction IDs transitively referenced by the
	// vertex
	queue := []avalanche.Vertex{vtx}
	whitlist := ids.NewSet(0)
	visitedVtx := ids.NewSet(0)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.Status() == choices.Accepted {
			// have reached the accepted frontier on the transitive closure
			// no need to continue the search on this path
			continue
		}
		curID := cur.ID()
		if visitedVtx.Contains(curID) {
			continue
		}
		visitedVtx.Add(curID)

		txs, err := cur.Txs()
		if err != nil {
			return nil, err
		}
		for _, tx := range txs {
			whitlist.Add(tx.ID())
		}
		whitlist.Add(curID)

		parents, err := cur.Parents()
		if err != nil {
			return nil, err
		}
		queue = append(queue, parents...)
	}
	return whitlist, nil
}

func (vtx *uniqueVertex) Height() (uint64, error) {
	vtx.refresh()

	if vtx.v.vtx == nil {
		return 0, fmt.Errorf("failed to get height for vertex with status: %s", vtx.v.status)
	}

	return vtx.v.vtx.Height(), nil
}

func (vtx *uniqueVertex) Epoch() (uint32, error) {
	vtx.refresh()

	if vtx.v.vtx == nil {
		return 0, fmt.Errorf("failed to get epoch for vertex with status: %s", vtx.v.status)
	}

	return vtx.v.vtx.Epoch(), nil
}

func (vtx *uniqueVertex) Txs() ([]snowstorm.Tx, error) {
	vtx.refresh()

	if vtx.v.vtx == nil {
		return nil, fmt.Errorf("failed to get txs for vertex with status: %s", vtx.v.status)
	}

	txs := vtx.v.vtx.Txs()
	if len(txs) != len(vtx.v.txs) {
		vtx.v.txs = make([]snowstorm.Tx, len(txs))
		for i, txBytes := range txs {
			tx, err := vtx.serializer.VM.ParseTx(txBytes)
			if err != nil {
				return nil, err
			}
			vtx.v.txs[i] = tx
		}
	}

	return vtx.v.txs, nil
}

func (vtx *uniqueVertex) Bytes() []byte { return vtx.v.vtx.Bytes() }

func (vtx *uniqueVertex) String() string {
	sb := strings.Builder{}

	parents, err := vtx.Parents()
	if err != nil {
		sb.WriteString(fmt.Sprintf("Vertex(ID = %s, Error=error while retrieving vertex parents: %s)", vtx.ID(), err))
		return sb.String()
	}
	txs, err := vtx.Txs()
	if err != nil {
		sb.WriteString(fmt.Sprintf("Vertex(ID = %s, Error=error while retrieving vertex txs: %s)", vtx.ID(), err))
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf(
		"Vertex(ID = %s, Status = %s, Number of Dependencies = %d, Number of Transactions = %d)",
		vtx.ID(),
		vtx.Status(),
		len(parents),
		len(txs),
	))

	parentFormat := fmt.Sprintf("\n    Parent[%s]: ID = %%s, Status = %%s",
		formatting.IntFormat(len(parents)-1))
	for i, parent := range parents {
		sb.WriteString(fmt.Sprintf(parentFormat, i, parent.ID(), parent.Status()))
	}

	txFormat := fmt.Sprintf("\n    Transaction[%s]: ID = %%s, Status = %%s",
		formatting.IntFormat(len(txs)-1))
	for i, tx := range txs {
		sb.WriteString(fmt.Sprintf(txFormat, i, tx.ID(), tx.Status()))
	}

	return sb.String()
}

type vertexState struct {
	latest bool

	vtx    vertex.StatelessVertex
	status choices.Status

	parents []avalanche.Vertex
	txs     []snowstorm.Tx
}
