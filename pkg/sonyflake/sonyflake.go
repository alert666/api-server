package sonyflake

import (
	"errors"
	"fmt"
	"time"

	"github.com/sony/sonyflake"
)

type SonyflakeID struct {
	sf *sonyflake.Sonyflake
}

func NewSonyflakeID() (*SonyflakeID, error) {
	settings := sonyflake.Settings{
		StartTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	instance := sonyflake.NewSonyflake(settings)
	if instance == nil {
		return nil, errors.New("sonyflake instance created failed")
	}

	return &SonyflakeID{sf: instance}, nil
}

// NextID 获取 id
func (g *SonyflakeID) NextID() (uint64, error) {
	if g.sf == nil {
		return 0, errors.New("sonyflake not initialized")
	}
	id, err := g.sf.NextID()
	if err != nil {
		return 0, fmt.Errorf("sonyflake next id failed: %w", err)
	}
	return id, nil
}
