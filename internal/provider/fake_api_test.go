package provider

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const fakeWorkspaceID = "wsp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type object = map[string]any

// fakeAPI is an in-memory Management API that follows the documented
// semantics the provider relies on: create-only secrets, write-only
// connection secrets, soft-deleted projects and environments, revoke-only
// API keys, and strict request bodies.
type fakeAPI struct {
	mu      sync.Mutex
	seq     int
	objects map[string]object // id -> resource
	// connectionSecrets records the last provider_client_secret per connection.
	connectionSecrets map[string]string
	themes            map[string]object
	settings          map[string]bool
}

func newFakeAPI(t *testing.T) *fakeAPI {
	f := &fakeAPI{
		objects:           map[string]object{},
		connectionSecrets: map[string]string{},
		themes:            map[string]object{},
		settings:          map[string]bool{},
	}
	server := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(server.Close)
	t.Setenv("ZEROWORD_BASE_URL", server.URL+"/v1")
	t.Setenv("ZEROWORD_API_KEY", "sk_live_test")
	return f
}

// update changes a stored resource as if someone edited it outside Terraform.
func (f *fakeAPI) update(id string, change func(object)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change(f.objects[id])
}

func (f *fakeAPI) connectionSecret(id string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connectionSecrets[id]
}

func (f *fakeAPI) nextID(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s_%032d", prefix, f.seq)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func problem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(object{"status": status, "code": code, "detail": detail})
}

// decode reads a JSON body and rejects fields outside allowed, like the
// API's strict schemas.
func decode(r *http.Request, allowed ...string) (object, error) {
	body := object{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, err
	}
	for key := range body {
		if !strings.Contains(","+strings.Join(allowed, ",")+",", ","+key+",") {
			return nil, fmt.Errorf("unexpected field %q", key)
		}
	}
	return body, nil
}

