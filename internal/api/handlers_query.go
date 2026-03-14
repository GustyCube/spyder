package api

import (
	"net/http"
)

func (s *Server) handleListDomains(w http.ResponseWriter, r *http.Request) {
	cursor, limit := parsePagination(r)
	filter := DomainFilter{
		Apex:   r.URL.Query().Get("apex"),
		Search: r.URL.Query().Get("search"),
		Cursor: cursor,
		Limit:  limit,
	}

	page, err := s.QueryStore.ListDomains(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleGetDomain(w http.ResponseWriter, r *http.Request) {
	host := r.PathValue("host")
	if host == "" {
		writeError(w, http.StatusBadRequest, "MISSING_HOST", "host is required")
		return
	}

	detail, err := s.QueryStore.GetDomain(r.Context(), host)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_ERROR", err.Error())
		return
	}
	if detail == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "domain not found")
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleListEdges(w http.ResponseWriter, r *http.Request) {
	cursor, limit := parsePagination(r)
	filter := EdgeFilter{
		Type:   r.URL.Query().Get("type"),
		Source: r.URL.Query().Get("source"),
		Target: r.URL.Query().Get("target"),
		Cursor: cursor,
		Limit:  limit,
	}

	page, err := s.QueryStore.ListEdges(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleListIPs(w http.ResponseWriter, r *http.Request) {
	cursor, limit := parsePagination(r)
	filter := IPFilter{
		Search: r.URL.Query().Get("search"),
		Cursor: cursor,
		Limit:  limit,
	}

	page, err := s.QueryStore.ListIPs(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleListCerts(w http.ResponseWriter, r *http.Request) {
	cursor, limit := parsePagination(r)
	filter := CertFilter{
		Search: r.URL.Query().Get("search"),
		Cursor: cursor,
		Limit:  limit,
	}

	page, err := s.QueryStore.ListCerts(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}
