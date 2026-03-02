package registry

import (
	"testing"

	"github.com/maxitosh/katapult/internal/domain"
)

func TestValidateTools_AllPresent(t *testing.T) {
	tools := domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: "5.72"}
	if err := ValidateTools(tools); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateTools_MinimumTar(t *testing.T) {
	tools := domain.ToolVersions{Tar: "1.28", Zstd: "1.5.5", Stunnel: "5.72"}
	if err := ValidateTools(tools); err != nil {
		t.Fatalf("tar 1.28 should be accepted, got: %v", err)
	}
}

func TestValidateTools_TarTooOld(t *testing.T) {
	tools := domain.ToolVersions{Tar: "1.27", Zstd: "1.5.5", Stunnel: "5.72"}
	if err := ValidateTools(tools); err == nil {
		t.Fatal("expected error for tar < 1.28")
	}
}

func TestValidateTools_MissingZstd(t *testing.T) {
	tools := domain.ToolVersions{Tar: "1.35", Zstd: "", Stunnel: "5.72"}
	if err := ValidateTools(tools); err == nil {
		t.Fatal("expected error for missing zstd")
	}
}

func TestValidateTools_MissingStunnel(t *testing.T) {
	tools := domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: ""}
	if err := ValidateTools(tools); err == nil {
		t.Fatal("expected error for missing stunnel")
	}
}

func TestValidateTools_AllMissing(t *testing.T) {
	tools := domain.ToolVersions{}
	err := ValidateTools(tools)
	if err == nil {
		t.Fatal("expected error for all tools missing")
	}
}

func TestValidateTools_TarWithPatchVersion(t *testing.T) {
	tools := domain.ToolVersions{Tar: "1.28.1", Zstd: "1.5.5", Stunnel: "5.72"}
	if err := ValidateTools(tools); err != nil {
		t.Fatalf("tar 1.28.1 should be accepted, got: %v", err)
	}
}
