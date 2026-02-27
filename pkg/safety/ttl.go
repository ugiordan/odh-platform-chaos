package safety

import "time"

// TTLAnnotationKey is the annotation key used to store the expiration timestamp on chaos-injected resources.
const TTLAnnotationKey = "chaos.opendatahub.io/expires"

// TTLExpiry returns an RFC3339 timestamp representing now plus the given duration.
func TTLExpiry(ttl time.Duration) string {
	return time.Now().Add(ttl).Format(time.RFC3339)
}

// IsExpired returns true if the given RFC3339 expiry string is in the past or malformed.
func IsExpired(expiryStr string) bool {
	expiry, err := time.Parse(time.RFC3339, expiryStr)
	if err != nil {
		return true // malformed = treat as expired
	}
	return time.Now().After(expiry)
}
