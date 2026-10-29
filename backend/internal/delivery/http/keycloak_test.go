package httpdelivery

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestKeycloakIDTokenVerification(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	enc := func(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }
	issuer := "https://auth.example.test/realms/chatbox"
	verifier := newKeycloakVerifier(issuer, "chatbox-web")
	public := map[string]any{"keys": []any{map[string]any{"kid": "key-1", "kty": "RSA", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}}
	body, _ := json.Marshal(public)
	verifier.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != issuer+"/protocol/openid-connect/certs" {
			t.Errorf("unexpected JWKS URL: %s", req.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})
	claims := map[string]any{"iss": issuer, "sub": "subject-1", "aud": "chatbox-web", "exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Unix(), "nonce": "nonce-1", "email_verified": true, "preferred_username": "alice", "given_name": "An", "family_name": "Nguyen"}
	sign := func(c map[string]any) string {
		input := enc(map[string]any{"alg": "RS256", "kid": "key-1"}) + "." + enc(c)
		h := sha256.Sum256([]byte(input))
		sig, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
		if e != nil {
			t.Fatal(e)
		}
		return input + "." + base64.RawURLEncoding.EncodeToString(sig)
	}
	identity, err := verifier.verifyIDToken(sign(claims), "nonce-1")
	if err != nil || identity.Subject != "subject-1" || identity.Username != "alice" || identity.DisplayName != "An Nguyen" {
		t.Fatalf("valid token rejected: %+v %v", identity, err)
	}
	if _, err := verifier.verifyIDToken(sign(claims), "other-nonce"); err == nil {
		t.Fatal("wrong nonce accepted")
	}
	claims["aud"] = "other-client"
	if _, err := verifier.verifyIDToken(sign(claims), "nonce-1"); err == nil {
		t.Fatal("wrong audience accepted")
	}
	claims["aud"] = "chatbox-web"
	forged := sign(claims)
	mark := strings.LastIndex(forged, ".") + 1
	if forged[mark] == 'A' {
		forged = forged[:mark] + "B" + forged[mark+1:]
	} else {
		forged = forged[:mark] + "A" + forged[mark+1:]
	}
	if _, err := verifier.verifyIDToken(forged, "nonce-1"); err == nil {
		t.Fatal("invalid signature accepted")
	}
}
