package main

import (
	"context"
	"net/http"
	"strings"
)

type retrievalScopeKey struct{}
type retrievalScope struct{ IDs []string }

func withMaterials(r *http.Request, ids []string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), retrievalScopeKey{}, retrievalScope{ids}))
}
func materialScope(r *http.Request) ([]string, bool) {
	v, ok := r.Context().Value(retrievalScopeKey{}).(retrievalScope)
	return v.IDs, ok
}
func scopeSQL(r *http.Request) (string, []any) {
	ids, ok := materialScope(r)
	if !ok {
		return "", nil
	}
	if len(ids) == 0 {
		return " AND 1=0", nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return " AND m.id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + ")", args
}
func allowedMaterial(r *http.Request, id string) bool {
	ids, ok := materialScope(r)
	if !ok {
		return true
	}
	for _, v := range ids {
		if id == v {
			return true
		}
	}
	return false
}
