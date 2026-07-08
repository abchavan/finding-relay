package relay

import "strings"

// ParseSuppress turns a comma-separated list of fingerprints into a set.
func ParseSuppress(s string) map[string]bool {
	set := map[string]bool{}
	for _, fp := range strings.Split(s, ",") {
		if fp = strings.TrimSpace(fp); fp != "" {
			set[fp] = true
		}
	}
	return set
}
