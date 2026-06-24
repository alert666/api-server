package alert_test

import (
	"fmt"
	"testing"

	"github.com/alert666/api-server/base/helper"
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
