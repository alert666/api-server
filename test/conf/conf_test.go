package store_test

import (
	"fmt"
	"testing"

	"github.com/alert666/api-server/base/config"
	"github.com/alert666/api-server/base/log"
)

func init() {
	config.LoadConfig("../../config.yaml")
	log.NewLogger()
}

func TestGetAlertExtraSync(t *testing.T) {
	e, err := config.GetAlertExtraSync()
	if err != nil {
		t.Fatal(e)
	}
	fmt.Printf("%v", e)
}

func TestGetKubernetesEvents(t *testing.T) {
	ccc, err := config.GetKubernetesEvents()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("ccc: %#v\n", ccc)
}
