package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	pb "github.com/alert666/alertmanager-proto/gen/go/data_tunnel/v1"
	"github.com/alert666/api-server/base/conf"
	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/store"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

const (
	cmdChanBufferSize     = 20
	defaultCommandTimeout = 30 * time.Second
)

var serverAddrTTL = 2 * time.Minute

// AgentCommand wraps a command with an optional request ID.
type AgentCommand struct {
	RequestID string
	Command   *pb.Command
}

// ClusterAgent represents an agent connection within a cluster.
type ClusterAgent struct {
	AgentID string
	Ch      chan *AgentCommand
}

// DataTunnelServicer defines the data tunnel service interface.
type DataTunnelServicer interface {
	RegisterAgent(ctx context.Context, init *pb.Init) (chan *AgentCommand, error)
	UnregisterAgent(ctx context.Context, agentID string, clusterID string, ch chan *AgentCommand) error
	HandleCommandResult(ctx context.Context, agentID string, requestID string, result *pb.CommandResult) error
	SendCommandAndWait(ctx context.Context, req *types.SendCommandAndWaitReq) (*pb.CommandResult, error)
	ExecuteCommandLocally(ctx context.Context, req *types.InternalForwardReq) (*pb.CommandResult, error)
}

// DataTunnelService implements DataTunnelServicer.
type DataTunnelService struct {
	mu        sync.RWMutex
	clusters  map[string][]*ClusterAgent
	clusterRR map[string]uint64
	pendingMu sync.Mutex
	pending   map[string]chan *pb.CommandResult

	cacheStore  store.CacheStorer
	serverID    string
	restyClient *resty.Client
}

// NewDataTunnelService creates a DataTunnelService.
func NewDataTunnelService(cacheStore store.CacheStorer) DataTunnelServicer {
	serverID := generateServerID()
	s := &DataTunnelService{
		clusters:    make(map[string][]*ClusterAgent),
		clusterRR:   make(map[string]uint64),
		pending:     make(map[string]chan *pb.CommandResult),
		cacheStore:  cacheStore,
		serverID:    serverID,
		restyClient: resty.New().SetTimeout(defaultCommandTimeout),
	}

	zap.L().Info("DataTunnelService created",
		zap.String("serverID", serverID),
		zap.String("advertiseAddr", conf.GetInternalAdvertiseAddr()),
	)

	go s.refreshServerAddr()

	return s
}

// refreshServerAddr periodically writes the advertise address to Redis.
func (s *DataTunnelService) refreshServerAddr() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.RLock()
		var activeClusters []string
		for cid, agents := range s.clusters {
			if len(agents) > 0 {
				activeClusters = append(activeClusters, cid)
			}
		}
		s.mu.RUnlock()

		if len(activeClusters) == 0 {
			continue
		}

		// Refresh server address TTL.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := s.cacheStore.SetObject(ctx, store.AgentServerType, s.serverID, conf.GetInternalAdvertiseAddr(), serverAddrTTL); err != nil {
			zap.L().Warn("refreshServerAddr failed",
				zap.String("cacheType", string(store.AgentServerType)),
				zap.String("serverID", s.serverID),
			)
		}
		cancel()

		// Refresh cluster set TTL for every active cluster.
		for _, cid := range activeClusters {
			ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
			if err := s.cacheStore.SetSet(ctx2, store.AgentClusterType, cid, []any{s.serverID}, &serverAddrTTL); err != nil {
				zap.L().Warn("refreshServerAddr failed",
					zap.String("cacheType", string(store.AgentClusterType)),
					zap.String("serverID", s.serverID),
					zap.String("clusterID", cid),
				)
			}
			cancel2()
		}
	}
}

// generateServerID creates a unique identifier for this server instance.
func generateServerID() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	grpcBind := conf.GetGRPCBind()
	_, port, err := net.SplitHostPort(grpcBind)
	if err != nil {
		port = grpcBind
	}
	return fmt.Sprintf("%s-%s", host, port)
}

