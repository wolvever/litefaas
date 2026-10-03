package manifest

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// projectNameRE matches a single resource name (same shape as the API).
var projectNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// ProjectID is the edge-rule provenance key for a deploy.
// The daemon does not invent this: the CLI sends resource names from the
// manifest. Names are sorted so api+web and web+api are one project.
func ProjectID(names []string) (string, error) {
	seen := map[string]struct{}{}
	var ids []string
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		if !projectNameRE.MatchString(n) {
			return "", fmt.Errorf("project id: invalid resource name %q", n)
		}
		seen[n] = struct{}{}
		ids = append(ids, n)
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("project id: no resource names")
	}
	sort.Strings(ids)
	id := strings.Join(ids, "+")
	if len(id) > 512 {
		return "", fmt.Errorf("project id: too long")
	}
	return id, nil
}
