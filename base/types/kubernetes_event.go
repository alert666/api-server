package types

import (
	"time"
)

// KubernetesEventReceiveReq 使用自定义时间类型
type KubernetesEventReceiveReq struct {
	MetadataName       string     `json:"metadataName"`
	ApiVersion         string     `json:"apiVersion"`
	FirstTimestamp     string     `json:"firstTimestamp"`
	LastTimestamp      string     `json:"lastTimestamp"`
	EventTime          string     `json:"eventTime"`
	Kind               string     `json:"kind"`
	Message            string     `json:"message"`
	Name               string     `json:"name"`
	Namespace          string     `json:"namespace"`
	Reason             string     `json:"reason"`
	ReportingComponent string     `json:"reportingComponent"`
	ReportingInstance  string     `json:"reportingInstance"`
	Type               string     `json:"type"`
	FirstTimestampTime *time.Time `json:"-"`
	LastTimestampTime  *time.Time `json:"-"`
	EventTimeTime      *time.Time `json:"-"`
}

// UnmarshalJSON 实现 json.Unmarshaler 接口
func (e *KubernetesEventReceiveReq) ParseTime() error {
	// 支持多种格式
	layouts := []string{
		"2006-01-02 15:04:05 -0700 MST", // 2026-07-23 17:54:01 +0800 CST
		"2006-01-02 15:04:05 -0700 UTC", // 0001-01-01 00:00:00 +0000 UTC
		"2006-01-02 15:04:05 -0700",     // 2026-07-23 17:54:01 +0800
		"2006-01-02 15:04:05 MST",       // 2026-07-23 17:54:01 CST
		"2006-01-02 15:04:05",           // 2026-07-23 17:54:01
		time.DateTime,                   // 2026-07-23 17:54:01
		time.RFC3339,                    // 2026-07-23T17:54:01+08:00
		time.RFC3339Nano,                // 2026-07-23T17:54:01.123+08:00
	}

	for _, layout := range layouts {
		if e.EventTime != "" {
			if t, err := time.Parse(layout, e.EventTime); err != nil {
				continue
			} else {
				e.EventTimeTime = new(t)
				break
			}
		}
	}

	for _, layout := range layouts {
		if e.FirstTimestamp != "" {
			if t, err := time.Parse(layout, e.FirstTimestamp); err != nil {
				continue
			} else {
				e.FirstTimestampTime = new(t)
				break
			}
		}
	}

	for _, layout := range layouts {
		if e.LastTimestamp != "" {
			if t, err := time.Parse(layout, e.LastTimestamp); err != nil {
				continue
			} else {
				e.LastTimestampTime = new(t)
				break
			}
		}
	}

	return nil
}

type QueryImagePullDurationReq struct {
	StartTimestamp int64 `form:"startTimestamp" binding:"required,gt=0"`
	EndTimestamp   int64 `form:"endTimestamp" binding:"required,gt=0"`
}

type QueryImagePullDurationRes struct {
	PulledImageEvents []*PulledImageEvent `json:"pulledImageEvents"`
	StartTimestamp    int64               `form:"startTime"`
	EndTimestamp      int64               `form:"endTime"`
	DurationSeconds   uint64              `json:"durationSeconds"`
	SizeByte          uint64              `json:"sizeByte"`
}

type PulledImageEvent struct {
	ImageName       string `json:"imageName"`
	DurationSeconds uint64 `json:"durationSeconds"`
	SizeByte        uint64 `json:"sizeByte"`
}
