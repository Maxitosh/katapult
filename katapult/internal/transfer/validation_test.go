package transfer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

type mockPVCFinder struct {
	pvcs map[string]bool // "cluster/pvc" -> found
}

func (m *mockPVCFinder) FindHealthyPVC(_ context.Context, clusterID, pvcName string) (bool, error) {
	key := clusterID + "/" + pvcName
	return m.pvcs[key], nil
}

type mockCommander struct {
	emptyPVCs map[string]bool // "cluster/pvc" -> empty
}

func (m *mockCommander) IsPVCEmpty(_ context.Context, clusterID, pvcName string) (bool, error) {
	key := clusterID + "/" + pvcName
	empty, ok := m.emptyPVCs[key]
	if !ok {
		return true, nil
	}
	return empty, nil
}

func (m *mockCommander) SendTransferCommand(_ context.Context, _, _ string, _ any) error {
	return nil
}

func (m *mockCommander) SendCancelCommand(_ context.Context, _, _ string) error {
	return nil
}

func TestValidateTransferRequest(t *testing.T) {
	finder := &mockPVCFinder{
		pvcs: map[string]bool{
			"cluster-a/ns/pvc-src":  true,
			"cluster-b/ns/pvc-dest": true,
		},
	}

	tests := []struct {
		name       string
		req        ValidateRequest
		commander  *mockCommander
		wantErr    bool
		errContain string
	}{
		{
			name: "valid transfer",
			req: ValidateRequest{
				TransferID:         "t1",
				SourceCluster:      "cluster-a",
				SourcePVC:          "ns/pvc-src",
				DestinationCluster: "cluster-b",
				DestinationPVC:     "ns/pvc-dest",
			},
			commander: &mockCommander{emptyPVCs: map[string]bool{"cluster-b/ns/pvc-dest": true}},
		},
		{
			name: "same source and dest PVC",
			req: ValidateRequest{
				TransferID:         "t2",
				SourceCluster:      "cluster-a",
				SourcePVC:          "ns/pvc-src",
				DestinationCluster: "cluster-a",
				DestinationPVC:     "ns/pvc-src",
			},
			commander:  &mockCommander{},
			wantErr:    true,
			errContain: "cannot be the same",
		},
		{
			name: "source PVC not found",
			req: ValidateRequest{
				TransferID:         "t3",
				SourceCluster:      "cluster-x",
				SourcePVC:          "ns/pvc-missing",
				DestinationCluster: "cluster-b",
				DestinationPVC:     "ns/pvc-dest",
			},
			commander:  &mockCommander{},
			wantErr:    true,
			errContain: "source PVC ns/pvc-missing not found",
		},
		{
			name: "dest PVC not found",
			req: ValidateRequest{
				TransferID:         "t4",
				SourceCluster:      "cluster-a",
				SourcePVC:          "ns/pvc-src",
				DestinationCluster: "cluster-x",
				DestinationPVC:     "ns/pvc-missing",
			},
			commander:  &mockCommander{},
			wantErr:    true,
			errContain: "destination PVC ns/pvc-missing not found",
		},
		{
			name: "dest non-empty without overwrite",
			req: ValidateRequest{
				TransferID:         "t5",
				SourceCluster:      "cluster-a",
				SourcePVC:          "ns/pvc-src",
				DestinationCluster: "cluster-b",
				DestinationPVC:     "ns/pvc-dest",
				AllowOverwrite:     false,
			},
			commander:  &mockCommander{emptyPVCs: map[string]bool{"cluster-b/ns/pvc-dest": false}},
			wantErr:    true,
			errContain: "not empty",
		},
		{
			name: "dest non-empty with overwrite allowed",
			req: ValidateRequest{
				TransferID:         "t6",
				SourceCluster:      "cluster-a",
				SourcePVC:          "ns/pvc-src",
				DestinationCluster: "cluster-b",
				DestinationPVC:     "ns/pvc-dest",
				AllowOverwrite:     true,
			},
			commander: &mockCommander{emptyPVCs: map[string]bool{"cluster-b/ns/pvc-dest": false}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(finder, tt.commander, slog.Default())
			err := v.ValidateTransferRequest(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateTransferRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
				t.Fatalf("error %q should contain %q", err.Error(), tt.errContain)
			}
		})
	}
}

type failingPVCFinder struct{}

func (f *failingPVCFinder) FindHealthyPVC(_ context.Context, _, _ string) (bool, error) {
	return false, fmt.Errorf("db connection failed")
}

func TestValidateTransferRequest_FinderError(t *testing.T) {
	v := NewValidator(&failingPVCFinder{}, &mockCommander{}, slog.Default())
	err := v.ValidateTransferRequest(context.Background(), ValidateRequest{
		SourceCluster:      "a",
		SourcePVC:          "pvc1",
		DestinationCluster: "b",
		DestinationPVC:     "pvc2",
	})
	if err == nil {
		t.Fatal("expected error from failing finder")
	}
}
