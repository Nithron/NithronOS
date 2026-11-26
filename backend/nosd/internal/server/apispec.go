package server

import (
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"nithronos/backend/nosd/internal/version"
)

// APIRoute is one method and path the router serves.
type APIRoute struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// chiParam matches a chi path parameter, with an optional regexp: {id} or
// {id:[0-9]+}.
var chiParam = regexp.MustCompile(`\{([^}:]+)(:[^}]*)?\}`)

// ListRoutes returns every route of r, sorted, with parameters written as
// {name}. It is the source of truth for the API description: the served
// OpenAPI document and docs/api/openapi.json are generated from it.
func ListRoutes(r chi.Routes) []APIRoute {
	seen := map[APIRoute]bool{}
	var out []APIRoute
	_ = chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		p := chiParam.ReplaceAllString(route, "{$1}")
		if len(p) > 1 {
			p = strings.TrimSuffix(p, "/")
		}
		rt := APIRoute{Method: method, Path: p}
		if !seen[rt] {
			seen[rt] = true
			out = append(out, rt)
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

// OpenAPIDocument describes routes as an OpenAPI 3 document. It lists every
// operation the server really has (and nothing else); request and response
// schemas are not described yet.
func OpenAPIDocument(routes []APIRoute) map[string]any {
	paths := map[string]map[string]any{}
	for _, rt := range routes {
		if strings.Contains(rt.Path, "*") {
			continue // mounted catch-alls (WebDAV, static files)
		}
		item := paths[rt.Path]
		if item == nil {
			item = map[string]any{}
			paths[rt.Path] = item
		}
		op := map[string]any{
			"operationId": operationID(rt),
			"tags":        []string{apiArea(rt.Path)},
			"responses":   map[string]any{"default": map[string]any{"description": "See docs/api/versioning-and-errors.md for the error format."}},
		}
		if params := pathParams(rt.Path); len(params) > 0 {
			op["parameters"] = params
		}
		item[strings.ToLower(rt.Method)] = op
	}
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "NithronOS API",
			"version":     version.Version,
			"description": "Generated from the nosd router (cmd/route-dump -format openapi); do not edit by hand.",
		},
		"paths": paths,
	}
}

// apiArea is the first path segment after /api/v1 (or the first segment).
func apiArea(p string) string {
	seg := strings.Split(strings.TrimPrefix(strings.TrimPrefix(p, "/api/v1"), "/api"), "/")
	for _, s := range seg {
		if s != "" && !strings.HasPrefix(s, "{") {
			return s
		}
	}
	return "root"
}

func operationID(rt APIRoute) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(rt.Method))
	for _, s := range strings.Split(strings.TrimPrefix(rt.Path, "/api/v1"), "/") {
		s = strings.Trim(s, "{}")
		for _, w := range strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' || r == '.' }) {
			b.WriteString(strings.ToUpper(w[:1]) + w[1:])
		}
	}
	return b.String()
}

func pathParams(p string) []map[string]any {
	var out []map[string]any
	for _, m := range chiParam.FindAllStringSubmatch(p, -1) {
		out = append(out, map[string]any{"name": m[1], "in": "path", "required": true, "schema": map[string]any{"type": "string"}})
	}
	return out
}
