// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpcchainvm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/flare-foundation/flare/api/keystore/gkeystore"
	"github.com/flare-foundation/flare/api/metrics"
	"github.com/flare-foundation/flare/api/proto/appsenderproto"
	"github.com/flare-foundation/flare/api/proto/galiasreaderproto"
	"github.com/flare-foundation/flare/api/proto/ghttpproto"
	"github.com/flare-foundation/flare/api/proto/gkeystoreproto"
	"github.com/flare-foundation/flare/api/proto/gsharedmemoryproto"
	"github.com/flare-foundation/flare/api/proto/gsubnetlookupproto"
	"github.com/flare-foundation/flare/api/proto/messengerproto"
	"github.com/flare-foundation/flare/api/proto/rpcdbproto"
	"github.com/flare-foundation/flare/api/proto/vmproto"
	"github.com/flare-foundation/flare/chains/atomic/gsharedmemory"
	"github.com/flare-foundation/flare/database/manager"
	"github.com/flare-foundation/flare/database/rpcdb"
	"github.com/flare-foundation/flare/ids"
	"github.com/flare-foundation/flare/ids/galiasreader"
	"github.com/flare-foundation/flare/snow"
	"github.com/flare-foundation/flare/snow/choices"
	"github.com/flare-foundation/flare/snow/consensus/snowman"
	"github.com/flare-foundation/flare/snow/engine/common"
	"github.com/flare-foundation/flare/snow/engine/common/appsender"
	"github.com/flare-foundation/flare/snow/engine/snowman/block"
	"github.com/flare-foundation/flare/snow/validation"
	"github.com/flare-foundation/flare/utils/wrappers"
	"github.com/flare-foundation/flare/version"
	"github.com/flare-foundation/flare/vms/components/chain"
	"github.com/flare-foundation/flare/vms/rpcchainvm/ghttp"
	"github.com/flare-foundation/flare/vms/rpcchainvm/grpcutils"
	"github.com/flare-foundation/flare/vms/rpcchainvm/gsubnetlookup"
	"github.com/flare-foundation/flare/vms/rpcchainvm/messenger"
)

var (
	errUnsupportedFXs = errors.New("unsupported feature extensions")

	_ block.ChainVM              = &VMClient{}
	_ block.BatchedChainVM       = &VMClient{}
	_ block.HeightIndexedChainVM = &VMClient{}
)

const (
	decidedCacheSize    = 2048
	missingCacheSize    = 2048
	unverifiedCacheSize = 2048
	bytesToIDCacheSize  = 2048
)

// VMClient is an implementation of VM that talks over RPC.
type VMClient struct {
	*chain.State
	client vmproto.VMClient
	broker *plugin.GRPCBroker
	proc   *plugin.Client

	messenger    *messenger.Server
	keystore     *gkeystore.Server
	sharedMemory *gsharedmemory.Server
	bcLookup     *galiasreader.Server
	snLookup     *gsubnetlookup.Server
	appSender    *appsender.Server

	serverCloser grpcutils.ServerCloser
	conns        []*grpc.ClientConn

	ctx *snow.Context
}

// NewClient returns a VM connected to a remote VM
func NewClient(client vmproto.VMClient, broker *plugin.GRPCBroker) *VMClient {
	return &VMClient{
		client: client,
		broker: broker,
	}
}

// SetProcess gives ownership of the server process to the client.
func (vm *VMClient) SetProcess(proc *plugin.Client) {
	vm.proc = proc
}

