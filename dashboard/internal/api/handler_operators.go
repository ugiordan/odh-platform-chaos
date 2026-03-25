package api

import "net/http"

func (s *Server) handleListOperators(w http.ResponseWriter, r *http.Request) {
	ops, err := s.store.ListOperators(nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ops)
}

func (s *Server) handleListComponents(w http.ResponseWriter, r *http.Request) {
	operator := pathSegment(r, "operator")
	exps, err := s.store.ListByOperator(operator, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	seen := map[string]bool{}
	var components []string
	for _, e := range exps {
		if !seen[e.Component] {
			seen[e.Component] = true
			components = append(components, e.Component)
		}
	}
	writeJSON(w, http.StatusOK, components)
}
