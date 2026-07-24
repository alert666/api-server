package time_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alert666/api-server/base/types"
)

func TestParseTime(t *testing.T) {
	body := `{
		"apiVersion": "v1",
		"firstTimestamp": "2026-07-23 18:33:09 +0800 CST",
		"lastTimestamp": "2026-07-23 18:33:09 +0800 CST",
		"eventTime": "0001-01-01 00:00:00 +0000 UTC",
		"kind": "Pod",
		"message": "Container image registry.cn-beijing.aliyuncs.com/ops1341/netshoot:v0.15 already present on machine",
		"name": "netshoot-debug-7959756ff7-42rm9",
		"namespace": "default",
		"reason": "Pulled",
		"reportingComponent": "kubelet",
		"reportingInstance": "zjsx-p1-worker-10",
		"type": "Normal",
		"firstTimestampTime": "2026-07-23T18:33:09+08:00",
		"lastTimestampTime": "2026-07-23T18:33:09+08:00",
		"eventTimeTime": "0001-01-01T00:00:00Z"
}`

	var tttt types.KubernetesEventReceiveReq

	if err := json.Unmarshal([]byte(body), &tttt); err != nil {
		t.Fatal(err)
	}

	if err := tttt.ParseTime(); err != nil {
		t.Fatal(err)
	}

	fmt.Printf("tttt.FirstTimestampTime.Format(time.DateTime): %v\n", tttt.FirstTimestampTime.Format(time.DateTime))
	fmt.Printf("tttt.LastTimestampTime.Format(time.DateTime): %v\n", tttt.LastTimestampTime.Format(time.DateTime))
	if tttt.EventTimeTime.IsZero() {
		tttt.EventTimeTime = nil
	}
	fmt.Printf("tttt.FirstTimestampTime.Format(time.DateTime): %v\n", tttt.FirstTimestampTime.Format(time.DateTime))
}
