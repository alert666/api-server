package helper

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"time"

	"github.com/alert666/api-server/base/types"
)

var re = regexp.MustCompile(`Successfully pulled image (.+?) in (.+?) \(.*?\)\. Image size: (\d+) bytes\.`)

func ParseImagePullEvent(msg string) (*types.PulledImageEvent, error) {
	matches := re.FindStringSubmatch(msg)
	if len(matches) < 4 {
		return nil, fmt.Errorf("消息格式不匹配: %s", msg)
	}

	imageName := matches[1]
	durationStr := matches[2]
	sizeStr := matches[3]

	size, err := strconv.ParseUint(sizeStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("解析镜像大小失败: %s, 错误: %w", sizeStr, err)
	}

	// 解析时长（可选）
	duration, err := parseDuration(durationStr)
	if err != nil {
		log.Printf("解析时长失败: %s, 错误: %v", durationStr, err)
	}

	return &types.PulledImageEvent{
		ImageName:       imageName,
		DurationSeconds: duration,
		SizeByte:        size,
	}, nil
}

// 解析时长字符串（如 "32m52.292s"）
func parseDuration(durationStr string) (uint64, error) {
	d, err := time.ParseDuration(durationStr)
	if err != nil {
		return 0, err
	}
	return uint64(d.Seconds()), nil
}