func (vm *VMClient) Initialize(
	ctx *snow.Context,
	dbManager manager.Manager,
	genesisBytes []byte,
	upgradeBytes []byte,
	configBytes []byte,
	toEngine chan<- common.Message,
	fxs []*common.Fx,
	appSender common.AppSender,
) error {
	if len(fxs) != 0 {
		return errUnsupportedFXs
	}

	vm.ctx = ctx

	// Initialize and serve each database and construct the db manager
	// initialize request parameters
	versionedDBs := dbManager.GetDatabases()
	versionedDBServers := make([]*vmproto.VersionedDBServer, len(versionedDBs))
	for i, semDB := range versionedDBs {
		dbBrokerID := vm.broker.NextId()
		db := rpcdb.NewServer(semDB.Database)
		go vm.broker.AcceptAndServe(dbBrokerID, vm.startDBServerFunc(db))
		versionedDBServers[i] = &vmproto.VersionedDBServer{
			DbServer: dbBrokerID,
			Version:  semDB.Version.String(),
		}
	}

	vm.messenger = messenger.NewServer(toEngine)
	vm.keystore = gkeystore.NewServer(ctx.Keystore, vm.broker)
	vm.sharedMemory = gsharedmemory.NewServer(ctx.SharedMemory, dbManager.Current().Database)
	vm.bcLookup = galiasreader.NewServer(ctx.BCLookup)
	vm.snLookup = gsubnetlookup.NewServer(ctx.SNLookup)
	vm.appSender = appsender.NewServer(appSender)

	// start the gRPC init server
	initServerID := vm.broker.NextId()
	go vm.broker.AcceptAndServe(initServerID, vm.startInitServer)

	resp, err := vm.client.Initialize(context.Background(), &vmproto.InitializeRequest{
		NetworkId:    ctx.NetworkID,
		SubnetId:     ctx.SubnetID[:],
		ChainId:      ctx.ChainID[:],
		NodeId:       ctx.NodeID.Bytes(),
		XChainId:     ctx.XChainID[:],
		AvaxAssetId:  ctx.AVAXAssetID[:],
		GenesisBytes: genesisBytes,
		UpgradeBytes: upgradeBytes,
		ConfigBytes:  configBytes,
		DbServers:    versionedDBServers,
		InitServer:   initServerID,
	})
	if err != nil {
		return err
	}

	id, err := ids.ToID(resp.LastAcceptedId)
	if err != nil {
		return err
	}
	parentID, err := ids.ToID(resp.LastAcceptedParentId)
	if err != nil {
		return err
	}

	status := choices.Status(resp.Status)
	if err := status.Valid(); err != nil {
		return err
	}

	timestamp := time.Time{}
	if err := timestamp.UnmarshalBinary(resp.Timestamp); err != nil {
		return err
	}

	lastAcceptedBlk := &BlockClient{
		vm:       vm,
		id:       id,
		parentID: parentID,
		status:   status,
		bytes:    resp.Bytes,
		height:   resp.Height,
		time:     timestamp,
	}

	registerer := prometheus.NewRegistry()
	multiGatherer := metrics.NewMultiGatherer()
	if err := multiGatherer.Register("rpcchainvm", registerer); err != nil {
		return err
	}
	if err := multiGatherer.Register("", vm); err != nil {
		return err
	}

	chainState, err := chain.NewMeteredState(
		registerer,
		&chain.Config{
			DecidedCacheSize:    decidedCacheSize,
			MissingCacheSize:    missingCacheSize,
			UnverifiedCacheSize: unverifiedCacheSize,
			BytesToIDCacheSize:  bytesToIDCacheSize,
			LastAcceptedBlock:   lastAcceptedBlk,
			GetBlock:            vm.getBlock,
			UnmarshalBlock:      vm.parseBlock,
			BuildBlock:          vm.buildBlock,
		},
	)
	if err != nil {
		return err
	}
	vm.State = chainState

	return vm.ctx.Metrics.Register(multiGatherer)
}

func (vm *VMClient) startDBServerFunc(db rpcdbproto.DatabaseServer) func(opts []grpc.ServerOption) *grpc.Server { // #nolint
	return func(opts []grpc.ServerOption) *grpc.Server {
		opts = append(opts, serverOptions...)
		server := grpc.NewServer(opts...)
		vm.serverCloser.Add(server)

		rpcdbproto.RegisterDatabaseServer(server, db)

		return server
	}
}

func (vm *VMClient) startInitServer(opts []grpc.ServerOption) *grpc.Server {
	opts = append(opts, serverOptions...)
	server := grpc.NewServer(opts...)
	vm.serverCloser.Add(server)

	// register the messenger service
	messengerproto.RegisterMessengerServer(server, vm.messenger)
	// register the keystore service
	gkeystoreproto.RegisterKeystoreServer(server, vm.keystore)
	// register the shared memory service
	gsharedmemoryproto.RegisterSharedMemoryServer(server, vm.sharedMemory)
	// register the blockchain alias service
	galiasreaderproto.RegisterAliasReaderServer(server, vm.bcLookup)
	// register the subnet alias service
	gsubnetlookupproto.RegisterSubnetLookupServer(server, vm.snLookup)
	// register the AppSender service
	appsenderproto.RegisterAppSenderServer(server, vm.appSender)

	return server
}

