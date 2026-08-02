package cache

import (
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// Entry holds the validated API key data in the local LRU.
type Entry struct {
	AccountID string
	KeyDigest string // SHA-256 hex of the secret portion, used to detect key rotation
	ExpiresAt time.Time
}

// LocalCache is a fixed-size in-process LRU. Losing entries is fine — the next
// layer (Redis) is still warm. The soft TTL is intentionally shorter than the
// Redis hard TTL so stale data doesn't linger in-process.
type LocalCache struct {
	lru *lru.Cache[string, Entry]
}

// NewLocalCache creates an LRU sized to hold up to maxEntries active API keys.
// 10 000 entries ≈ ~5 MB at ~500 bytes per entry — safe to run in a pod.
func NewLocalCache(maxEntries int) (*LocalCache, error) {
	l, err := lru.New[string, Entry](maxEntries)
	if err != nil {
		return nil, err
	}
	return &LocalCache{lru: l}, nil
}

// Get returns the cached account ID if the entry exists, is not expired, and
// the stored digest matches the digest of the incoming key material.
// Returns ("", false) on any miss — never errors, always falls through.
func (c *LocalCache) Get(key, digest string) (accountID string, ok bool) {
	e, found := c.lru.Get(key)
	if !found {
		return "", false
	}
	if time.Now().After(e.ExpiresAt) || e.KeyDigest != digest {
		c.lru.Remove(key)
		return "", false
	}
	return e.AccountID, true
}

// Set stores a validated entry in the LRU with the given TTL.
func (c *LocalCache) Set(key, accountID, digest string, ttl time.Duration) {
	c.lru.Add(key, Entry{
		AccountID: accountID,
		KeyDigest: digest,
		ExpiresAt: time.Now().Add(ttl),
	})
}

// Invalidate removes a key from the LRU (e.g. on revocation).
func (c *LocalCache) Invalidate(key string) {
	c.lru.Remove(key)
}