func clone(o object) object {
	out := maps.Clone(o)
	for _, hidden := range []string{"secret", "client_secret"} {
		delete(out, hidden)
	}
	return out
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer sk_live_test" {
		problem(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/"), "/")
	collection, id, sub := parts[0], "", ""
	if len(parts) > 1 {
		id = parts[1]
	}
	if len(parts) > 2 {
		sub = parts[2]
	}
	switch {
	case collection == "workspaces" && r.Method == http.MethodGet && id == fakeWorkspaceID:
		writeJSON(w, 200, object{"id": fakeWorkspaceID, "name": "Staging", "slug": "staging", "status": "active"})
	case id == "" && r.Method == http.MethodPost:
		f.create(w, r, collection)
	case sub == "settings":
		f.environmentSettings(w, r, id)
	case sub == "theme":
		f.environmentTheme(w, r, id)
	case sub == "revoke" && r.Method == http.MethodPost:
		f.mutate(w, id, func(o object) { o["revoked_at"] = "2026-11-24T00:00:00.000Z" })
	case sub == "" && r.Method == http.MethodGet:
		o, ok := f.objects[id]
		if !ok {
			problem(w, 404, "not_found", "")
			return
		}
		writeJSON(w, 200, clone(o))
	case sub == "" && r.Method == http.MethodPatch:
		f.patch(w, r, collection, id)
	case sub == "" && r.Method == http.MethodDelete:
		f.delete(w, collection, id)
	default:
		problem(w, 404, "not_found", r.Method+" "+r.URL.Path)
	}
}

func (f *fakeAPI) mutate(w http.ResponseWriter, id string, change func(object)) {
	o, ok := f.objects[id]
	if !ok {
		problem(w, 404, "not_found", "")
		return
	}
	change(o)
	writeJSON(w, 200, clone(o))
}

func (f *fakeAPI) liveEnvironment(id string) bool {
	env, ok := f.objects[id]
	return ok && env["status"] != "deleted"
}

func (f *fakeAPI) create(w http.ResponseWriter, r *http.Request, collection string) {
	var (
		body object
		err  error
		o    object
	)
	switch collection {
	case "projects":
		if body, err = decode(r, "workspace_id", "name", "slug"); err == nil {
			o = object{"id": f.nextID("prj"), "deleted_at": nil, "slug": strings.ToLower(fmt.Sprint(body["name"]))}
		}
	case "environments":
		if body, err = decode(r, "project_id", "type", "name"); err == nil {
			envID := f.nextID("env")
			o = object{
				"id": envID, "status": "active", "issuer": "https://id.example/e/" + envID,
				"interaction_domain": envID + ".login.example", "rp_id": envID + ".login.example",
			}
			f.themes[envID] = object{"methods": []any{"email_otp", "passkey"}}
		}
	case "clients":
		if body, err = decode(r, "environment_id", "client_type", "name", "redirect_uris", "post_logout_redirect_uris",
			"allowed_origins", "allowed_scopes", "allowed_resources", "token_endpoint_auth_method", "require_pkce", "mobile_app"); err == nil {
			o = object{"id": f.nextID("cli"), "status": "active", "mobile_app": nil}
			if body["token_endpoint_auth_method"] != "none" {
				o["client_secret"] = "client-secret-" + fmt.Sprint(f.seq)
			}
		}
	case "connections":
		if body, err = decode(r, "environment_id", "provider", "mode", "provider_key", "issuer", "provider_client_id",
			"provider_client_secret", "scopes", "profile_mapping"); err == nil {
			o = object{"id": f.nextID("con"), "status": "active", "issuer": nil, "owner_confirmed": body["provider"] != "generic_oidc"}
		}
	case "api-keys":
		if body, err = decode(r, "workspace_id", "name", "scopes", "environment_id", "expires_at"); err == nil {
			keyID := f.nextID("mky")
			o = object{"id": keyID, "environment_id": nil, "expires_at": nil, "revoked_at": nil, "prefix": "sk_live_ab", "secret": "sk_live_secret_" + keyID}
		}
	case "domains":
		if body, err = decode(r, "environment_id", "hostname", "rp_enabled"); err == nil {
			o = object{
				"id": f.nextID("dom"), "status": "pending", "ownership_verified": false,
				"verification_record_name": "_zeroword." + fmt.Sprint(body["hostname"]), "verification_record_value": "token",
			}
		}
	case "webhooks":
		if body, err = decode(r, "environment_id", "url", "event_types"); err == nil {
			o = object{"id": f.nextID("whk"), "status": "active", "secret": "whsec_" + fmt.Sprint(f.seq)}
		}
	default:
		problem(w, 404, "not_found", collection)
		return
	}
	if err != nil {
		problem(w, 400, "invalid_request", err.Error())
		return
	}
	if envID, ok := body["environment_id"].(string); ok && collection != "api-keys" && !f.liveEnvironment(envID) {
		problem(w, 404, "not_found", "environment")
		return
	}
	if secret, ok := body["provider_client_secret"].(string); ok {
		f.connectionSecrets[o["id"].(string)] = secret
		delete(body, "provider_client_secret")
	}
	maps.Copy(o, body)
	f.objects[o["id"].(string)] = o
	writeJSON(w, 201, o)
}

func (f *fakeAPI) patch(w http.ResponseWriter, r *http.Request, collection, id string) {
	allowed := map[string][]string{
		"projects":     {"name", "slug"},
		"environments": {"name", "status"},
		"clients": {"name", "redirect_uris", "post_logout_redirect_uris", "allowed_origins", "allowed_scopes",
			"allowed_resources", "token_endpoint_auth_method", "require_pkce", "status", "mobile_app"},
		"connections": {"issuer", "provider_client_id", "provider_client_secret", "scopes", "profile_mapping", "status", "owner_confirmed"},
		"domains":     {"rp_enabled"},
		"webhooks":    {"url", "event_types", "status"},
	}[collection]
	body, err := decode(r, allowed...)
	if err != nil || allowed == nil {
		problem(w, 400, "invalid_request", fmt.Sprint(err))
		return
	}
	f.mutate(w, id, func(o object) {
		if secret, ok := body["provider_client_secret"].(string); ok {
			f.connectionSecrets[id] = secret
			delete(body, "provider_client_secret")
		}
		if issuer, ok := body["issuer"]; ok && o["provider"] == "generic_oidc" && issuer != o["issuer"] {
			o["owner_confirmed"] = false
		}
		maps.Copy(o, body)
	})
}

func (f *fakeAPI) delete(w http.ResponseWriter, collection, id string) {
	o, ok := f.objects[id]
	if !ok {
		problem(w, 404, "not_found", "")
		return
	}
	switch collection {
	case "projects":
		for _, other := range f.objects {
			if other["project_id"] == id && other["status"] != "deleted" {
				problem(w, 409, "project_has_live_environments", "")
				return
			}
		}
		o["deleted_at"] = "2026-11-24T00:00:00.000Z"
	case "environments":
		o["status"] = "deleted"
	default:
		delete(f.objects, id)
	}
	writeJSON(w, 200, object{"id": id, "deleted": true})
}

func (f *fakeAPI) environmentSettings(w http.ResponseWriter, r *http.Request, envID string) {
	if !f.liveEnvironment(envID) {
		problem(w, 404, "not_found", "")
		return
	}
	if r.Method == http.MethodPatch {
		body, err := decode(r, "impersonation_enabled")
		if err != nil {
			problem(w, 400, "invalid_request", err.Error())
			return
		}
		f.settings[envID] = body["impersonation_enabled"].(bool)
	}
	writeJSON(w, 200, object{"environment_id": envID, "impersonation_enabled": f.settings[envID]})
}

func (f *fakeAPI) environmentTheme(w http.ResponseWriter, r *http.Request, envID string) {
	if !f.liveEnvironment(envID) {
		problem(w, 404, "not_found", "")
		return
	}
	if r.Method == http.MethodPut {
		body := object{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			problem(w, 400, "invalid_theme", err.Error())
			return
		}
		f.themes[envID] = body
		f.seq++
	}
	writeJSON(w, 200, object{"environment_id": envID, "version": f.seq, "theme": f.themes[envID]})
}
