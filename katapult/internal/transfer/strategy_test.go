package transfer

import (
	"testing"

	"github.com/maxitosh/katapult/internal/domain"
)

func strPtr(s string) *string { return &s }

func TestSelectStrategy(t *testing.T) {
	tests := []struct {
		name             string
		sourceCluster    string
		destCluster      string
		strategyOverride *string
		s3Config         S3Config
		want             domain.TransferStrategy
		wantErr          bool
	}{
		{
			name:             "valid override stream",
			sourceCluster:    "a",
			destCluster:      "b",
			strategyOverride: strPtr("stream"),
			want:             domain.TransferStrategyStream,
		},
		{
			name:             "valid override s3",
			sourceCluster:    "a",
			destCluster:      "b",
			strategyOverride: strPtr("s3"),
			want:             domain.TransferStrategyS3,
		},
		{
			name:             "valid override direct",
			sourceCluster:    "a",
			destCluster:      "b",
			strategyOverride: strPtr("direct"),
			want:             domain.TransferStrategyDirect,
		},
		{
			name:             "invalid override",
			sourceCluster:    "a",
			destCluster:      "b",
			strategyOverride: strPtr("invalid"),
			wantErr:          true,
		},
		{
			name:          "same cluster selects stream",
			sourceCluster: "cluster-a",
			destCluster:   "cluster-a",
			want:          domain.TransferStrategyStream,
		},
		{
			name:          "cross-cluster with S3 selects s3",
			sourceCluster: "cluster-a",
			destCluster:   "cluster-b",
			s3Config:      S3Config{Configured: true},
			want:          domain.TransferStrategyS3,
		},
		{
			name:          "cross-cluster without S3 selects direct",
			sourceCluster: "cluster-a",
			destCluster:   "cluster-b",
			s3Config:      S3Config{Configured: false},
			want:          domain.TransferStrategyDirect,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectStrategy(tt.sourceCluster, tt.destCluster, tt.strategyOverride, tt.s3Config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SelectStrategy() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("SelectStrategy() = %v, want %v", got, tt.want)
			}
		})
	}
}
