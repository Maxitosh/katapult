package registry

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/chainstack/katapult/internal/domain"
)

const minTarMajor = 1
const minTarMinor = 28

// ValidateTools checks that all required tools meet minimum version requirements.
// Returns an error describing any missing or insufficient tools.
func ValidateTools(tools domain.ToolVersions) error {
	var errs []string

	if tools.Tar == "" {
		errs = append(errs, "tar: not present")
	} else if err := validateTarVersion(tools.Tar); err != nil {
		errs = append(errs, fmt.Sprintf("tar: %v", err))
	}

	if tools.Zstd == "" {
		errs = append(errs, "zstd: not present")
	}

	if tools.Stunnel == "" {
		errs = append(errs, "stunnel: not present")
	}

	if len(errs) > 0 {
		return fmt.Errorf("tool verification failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// validateTarVersion checks that tar version is >= 1.28.
// Accepts formats like "1.28", "1.35", "1.28.1".
func validateTarVersion(version string) error {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return fmt.Errorf("invalid version format %q", version)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid major version %q", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid minor version %q", parts[1])
	}

	if major < minTarMajor || (major == minTarMajor && minor < minTarMinor) {
		return fmt.Errorf("version %s is below minimum %d.%d", version, minTarMajor, minTarMinor)
	}

	return nil
}
