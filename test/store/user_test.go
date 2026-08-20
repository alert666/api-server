package store_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/store"
)

func TestCreateTable(t *testing.T) {
	if err := db.AutoMigrate(&model.Oauth2User{}); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
}

func TestStoreQ(t *testing.T) {
	err := store.Q.Transaction(func(tx *store.Query) error {
		tx.WithContext(context.Background()).AlertSendRecord.Create(&model.AlertSendRecord{
			ID:         2,
			SendStatus: "sss",
		})
		tx.WithContext(context.Background()).AlertSendRecord.Create(&model.AlertSendRecord{
			ID:         1,
			SendStatus: "sss",
		})
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestTransformationAlertHistoryToAlertReq(t *testing.T) {
	object, err := store.AlertHistory.WithContext(context.TODO()).Where(store.AlertHistory.Alertname.Eq("IDCHeartbeatFailed")).First()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("object: %v\n", object)
}