func (vm *VMClient) SetState(state snow.State) error {
	_, err := vm.client.SetState(context.Background(), &vmproto.SetStateRequest{
		State: uint32(state),
	})

	return err
}

func (vm *VMClient) Shutdown() error {
	errs := wrappers.Errs{}
	_, err := vm.client.Shutdown(context.Background(), &emptypb.Empty{})
	errs.Add(err)

	vm.serverCloser.Stop()
	for _, conn := range vm.conns {
		errs.Add(conn.Close())
	}

	vm.proc.Kill()
	return errs.Err
}

func (vm *VMClient) CreateHandlers() (map[string]*common.HTTPHandler, error) {
	resp, err := vm.client.CreateHandlers(context.Background(), &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	handlers := make(map[string]*common.HTTPHandler, len(resp.Handlers))
	for _, handler := range resp.Handlers {
		conn, err := vm.broker.Dial(handler.Server)
		if err != nil {
			return nil, err
		}

		vm.conns = append(vm.conns, conn)
		handlers[handler.Prefix] = &common.HTTPHandler{
			LockOptions: common.LockOption(handler.LockOptions),
			Handler:     ghttp.NewClient(ghttpproto.NewHTTPClient(conn), vm.broker),
		}
	}
	return handlers, nil
}

func (vm *VMClient) CreateStaticHandlers() (map[string]*common.HTTPHandler, error) {
	resp, err := vm.client.CreateStaticHandlers(context.Background(), &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	handlers := make(map[string]*common.HTTPHandler, len(resp.Handlers))
	for _, handler := range resp.Handlers {
		conn, err := vm.broker.Dial(handler.Server)
		if err != nil {
			return nil, err
		}

		vm.conns = append(vm.conns, conn)
		handlers[handler.Prefix] = &common.HTTPHandler{
			LockOptions: common.LockOption(handler.LockOptions),
			Handler:     ghttp.NewClient(ghttpproto.NewHTTPClient(conn), vm.broker),
		}
	}
	return handlers, nil
}

func (vm *VMClient) buildBlock() (snowman.Block, error) {
	resp, err := vm.client.BuildBlock(context.Background(), &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	id, err := ids.ToID(resp.Id)
	if err != nil {
		return nil, err
	}

	parentID, err := ids.ToID(resp.ParentId)
	if err != nil {
		return nil, err
	}

	timestamp := time.Time{}
	if err := timestamp.UnmarshalBinary(resp.Timestamp); err != nil {
		return nil, err
	}

	return &BlockClient{
		vm:       vm,
		id:       id,
		parentID: parentID,
		status:   choices.Processing,
		bytes:    resp.Bytes,
		height:   resp.Height,
		time:     timestamp,
	}, nil
}

func (vm *VMClient) parseBlock(bytes []byte) (snowman.Block, error) {
	resp, err := vm.client.ParseBlock(context.Background(), &vmproto.ParseBlockRequest{
		Bytes: bytes,
	})
	if err != nil {
		return nil, err
	}

	id, err := ids.ToID(resp.Id)
	if err != nil {
		return nil, err
	}

	parentID, err := ids.ToID(resp.ParentId)
	if err != nil {
		return nil, err
	}

	status := choices.Status(resp.Status)
	if err := status.Valid(); err != nil {
		return nil, err
	}

	timestamp := time.Time{}
	if err := timestamp.UnmarshalBinary(resp.Timestamp); err != nil {
		return nil, err
	}

	blk := &BlockClient{
		vm:       vm,
		id:       id,
		parentID: parentID,
		status:   status,
		bytes:    bytes,
		height:   resp.Height,
		time:     timestamp,
	}

	return blk, nil
}

func (vm *VMClient) getBlock(id ids.ID) (snowman.Block, error) {
	resp, err := vm.client.GetBlock(context.Background(), &vmproto.GetBlockRequest{
		Id: id[:],
	})
	if err != nil {
		return nil, err
	}

	parentID, err := ids.ToID(resp.ParentId)
	if err != nil {
		return nil, err
	}

	status := choices.Status(resp.Status)
	if err := status.Valid(); err != nil {
		return nil, err
	}

	timestamp := time.Time{}
	if err := timestamp.UnmarshalBinary(resp.Timestamp); err != nil {
		return nil, err
	}

	blk := &BlockClient{
		vm:       vm,
		id:       id,
		parentID: parentID,
		status:   status,
		bytes:    resp.Bytes,
		height:   resp.Height,
		time:     timestamp,
	}

	return blk, nil
}

func (vm *VMClient) SetPreference(id ids.ID) error {
	_, err := vm.client.SetPreference(context.Background(), &vmproto.SetPreferenceRequest{
		Id: id[:],
	})
	return err
}

func (vm *VMClient) HealthCheck() (interface{}, error) {
	return vm.client.Health(
		context.Background(),
		&emptypb.Empty{},
	)
}

func (vm *VMClient) AppRequest(nodeID ids.ShortID, requestID uint32, deadline time.Time, request []byte) error {
	deadlineBytes, err := deadline.MarshalBinary()
	if err != nil {
		return err
	}
	_, err = vm.client.AppRequest(
		context.Background(),
		&vmproto.AppRequestMsg{
			NodeId:    nodeID[:],
			RequestId: requestID,
			Request:   request,
			Deadline:  deadlineBytes,
		},
	)
	return err
}

func (vm *VMClient) AppResponse(nodeID ids.ShortID, requestID uint32, response []byte) error {
	_, err := vm.client.AppResponse(
		context.Background(),
		&vmproto.AppResponseMsg{
			NodeId:    nodeID[:],
			RequestId: requestID,
			Response:  response,
		},
	)
	return err
}

func (vm *VMClient) AppRequestFailed(nodeID ids.ShortID, requestID uint32) error {
	_, err := vm.client.AppRequestFailed(
		context.Background(),
		&vmproto.AppRequestFailedMsg{
			NodeId:    nodeID[:],
			RequestId: requestID,
		},
	)
	return err
}

func (vm *VMClient) AppGossip(nodeID ids.ShortID, msg []byte) error {
	_, err := vm.client.AppGossip(
		context.Background(),
		&vmproto.AppGossipMsg{
			NodeId: nodeID[:],
			Msg:    msg,
		},
	)
	return err
}

func (vm *VMClient) VerifyHeightIndex() error {
	resp, err := vm.client.VerifyHeightIndex(
		context.Background(),
		&emptypb.Empty{},
	)
	if err != nil {
		return err
	}
	return errCodeToError[resp.Err]
}

func (vm *VMClient) GetBlockIDAtHeight(height uint64) (ids.ID, error) {
	resp, err := vm.client.GetBlockIDAtHeight(
		context.Background(),
		&vmproto.GetBlockIDAtHeightRequest{Height: height},
	)
	if err != nil {
		return ids.Empty, err
	}
	if errCode := resp.Err; errCode != 0 {
		return ids.Empty, errCodeToError[errCode]
	}
	return ids.ToID(resp.BlkId)
}

func (vm *VMClient) GetAncestors(
	blkID ids.ID,
	maxBlocksNum int,
	maxBlocksSize int,
	maxBlocksRetrivalTime time.Duration,
) ([][]byte, error) {
	resp, err := vm.client.GetAncestors(context.Background(), &vmproto.GetAncestorsRequest{
		BlkId:                 blkID[:],
		MaxBlocksNum:          int32(maxBlocksNum),
		MaxBlocksSize:         int32(maxBlocksSize),
		MaxBlocksRetrivalTime: int64(maxBlocksRetrivalTime),
	})
	if err != nil {
		return nil, err
	}
	return resp.BlksBytes, nil
}

func (vm *VMClient) BatchedParseBlock(blksBytes [][]byte) ([]snowman.Block, error) {
	resp, err := vm.client.BatchedParseBlock(context.Background(), &vmproto.BatchedParseBlockRequest{
		Request: blksBytes,
	})
	if err != nil {
		return nil, err
	}
	if len(blksBytes) != len(resp.Response) {
		return nil, fmt.Errorf("BatchedParse block returned different number of blocks than expected")
	}

	res := make([]snowman.Block, 0, len(blksBytes))
	for idx, blkResp := range resp.Response {
		id, err := ids.ToID(blkResp.Id)
		if err != nil {
			return nil, err
		}

		parentID, err := ids.ToID(blkResp.ParentId)
		if err != nil {
			return nil, err
		}

		status := choices.Status(blkResp.Status)
		if err := status.Valid(); err != nil {
			return nil, err
		}

		timestamp := time.Time{}
		if err := timestamp.UnmarshalBinary(blkResp.Timestamp); err != nil {
			return nil, err
		}

		blk := &BlockClient{
			vm:       vm,
			id:       id,
			parentID: parentID,
			status:   status,
			bytes:    blksBytes[idx],
			height:   blkResp.Height,
			time:     timestamp,
		}

		res = append(res, blk)
	}

	return res, nil
}

func (vm *VMClient) Version() (string, error) {
	resp, err := vm.client.Version(
		context.Background(),
		&emptypb.Empty{},
	)
	if err != nil {
		return "", err
	}
	return resp.Version, nil
}

func (vm *VMClient) Connected(nodeID ids.ShortID, nodeVersion version.Application) error {
	_, err := vm.client.Connected(context.Background(), &vmproto.ConnectedRequest{
		NodeId:  nodeID[:],
		Version: nodeVersion.String(),
	})
	return err
}

func (vm *VMClient) Disconnected(nodeID ids.ShortID) error {
	_, err := vm.client.Disconnected(context.Background(), &vmproto.DisconnectedRequest{
		NodeId: nodeID[:],
	})
	return err
}

func (vm *VMClient) GetValidators(blockID ids.ID) (validation.Set, error) {
	res, err := vm.client.FetchValidators(context.Background(), &vmproto.FetchValidatorsRequest{
		BlkId: blockID[:],
	})
	if err != nil {
		return nil, fmt.Errorf("could not get fetch validators: %w", err)
	}
	if len(res.ValidatorIds) != len(res.Weights) {
		return nil, fmt.Errorf("mismatch between validators and weights (%d != %d)", len(res.ValidatorIds), len(res.Weights))
	}
	validators := validation.NewSet()
	for i, validatorId := range res.ValidatorIds {
		validatorID, err := ids.ToShortID(validatorId)
		if err != nil {
			return nil, fmt.Errorf("could not parse validator ID: %w", err)
		}
		weight := res.Weights[i]
		err = validators.AddWeight(validatorID, weight)
		if err != nil {
			return nil, fmt.Errorf("could not add validator weight: %w", err)
		}
	}
	return validators, nil
}

// BlockClient is an implementation of Block that talks over RPC.
type BlockClient struct {
	vm *VMClient

	id       ids.ID
	parentID ids.ID
	status   choices.Status
	bytes    []byte
	height   uint64
	time     time.Time
}

func (b *BlockClient) ID() ids.ID { return b.id }

func (b *BlockClient) Accept() error {
	b.status = choices.Accepted
	_, err := b.vm.client.BlockAccept(context.Background(), &vmproto.BlockAcceptRequest{
		Id: b.id[:],
	})
	return err
}

func (b *BlockClient) Reject() error {
	b.status = choices.Rejected
	_, err := b.vm.client.BlockReject(context.Background(), &vmproto.BlockRejectRequest{
		Id: b.id[:],
	})
	return err
}

func (b *BlockClient) Status() choices.Status { return b.status }

func (b *BlockClient) Parent() ids.ID {
	return b.parentID
}

func (b *BlockClient) Verify() error {
	resp, err := b.vm.client.BlockVerify(context.Background(), &vmproto.BlockVerifyRequest{
		Bytes: b.bytes,
	})
	if err != nil {
		return err
	}
	return b.time.UnmarshalBinary(resp.Timestamp)
}

func (b *BlockClient) Bytes() []byte        { return b.bytes }
func (b *BlockClient) Height() uint64       { return b.height }
func (b *BlockClient) Timestamp() time.Time { return b.time }
