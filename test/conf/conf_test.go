package store_test

import (
	"fmt"
	"testing"

	"github.com/alert666/api-server/base/conf"
	"github.com/alert666/api-server/base/log"
)

func init() {
	conf.LoadConfig("../../config.yaml")
	log.NewLogger()
}

func TestGetAlertExtraSync(t *testing.T) {
	e, err := conf.GetAlertExtraSync()
	if err != nil {
		t.Fatal(e)
	}
	fmt.Printf("%v", e)
}
