package syncer

import (
	"regexp"
	"strings"
)

// homeRegex matches file:// URIs pointing into /Users/<user> or /home/<user>.
// Group 1 matches the "/Users/<user>" or "/home/<user>" prefix.
// Group 2 matches the remainder of the path (e.g., "/Projects/foo").
var homeRegex = regexp.MustCompile(`^file://(/Users/[^/]+|/home/[^/]+)(/.*)?$`)

// AdaptWorkspaceURIs adapts workspace URIs from a source machine to the destination machine's home directory.
// If an entry is a file:// URI pointing to a macOS (/Users/<user>) or Linux (/home/<user>) home path,
// the user home prefix is replaced with destHome while preserving the subpath.
func AdaptWorkspaceURIs(uris []string, destHome string) []string {
	if len(uris) == 0 || strings.TrimSpace(destHome) == "" {
		return uris
	}

	cleanDestHome := strings.TrimRight(destHome, "/")
	adapted := make([]string, len(uris))

	for i, uri := range uris {
		matches := homeRegex.FindStringSubmatch(uri)
		if len(matches) >= 2 {
			remainder := ""
			if len(matches) > 2 {
				remainder = matches[2]
			}
			adapted[i] = "file://" + cleanDestHome + remainder
		} else {
			adapted[i] = uri
		}
	}

	return adapted
}