func (s *DataTunnelService) RegisterAgent(ctx context.Context, init *pb.Init) (chan *AgentCommand, error) {
	agentID := init.GetAgentID()
	clusterID := init.GetClusterID()

	ch := make(chan *AgentCommand, cmdChanBufferSize)

	s.mu.Lock()
	s.clusters[clusterID] = append(s.clusters[clusterID], &ClusterAgent{
		AgentID: agentID,
		Ch:      ch,
	})
	s.mu.Unlock()

	_ = s.cacheStore.DelKey(ctx, store.AgentClusterType, clusterID)
	_ = s.cacheStore.SetSet(ctx, store.AgentClusterType, clusterID, []any{s.serverID}, &serverAddrTTL)
	_ = s.cacheStore.SetObject(ctx, store.AgentServerType, s.serverID, conf.GetInternalAdvertiseAddr(), serverAddrTTL)

	zap.L().Info("agent registered",
		zap.String("agentID", agentID),
		zap.String("clusterID", clusterID),
		zap.Int("replicas", len(s.clusters[clusterID])),
	)
	return ch, nil
}

func (s *DataTunnelService) UnregisterAgent(_ context.Context, agentID string, clusterID string, ch chan *AgentCommand) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	agents := s.clusters[clusterID]
	for i, a := range agents {
		if a.Ch == ch {
			close(a.Ch)
			s.clusters[clusterID] = append(agents[:i], agents[i+1:]...)
			remaining := len(s.clusters[clusterID])
			zap.L().Info("agent unregistered",
				zap.String("agentID", agentID),
				zap.Int("remaining", remaining),
			)

			if remaining == 0 {
				delete(s.clusters, clusterID)
				delete(s.clusterRR, clusterID)
				_ = s.cacheStore.RemSet(context.Background(), store.AgentClusterType, clusterID, s.serverID)
			}
			return nil
		}
	}
	return nil
}

func (s *DataTunnelService) HandleCommandResult(_ context.Context, _ string, requestID string, result *pb.CommandResult) error {
	s.pendingMu.Lock()
	ch, ok := s.pending[requestID]
	if ok {
		delete(s.pending, requestID)
	}
	s.pendingMu.Unlock()

	if !ok {
		return nil
	}

	select {
	case ch <- result:
	default:
	}
	return nil
}

func (s *DataTunnelService) pickClusterChannel(ctx context.Context, taskID, clusterID string) (chan *AgentCommand, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agents := s.clusters[clusterID]
	if len(agents) == 0 {
		return nil, fmt.Errorf("cluster %s has no connected agents", clusterID)
	}

	idx := s.clusterRR[clusterID] % uint64(len(agents))
	s.clusterRR[clusterID]++
	log.WithRequestID(ctx).Info("request sent to agent", zap.String("taskID", taskID), zap.String("agentID", agents[idx].AgentID))
	return agents[idx].Ch, nil
}

func injectTenant(ctx context.Context, params map[string]string) map[string]string {
	if tenant, err := helper.GetTenant(ctx); err == nil {
		if params == nil {
			params = make(map[string]string)
		}
		params["tenant"] = tenant
	}
	return params
}

func (s *DataTunnelService) ExecuteCommandLocally(ctx context.Context, req *types.InternalForwardReq) (*pb.CommandResult, error) {
	var (
		clusterID  = req.ClusterID
		waitResult = req.WaitResult
		cmd        = &pb.Command{
			Type:        pb.CommandType(req.Type),
			Description: req.Description,
			Params:      req.Params,
		}
	)
	taskID := fmt.Sprintf("%s-%d", clusterID, time.Now().UnixNano())
	ch, err := s.pickClusterChannel(ctx, taskID, clusterID)
	if err != nil {
		return nil, err
	}

	if !waitResult {
		select {
		case ch <- &AgentCommand{Command: cmd}:
			return nil, nil
		default:
			return nil, fmt.Errorf("cluster %s agent channel is full", clusterID)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	resultCh := make(chan *pb.CommandResult, 1)
	s.pendingMu.Lock()
	s.pending[taskID] = resultCh
	s.pendingMu.Unlock()

	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, taskID)
		s.pendingMu.Unlock()
	}()

	select {
	case ch <- &AgentCommand{RequestID: taskID, Command: cmd}:
	default:
		return nil, fmt.Errorf("cluster %s agent channel is full", clusterID)
	}

	timeout := defaultCommandTimeout
	if dl, ok := ctx.Deadline(); ok {
		if remaining := time.Until(dl); remaining < timeout {
			timeout = remaining
		}
	}

	select {
	case result := <-resultCh:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, fmt.Errorf("command %s timed out after %v", taskID, timeout)
	}
}

