package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Admin struct {
	issuer, clientID, secret string
	client                   *http.Client
}

type role struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewAdmin(issuer, clientID, secret string) *Admin {
	if issuer == "" || clientID == "" || secret == "" {
		return nil
	}
	return &Admin{strings.TrimRight(issuer, "/"), clientID, secret, &http.Client{Timeout: 8 * time.Second}}
}

func (a *Admin) token(ctx context.Context) (string, error) {
	v := url.Values{"grant_type": {"client_credentials"}, "client_id": {a.clientID}, "client_secret": {a.secret}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.issuer+"/protocol/openid-connect/token", strings.NewReader(v.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Keycloak token status %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", errors.New("Keycloak returned no access token")
	}
	return out.AccessToken, nil
}

func (a *Admin) endpoint(path string) string {
	base := strings.TrimSuffix(a.issuer, "/realms/"+a.realm())
	return base + "/admin/realms/" + url.PathEscape(a.realm()) + path
}

func (a *Admin) realm() string {
	i := strings.LastIndex(a.issuer, "/realms/")
	if i < 0 {
		return ""
	}
	return a.issuer[i+8:]
}

func (a *Admin) request(ctx context.Context, method, path string, body any, out any) error {
	token, err := a.token(ctx)
	if err != nil {
		return err
	}
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.endpoint(path), payload)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Keycloak Admin API %s status %d", method, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
	}
	return nil
}

func (a *Admin) Roles(ctx context.Context, sub string) (map[string]bool, error) {
	if sub == "" {
		return nil, errors.New("missing Keycloak user ID")
	}
	var items []role
	err := a.request(ctx, http.MethodGet, "/users/"+url.PathEscape(sub)+"/role-mappings/realm/composite", nil, &items)
	if err != nil {
		return nil, err
	}
	roles := make(map[string]bool, len(items))
	for _, item := range items {
		roles[item.Name] = true
	}
	return roles, nil
}

func (a *Admin) realmRole(ctx context.Context, name string) (role, error) {
	var item role
	err := a.request(ctx, http.MethodGet, "/roles/"+url.PathEscape(name), nil, &item)
	return item, err
}

func (a *Admin) SetPlan(ctx context.Context, sub, plan string) error {
	if sub == "" || (plan != "free" && plan != "paid") {
		return errors.New("invalid plan change")
	}
	path := "/users/" + url.PathEscape(sub) + "/role-mappings/realm"
	// Free is a default realm role. Paid overrides it; an inherited Free role
	// cannot be removed from an individual user.
	paid, err := a.realmRole(ctx, "chatbox-paid")
	if err != nil {
		return err
	}
	if plan == "paid" {
		if err := a.request(ctx, http.MethodPost, path, []role{paid}, nil); err != nil {
			return err
		}
		return a.verifyPlan(ctx, sub, plan)
	}
	free, err := a.realmRole(ctx, "chatbox-free")
	if err != nil {
		return err
	}
	if err := a.request(ctx, http.MethodPost, path, []role{free}, nil); err != nil {
		return err
	}
	if err := a.request(ctx, http.MethodDelete, path, []role{paid}, nil); err != nil {
		return err
	}
	return a.verifyPlan(ctx, sub, plan)
}

func (a *Admin) verifyPlan(ctx context.Context, sub, plan string) error {
	roles, err := a.Roles(ctx, sub)
	if err != nil {
		return err
	}
	if roles["chatbox-paid"] != (plan == "paid") {
		return errors.New("Keycloak plan role did not change")
	}
	return nil
}

func (a *Admin) AssignRole(ctx context.Context, sub, name string) error {
	if sub == "" {
		return errors.New("missing Keycloak user ID")
	}
	r, err := a.realmRole(ctx, name)
	if err != nil {
		return err
	}
	return a.request(ctx, http.MethodPost, "/users/"+url.PathEscape(sub)+"/role-mappings/realm", []role{r}, nil)
}
