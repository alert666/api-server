package types

// QueryStartEndTimestamp 公共时间戳查询参数
type QueryStartEndTimestamp struct {
	StartTimestamp int64 `form:"startTimestamp" binding:"required,gt=0"`
	EndTimestamp   int64 `form:"endTimestamp" binding:"required,gt=0"`
}

type QeryIDCMetricsReq struct {
	AlertName string `form:"alertName" binding:"required,oneof=KubeNodeNotReady GPUCardLoss"` // 要查询指标名称
	*QueryStartEndTimestamp
}

type QeryIDCMetricsRes struct {
	IDCMetrics     []IDCMetrics `json:"idcMetrics"`
	StartTimestamp int64        `json:"startTimestamp"` // 查询开始时间
	EndTimestamp   int64        `json:"endTimestamp"`   // 查询结束时间
	Cluster        string       `json:"cluster"`        // 集群 id
	AlertName      string       `json:"alertName"`      // 告警名称
}

type IDCMetrics struct {
	Node                string `json:"node"`                // 节点 hostname
	IP                  string `json:"ip"`                  // 节点 IP
	AlertStartTimestamp int64  `json:"alertStartTimestamp"` // 告警开始时间
	AlertEndTimestamp   *int64 `json:"alertEndTimestamp"`   // 告警结束时间
}

func NewQeryIDCMetricsRes() *QeryIDCMetricsRes {
	return &QeryIDCMetricsRes{
		IDCMetrics: make([]IDCMetrics, 0, 30),
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
