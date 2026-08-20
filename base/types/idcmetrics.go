package types

// QueryStartEndTimestamp 公共时间戳查询参数
type QueryStartEndTimestamp struct {
	StartTimestamp int64 `form:"startTimestamp" binding:"required,gt=0"`
	EndTimestamp   int64 `form:"endTimestamp" binding:"required,gt=0"`
}

type GetIDCMetricsReq struct {
	AlertName string `form:"alertName" binding:"required,oneof=KubeNodeNotReady GPUCardLoss IDCHeartbeatFailed"` // 要查询指标名称
	*QueryStartEndTimestamp
}

type GetIDCMetricsRes struct {
	IDCMetrics     []IDCMetrics `json:"idcMetrics"`
	StartTimestamp int64        `json:"startTimestamp"` // 查询开始时间
	EndTimestamp   int64        `json:"endTimestamp"`   // 查询结束时间
	Cluster        string       `json:"cluster"`        // 集群 id
	AlertName      string       `json:"alertName"`      // 告警名称
}

type IDCMetrics struct {
	Node                string `json:"node,omitempty"`                // 节点 hostname
	IP                  string `json:"ip,omitempty"`                  // 节点 IP
	AlertStartTimestamp int64  `json:"alertStartTimestamp,omitempty"` // 告警开始时间
	AlertEndTimestamp   *int64 `json:"alertEndTimestamp,omitempty"`   // 告警结束时间
}

func NewGetIDCMetricsRes() *GetIDCMetricsRes {
	return &GetIDCMetricsRes{
		IDCMetrics: make([]IDCMetrics, 0, 30),
	}
}

type QueryIDCMetricsReq struct {
	QueryIDCMetrics []QueryIDCMetrics `json:"queryIDCMetrics"`
}

type QueryIDCMetrics struct {
	AlertName      string `json:"alertName" binding:"required,oneof=KubeNodeNotReady GPUCardLoss IDCHeartbeatFailed"`
	StartTimestamp int64  `json:"startTimestamp" binding:"required,gt=0"`
	Node           string `json:"node" binding:"required"`
	IP             string `json:"ip" binding:"required,ip"`
}

type QueryIDCMetricsRes struct {
	Cluster            string               `json:"cluster"`
	QueryIDCMetricsres []QueryIDCMetricsres `json:"queryIDCMetrics"`
}

type QueryIDCMetricsres struct {
	AlertName string `json:"alertName"`
	*IDCMetrics
}

func NewQueryIDCMetricsRes() *QueryIDCMetricsRes {
	return &QueryIDCMetricsRes{
		QueryIDCMetricsres: make([]QueryIDCMetricsres, 0, 10),
	}
}

type QueryImagePullDurationReq struct {
	*QueryStartEndTimestamp
}

type QueryImagePullDurationRes struct {
	PulledImageEvents []*PulledImageEvent `json:"pulledImageEvents"`
	StartTimestamp    int64               `json:"startTimestamp"`
	EndTimestamp      int64               `json:"endTimestamp"`
	DurationSeconds   uint64              `json:"durationSeconds"`
	SizeByte          uint64              `json:"sizeByte"`
}

type PulledImageEvent struct {
	ImageName       string `json:"imageName"`
	DurationSeconds uint64 `json:"durationSeconds"`
	SizeByte        uint64 `json:"sizeByte"`
}

type IDCHeartbeatReq struct {
	Node               string `json:"node"`
	IP                 string `json:"ip"`
	HeartbeatTimestamp int64  `json:"heartbeatTimestamp"`
}

type DeleteIDCHeartbeatReq struct {
	IP string `json:"ip"`
}
