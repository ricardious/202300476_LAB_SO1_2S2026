package main

import (
	"context"
	"errors"
	"testing"

	"github.com/dapr/go-sdk/service/common"
	"so1/proyecto3/internal/report"
)

func TestDaprAcknowledgement(t *testing.T) {
	valid := []byte(`{"country":"ESP","warplanes_in_air":1,"warships_in_water":2,"timestamp":"2026-10-19T12:00:00Z","report_id":"dapr-unit"}`)
	for _, tc := range []struct {
		name                         string
		data                         []byte
		applied                      bool
		storeErr                     error
		wantCall, wantRetry, wantErr bool
	}{
		{"stored", valid, true, nil, true, false, false},
		{"duplicate", valid, false, nil, true, false, false},
		{"storage outage", valid, false, errors.New("unavailable"), true, true, true},
		{"invalid message", []byte(`{"country":"GMT"}`), false, nil, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := daprEventHandler(func(ctx context.Context, r report.Report) (bool, error) {
				called = true
				if r.ID != "dapr-unit" {
					t.Fatalf("lost report ID: %s", r.ID)
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("missing storage deadline")
				}
				return tc.applied, tc.storeErr
			})
			retry, err := h(context.Background(), &common.TopicEvent{RawData: tc.data})
			if called != tc.wantCall || retry != tc.wantRetry || (err != nil) != tc.wantErr {
				t.Fatalf("called=%t retry=%t err=%v", called, retry, err)
			}
		})
	}
}
