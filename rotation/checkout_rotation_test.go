package rotation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
)

type fakeAPI struct {
	logs        json.RawMessage
	revokeCalls int
}

func (f *fakeAPI) CreateKey(context.Context, string, string, string) (json.RawMessage, error) {
	return json.RawMessage(`{"id":"key-candidate","key":"one-time-created-value"}`), nil
}
func (f *fakeAPI) RotateKey(context.Context, string, int, string) (json.RawMessage, error) {
	return json.RawMessage(`{"id":"key-candidate","key":"one-time-rotated-value"}`), nil
}
func (f *fakeAPI) SearchLogs(context.Context) (json.RawMessage, error) { return f.logs, nil }
func (f *fakeAPI) RevokeKey(context.Context, string) (json.RawMessage, error) {
	f.revokeCalls++
	return json.RawMessage(`{}`), nil
}

func TestRunMakesRevokeDecisionFromDeploymentLogs(t *testing.T) {
	tests := []struct {
		name         string
		logs         string
		execute      bool
		wantDecision string
		wantStale    []string
		wantRevokes  int
	}{
		{name: "hold while fulfillment uses old key", logs: `{"logs":[{"deployment":"fulfillment-v3","key_id":"key-candidate"}]}`, execute: true, wantDecision: "hold", wantStale: []string{"fulfillment-v3"}},
		{name: "report ready without mutation", logs: `{"logs":[{"deployment":"checkout-v8","key_id":"another-key"}]}`, wantDecision: "ready-to-revoke"},
		{name: "revoke after no old-key traffic", logs: `{"logs":[]}`, execute: true, wantDecision: "revoked", wantRevokes: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &fakeAPI{logs: json.RawMessage(tt.logs)}
			got, err := Run(context.Background(), api, Request{ProjectID: "shop", GraceHours: 24, SecretDir: filepath.Join(t.TempDir(), "keys"), ExecuteRevoke: tt.execute})
			if err != nil {
				t.Fatal(err)
			}
			if got.Decision != tt.wantDecision || !slices.Equal(got.StaleDeployments, tt.wantStale) || api.revokeCalls != tt.wantRevokes {
				t.Fatalf("got decision=%q stale=%v revokes=%d", got.Decision, got.StaleDeployments, api.revokeCalls)
			}
		})
	}
}
