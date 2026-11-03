package keycloak

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetPlanUsesServiceAccountAndRealmRoleMappings(t *testing.T) {
	var calls []string
	paid := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/realms/chatbox/protocol/openid-connect/token" {
			if r.FormValue("grant_type") != "client_credentials" || r.FormValue("client_id") != "backend" || r.FormValue("client_secret") != "secret" {
				t.Error("bad client credentials")
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "token"})
			return
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Error("missing admin token")
		}
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/admin/realms/chatbox/roles/chatbox-paid":
			_ = json.NewEncoder(w).Encode(role{ID: "paid-id", Name: "chatbox-paid"})
		case "/admin/realms/chatbox/roles/chatbox-free":
			_ = json.NewEncoder(w).Encode(role{ID: "free-id", Name: "chatbox-free"})
		case "/admin/realms/chatbox/users/user-1/role-mappings/realm":
			if r.Method == http.MethodPost {
				paid = true
			}
			if r.Method == http.MethodDelete {
				paid = false
			}
			w.WriteHeader(http.StatusNoContent)
		case "/admin/realms/chatbox/users/user-1/role-mappings/realm/composite":
			roles := []role{{ID: "free-id", Name: "chatbox-free"}}
			if paid {
				roles = append(roles, role{ID: "paid-id", Name: "chatbox-paid"})
			}
			_ = json.NewEncoder(w).Encode(roles)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	a := NewAdmin(srv.URL+"/realms/chatbox", "backend", "secret")
	if err := a.SetPlan(context.Background(), "user-1", "paid"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || calls[0] != "GET /admin/realms/chatbox/roles/chatbox-paid" || calls[1] != "POST /admin/realms/chatbox/users/user-1/role-mappings/realm" || calls[2] != "GET /admin/realms/chatbox/users/user-1/role-mappings/realm/composite" {
		t.Fatalf("paid calls: %v", calls)
	}
	calls = nil
	if err := a.SetPlan(context.Background(), "user-1", "free"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 5 || calls[3] != "DELETE /admin/realms/chatbox/users/user-1/role-mappings/realm" || calls[4] != "GET /admin/realms/chatbox/users/user-1/role-mappings/realm/composite" {
		t.Fatalf("free calls: %v", calls)
	}
}
