package agent

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/chainstack/katapult/internal/domain"
)

var tarVersionRe = regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?)`)

// VerifyTools checks that required tools (tar, zstd, stunnel) are available
// on the system and returns their versions.
func VerifyTools() (domain.ToolVersions, error) {
	var tools domain.ToolVersions
	var errs []string

	tarVer, err := getTarVersion()
	if err != nil {
		errs = append(errs, fmt.Sprintf("tar: %v", err))
	} else {
		tools.Tar = tarVer
	}

	zstdVer, err := getToolVersion("zstd", "--version")
	if err != nil {
		errs = append(errs, fmt.Sprintf("zstd: %v", err))
	} else {
		tools.Zstd = zstdVer
	}

	stunnelVer, err := getToolVersion("stunnel", "-version")
	if err != nil {
		errs = append(errs, fmt.Sprintf("stunnel: %v", err))
	} else {
		tools.Stunnel = stunnelVer
	}

	if len(errs) > 0 {
		return tools, fmt.Errorf("missing required tools: %s", strings.Join(errs, "; "))
	}
	return tools, nil
}

func getTarVersion() (string, error) {
	out, err := exec.Command("tar", "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("not found or not executable: %w", err)
	}
	matches := tarVersionRe.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		return "", fmt.Errorf("could not parse version from output")
	}
	return matches[1], nil
}

func getToolVersion(name string, args ...string) (string, error) {
	// stunnel -version writes to stderr, so use CombinedOutput.
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		// Some tools return non-zero for --version / -version; check if output is present.
		if len(out) == 0 {
			return "", fmt.Errorf("not found or not executable: %w", err)
		}
	}
	matches := tarVersionRe.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		return "", fmt.Errorf("could not parse version from output")
	}
	return matches[1], nil
}
