package types

import "encoding/json"

type SendCommandAndWaitReq struct {
	Type        int32             `json:"type" binding:"required"`
	Description string            `json:"description" binding:"required"`
	Params      map[string]string `json:"params"`
}

// InternalForwardReq is the HTTP body sent to a peer server for cross-replica command forwarding.
type InternalForwardReq struct {
	ClusterID   string            `json:"clusterId"`
	Type        int32             `json:"type"`
	Description string            `json:"description"`
	Params      map[string]string `json:"params"`
	WaitResult  bool              `json:"waitResult"`
}

// InternalForwardResp is the HTTP body returned by a peer server.
type InternalForwardResp struct {
	Success bool            `json:"success"`
	Error   string          `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}
