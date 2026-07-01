package model

import (
	"errors"
	"strings"
	"testing"
)

func TestUpdateSendRecordStatusLeavesSuccessErrorMessageNil(t *testing.T) {
	record := UpdateSendRecordStatus(nil)

	if record.SendStatus != AlertSendRecordStatusSuccess {
		t.Fatalf("SendStatus = %q, want %q", record.SendStatus, AlertSendRecordStatusSuccess)
	}
	if record.ErrorMessage != nil {
		t.Fatalf("ErrorMessage = %q, want nil", *record.ErrorMessage)
	}
}

func TestUpdateSendRecordStatusSetsFailedErrorMessage(t *testing.T) {
	record := UpdateSendRecordStatus(errors.New("send failed"))

	if record.SendStatus != AlertSendRecordStatusFailed {
		t.Fatalf("SendStatus = %q, want %q", record.SendStatus, AlertSendRecordStatusFailed)
	}
	if record.ErrorMessage == nil {
		t.Fatal("ErrorMessage = nil, want error detail")
	}
	if !strings.Contains(*record.ErrorMessage, "send failed") {
		t.Fatalf("ErrorMessage = %q, want it to contain send failed", *record.ErrorMessage)
	}
}
