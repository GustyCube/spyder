package api

import (
	"testing"

	"github.com/gustycube/spyder/internal/config"
)

func TestKeyStoreLookup(t *testing.T) {
	keys := []config.APIKey{
		{Key: "sk_admin_123", Scope: "admin", Name: "admin"},
		{Key: "sk_read_456", Scope: "read", Name: "reader"},
	}
	ks := NewKeyStore(keys)

	key, ok := ks.Lookup("sk_admin_123")
	if !ok {
		t.Fatal("key not found")
	}
	if key.Scope != "admin" {
		t.Errorf("scope = %q", key.Scope)
	}

	_, ok = ks.Lookup("invalid")
	if ok {
		t.Error("should not find invalid key")
	}
}

func TestScopeHierarchy(t *testing.T) {
	if !ScopeAllows("admin", "read") {
		t.Error("admin should allow read")
	}
	if !ScopeAllows("admin", "write") {
		t.Error("admin should allow write")
	}
	if !ScopeAllows("write", "read") {
		t.Error("write should allow read")
	}
	if !ScopeAllows("write", "write") {
		t.Error("write should allow write")
	}
	if ScopeAllows("write", "admin") {
		t.Error("write should not allow admin")
	}
	if ScopeAllows("read", "write") {
		t.Error("read should not allow write")
	}
}

func TestKeyStoreReload(t *testing.T) {
	ks := NewKeyStore([]config.APIKey{{Key: "k1", Scope: "admin"}})
	if _, ok := ks.Lookup("k1"); !ok {
		t.Fatal("k1 not found")
	}

	ks.Reload([]config.APIKey{{Key: "k2", Scope: "read"}})
	if _, ok := ks.Lookup("k1"); ok {
		t.Error("k1 should be gone after reload")
	}
	if _, ok := ks.Lookup("k2"); !ok {
		t.Error("k2 not found after reload")
	}
}

func TestKeyStoreAddRemove(t *testing.T) {
	ks := NewKeyStore(nil)
	ks.Add(config.APIKey{Key: "new", Scope: "write", Name: "test"})
	if k, ok := ks.Lookup("new"); !ok || k.Scope != "write" {
		t.Error("add failed")
	}

	ks.Remove("new")
	if _, ok := ks.Lookup("new"); ok {
		t.Error("remove failed")
	}
}

func TestKeyStoreList(t *testing.T) {
	ks := NewKeyStore([]config.APIKey{
		{Key: "a", Scope: "admin"},
		{Key: "b", Scope: "read"},
	})
	list := ks.List()
	if len(list) != 2 {
		t.Errorf("list len = %d", len(list))
	}
}
