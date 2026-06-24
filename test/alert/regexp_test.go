package alert_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/model"
)

const template = `template_id: "AAqtHMkdgJ5i6"
template_version_name: "1.0.1"
template_variable:
  alertName: {{ index .Labels "alertname" | printf "%q" }}
  alertDescribe: {{ getDescript . | printf "%q" }}
  alertCluster: {{ getClusterLabel (index .Labels "cluster") }}
  alertLevel: {{ index .Labels "severity" | printf "%q" }}
  alertStartTime: {{ timeFormat .StartsAt | printf "%q" }}
  alertEndTime: {{ getEndTime .EndsAt "告警未恢复" | printf "%q" }}
  alertUser: "<at id=ljh202606></at>"
  grafanaLink: {{ newViewLink (getGrafanaExploreLink "https://kp-grafana.prod.karmada.suanleme.local" .GeneratorURL "thanos" ) }}
  alertmanagerAddr: {{ newAlertManagerLink "https://cloud.suanlene.cn/workspace/alert/history?page=1&pageSize=15&status=firing" (index .Labels "cluster") }}`

func TestRese(t *testing.T) {
	rid, result := helper.OverrideAt("oc_119e4c05afe7189a9c82e52489ede217;;<at email=huyf@suanleme.cn></at>", template)
	fmt.Println("\n========== 🔥🔥🔥 DEBUG TRACE 🔥🔥🔥 ==========")
	fmt.Printf("[DBG] %s:%d %s = %#v\n", "regexp_test.go", 24, "rid", rid)
	fmt.Printf("[DBG] %s:%d %s = %#v\n", "regexp_test.go", 24, "result", result)
	fmt.Println("========== 🔥🔥🔥 DEBUG END 🔥🔥🔥 ==========")
}

func TestGetRemoteReceive(t *testing.T) {
	log.NewLogger()

	te := &model.AlertTemplate{
		ReceiveIdType: string(model.Remote),
		ReceiveId:     []string{"http://127.0.0.1:9090/api/v1/tenant/node-pod-region;;4045d6c1da2ab78e2fc21e6956bb79f4a5678b75d09d2eddaa8f838399043969;;chat_id"},
		// ReceiveId:     []string{"https://gongjiyun-business-data.suanlene.cn/api/v1/tenant/node-pod-region;;4045d6c1da2ab78e2fc21e6956bb79f4a5678b75d09d2eddaa8f838399043969;;chat_id"},
	}

	if err := helper.GetRemoteReceive(context.Background(), "cn-henan-2", te); err != nil {
		t.Fatal(err)
	}

	by, err := json.Marshal(&te)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("\n========== 🔥🔥🔥 DEBUG TRACE 🔥🔥🔥 ==========")
	fmt.Println(string(by))
	fmt.Println("========== 🔥🔥🔥 DEBUG END 🔥🔥🔥 ==========")
}