func (s *DataTunnelService) findPeerServer(ctx context.Context, clusterID string) (string, error) {
	serverIDs, err := s.cacheStore.GetSet(ctx, store.AgentClusterType, clusterID)
	if err != nil {
		return "", fmt.Errorf("failed to query peer servers: %w", err)
	}

	var peers []string
	for _, m := range serverIDs {
		if m != s.serverID {
			peers = append(peers, m)
		}
	}
	if len(peers) == 0 {
		return "", fmt.Errorf("no peer server found for cluster %s", clusterID)
	}

	idx := time.Now().UnixNano() % int64(len(peers))
	peerID := peers[idx]

	var addr string
	found, err := s.cacheStore.GetObject(ctx, store.AgentServerType, peerID, &addr)
	if err != nil {
		return "", fmt.Errorf("failed to get peer %s address: %w", peerID, err)
	}
	if !found {
		return "", fmt.Errorf("peer %s address not found", peerID)
	}
	return addr, nil
}

func (s *DataTunnelService) forwardToPeer(ctx context.Context, req *types.InternalForwardReq) (*pb.CommandResult, error) {

	var (
		clusterID = req.ClusterID
		cmd       = &pb.Command{
			Type:        pb.CommandType(req.Type),
			Description: req.Description,
			Params:      req.Params,
		}
		waitResult = req.WaitResult
	)

	peerAddr, err := s.findPeerServer(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	reqBody := types.InternalForwardReq{
		ClusterID:   clusterID,
		Type:        int32(cmd.GetType()),
		Description: cmd.GetDescription(),
		Params:      cmd.GetParams(),
		WaitResult:  waitResult,
	}

	log.WithRequestID(ctx).Info("request forwarding", zap.String("clusterID", clusterID), zap.String("addr", peerAddr))

	var peerResp types.Response
	resp, err := s.restyClient.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+conf.GetInternalToken()).
		SetHeader(constant.RequestIDHeader, log.GetRequestIDFromContext(ctx)).
		SetBody(reqBody).
		SetResult(&peerResp).
		Post(peerAddr + "/internal/v1/forward-command")

	if err != nil {
		return nil, fmt.Errorf("peer request failed: %w", err)
	}

	if resp.IsError() || peerResp.Code != 0 {
		errMsg := peerResp.Error
		if errMsg == "" {
			errMsg = "unknown peer error"
		}
		return nil, fmt.Errorf("peer returned error: %s", errMsg)
	}

	perrRespByte, err := json.Marshal(&peerResp.Data)
	if err != nil {
		return nil, err
	}

	var pbResp pb.CommandResult
	if err := json.Unmarshal(perrRespByte, &pbResp); err != nil {
		return nil, err
	}

	return &pbResp, nil
}

func (s *DataTunnelService) SendCommandAndWait(ctx context.Context, req *types.SendCommandAndWaitReq) (*pb.CommandResult, error) {
	clusterID, err := helper.GetTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	req.Params = injectTenant(ctx, req.Params)

	executeCommandLocallyReq := &types.InternalForwardReq{
		ClusterID:   clusterID,
		Type:        req.Type,
		Description: req.Description,
		Params:      req.Params,
		WaitResult:  true,
	}

	log.WithRequestID(ctx).Info("SendCommandAndWait",
		zap.String("clusterID", clusterID),
		zap.Any("executeCommandLocallyReq", executeCommandLocallyReq),
	)

	result, err := s.ExecuteCommandLocally(ctx, executeCommandLocallyReq)
	if err != nil {
		log.WithRequestID(ctx).Info("agent not local, forwarding to peer",
			zap.String("clusterID", clusterID),
		)
		return s.forwardToPeer(ctx, executeCommandLocallyReq)
	}
	return result, nil
}
