package tempo

import "strings"

// relativeNext converts Tempo's absolute `metadata.next` URL into a path
// relative to baseURL so it can be passed to newRequest. It returns "" when
// there is no next page.
func relativeNext(baseURL, next string) string {
	if next == "" {
		return ""
	}
	if strings.HasPrefix(next, baseURL) {
		return strings.TrimPrefix(next, baseURL)
	}
	// Fall back to using the absolute URL's path+query if it is on another host
	// (should not normally happen); as a last resort return "" to stop paging.
	if i := strings.Index(next, "://"); i >= 0 {
		rest := next[i+3:]
		if slash := strings.Index(rest, "/"); slash >= 0 {
			return rest[slash:]
		}
	}
	return ""
}
