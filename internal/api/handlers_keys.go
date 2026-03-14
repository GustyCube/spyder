package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gustycube/spyder/internal/config"
)

func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	keys := s.KeyStore.List()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"keys": keys,
	})
}

func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Scope string `json:"scope"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "MISSING_NAME", "name is required")
		return
	}
	if req.Scope == "" {
		req.Scope = "read"
	}
	if req.Scope != "read" && req.Scope != "write" && req.Scope != "admin" {
		writeError(w, http.StatusBadRequest, "INVALID_SCOPE", "scope must be read, write, or admin")
		return
	}

	// Generate a random API key
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		writeError(w, http.StatusInternalServerError, "KEY_GEN_ERROR", "failed to generate key")
		return
	}
	key := "spyder_" + hex.EncodeToString(b)

	s.KeyStore.Add(config.APIKey{Key: key, Name: req.Name, Scope: req.Scope})

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"key":   key,
		"name":  req.Name,
		"scope": req.Scope,
	})
}

func (s *Server) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "MISSING_KEY", "key is required")
		return
	}

	if _, exists := s.KeyStore.Lookup(key); !exists {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "key not found")
		return
	}
	s.KeyStore.Remove(key)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": true,
	})
}
