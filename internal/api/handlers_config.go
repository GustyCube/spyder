package api

import (
	"net/http"

	"github.com/gustycube/spyder/internal/config"
)

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.RuntimeCfg.Get())
}

func (s *Server) handlePatchConfig(w http.ResponseWriter, r *http.Request) {
	var patch config.ConfigPatch
	if err := readJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	warnings, err := s.RuntimeCfg.Apply(patch)
	if err != nil {
		writeError(w, http.StatusConflict, "APPLY_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"config":   s.RuntimeCfg.Get(),
		"warnings": warnings,
	})
}
