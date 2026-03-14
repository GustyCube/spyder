package api

import (
	"sync"

	"github.com/gustycube/spyder/internal/config"
)

var scopeLevel = map[string]int{"read": 1, "write": 2, "admin": 3}

func ScopeAllows(have, need string) bool {
	return scopeLevel[have] >= scopeLevel[need]
}

type KeyStore struct {
	mu   sync.RWMutex
	keys map[string]config.APIKey
}

func NewKeyStore(keys []config.APIKey) *KeyStore {
	ks := &KeyStore{keys: make(map[string]config.APIKey)}
	for _, k := range keys {
		ks.keys[k.Key] = k
	}
	return ks
}

func (ks *KeyStore) Lookup(key string) (config.APIKey, bool) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	k, ok := ks.keys[key]
	return k, ok
}

func (ks *KeyStore) Reload(keys []config.APIKey) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.keys = make(map[string]config.APIKey)
	for _, k := range keys {
		ks.keys[k.Key] = k
	}
}

func (ks *KeyStore) Add(k config.APIKey) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.keys[k.Key] = k
}

func (ks *KeyStore) Remove(key string) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	delete(ks.keys, key)
}

func (ks *KeyStore) List() []config.APIKey {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	result := make([]config.APIKey, 0, len(ks.keys))
	for _, k := range ks.keys {
		result = append(result, k)
	}
	return result
}
