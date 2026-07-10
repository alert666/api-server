package regexp_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/alert666/api-server/base/helper"
)

func TestGegexp(t *testing.T) {

	var msgs = []string{
		"Container image harbor1.suanleme.cn/public-hub/laiqwq/mineru-saas-worker:v4.6.2-liaojie-rc1 already present on machine",
		"Container image easzlab.io.local:5000/istio/proxyv2:1.24.2 already present on machine",
		"Container image easzlab.io.local:5000/istio/proxyv2:1.24.2 already present on machine11",
		"Container image easzlab.io.local:5000/istio/proxyv2:1.24.2 already  on machine",
		"image easzlab.io.local:5000/istio/proxyv2:1.24.2 on machine",
		"image easzlab.io.local:5000/istio/proxyv2:1.24.2 on machine111",
	}

	for _, v := range msgs {
		matched, err := regexp.MatchString(
			`^Container image .* already present on machine$`,
			v,
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(matched)
	}
}

func TestPulleImage(t *testing.T) {
	messages := []string{
		"Successfully pulled image registry.cn-beijing.aliyuncs.com/qqlx/gongjiyun-business-data:main-d5fa6bb-20260723-022910 in 3.577s (3.577s including waiting). Image size: 15968935 bytes.",
		"Successfully pulled image harbor.suanleme.cn/public-hub/vllm-openai:v2026-3-5-nightly in 12m49.914s (12m49.915s including waiting). Image size: 9268379718 bytes.",
		"Successfully pulled image harbor.suanleme.cn/public-hub/vllm-openai:v2026-3-5-nightly in 12h49.914s (12m49.915s including waiting). Image size: 9268379718 bytes.",
	}

	for _, msg := range messages {
		event, err := helper.ParseImagePullEvent(msg)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("event: %#v\n", event)
	}
}
