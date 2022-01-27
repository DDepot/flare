// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"github.com/flare-foundation/flare/utils/constants"
	"github.com/flare-foundation/flare/utils/sampler"
)

// getIPs returns the beacon IPs for each network
func getIPs(networkID uint32) []string {
	switch networkID {
	case constants.FlareID:
		return []string{}
	case constants.SongbirdID:
		return []string{}
	case constants.CostonID:
		return []string{
			"34.159.59.126:9651",
			"34.159.241.96:9651",
		}
	}
	return nil
}

// getNodeIDs returns the beacon node IDs for each network
func getNodeIDs(networkID uint32) []string {
	switch networkID {
	case constants.FlareID:
		return []string{}
	case constants.SongbirdID:
		return []string{}
	case constants.CostonID:
		return []string{
			"NodeID-577wsCQeeVmkEt1AcMd84oi8LrHpexNU2",
			"NodeID-5ifRWcYdqF4mh5xjWv1MnRka4M6wjx3N1",
		}
	}
	return nil
}

// SampleBeacons returns the some beacons this node should connect to
func SampleBeacons(networkID uint32, count int) ([]string, []string) {
	ips := getIPs(networkID)
	ids := getNodeIDs(networkID)

	if numIPs := len(ips); numIPs < count {
		count = numIPs
	}

	sampledIPs := make([]string, 0, count)
	sampledIDs := make([]string, 0, count)

	s := sampler.NewUniform()
	_ = s.Initialize(uint64(len(ips)))
	indices, _ := s.Sample(count)
	for _, index := range indices {
		sampledIPs = append(sampledIPs, ips[int(index)])
		sampledIDs = append(sampledIDs, ids[int(index)])
	}

	return sampledIPs, sampledIDs
}
