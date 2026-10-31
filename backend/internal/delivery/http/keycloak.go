package httpdelivery

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"
)

type keycloakVerifier struct {
	issuer   string
	clientID string
	client   *http.Client
}

type keycloakIdentity struct{ Subject, Username, DisplayName, Email, FirstName, LastName string }

func newKeycloakVerifier(issuer, clientID string) *keycloakVerifier {
	if issuer == "" || clientID == "" {
		return nil
	}
	return &keycloakVerifier{issuer: strings.TrimRight(issuer, "/"), clientID: clientID, client: &http.Client{Timeout: 5 * time.Second}}
}

func jwtPart(part string, out any) error {
	b, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func (v *keycloakVerifier) verifyIDToken(token, expectedNonce string) (keycloakIdentity, error) {
	return v.verifyToken(token, expectedNonce, false)
}

func (v *keycloakVerifier) verifyAccessToken(token string) (keycloakIdentity, error) {
	return v.verifyToken(token, "", true)
}

func (v *keycloakVerifier) verifyToken(token, expectedNonce string, access bool) (keycloakIdentity, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(token) > 16000 || (!access && expectedNonce == "") {
		return keycloakIdentity{}, errors.New("invalid id token")
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if jwtPart(parts[0], &header) != nil || header.Alg != "RS256" || header.Kid == "" {
		return keycloakIdentity{}, errors.New("unsupported token signature")
	}
	var claims struct {
		Issuer            string          `json:"iss"`
		Subject           string          `json:"sub"`
		Audience          json.RawMessage `json:"aud"`
		Expires           int64           `json:"exp"`
		Issued            int64           `json:"iat"`
		Nonce             string          `json:"nonce"`
		PreferredUsername string          `json:"preferred_username"`
		GivenName         string          `json:"given_name"`
		FamilyName        string          `json:"family_name"`
		Name              string          `json:"name"`
		Email             string          `json:"email"`
		AuthorizedParty   string          `json:"azp"`
		Type              string          `json:"typ"`
	}
	if jwtPart(parts[1], &claims) != nil {
		return keycloakIdentity{}, errors.New("invalid token claims")
	}
	now := time.Now().Unix()
	if claims.Issuer != v.issuer || claims.Subject == "" || (!access && claims.Nonce != expectedNonce) || claims.Expires <= now || claims.Issued > now+60 || claims.Issued <= 0 {
		return keycloakIdentity{}, errors.New("token claims rejected")
	}
	if access && (claims.AuthorizedParty != v.clientID || claims.Type != "Bearer") {
		return keycloakIdentity{}, errors.New("wrong token client or type")
	}
	if !access {
		var audience string
		if json.Unmarshal(claims.Audience, &audience) == nil {
			if audience != v.clientID {
				return keycloakIdentity{}, errors.New("wrong token audience")
			}
		} else {
			var audiences []string
			if json.Unmarshal(claims.Audience, &audiences) != nil {
				return keycloakIdentity{}, errors.New("wrong token audience")
			}
			found := false
			for _, a := range audiences {
				if a == v.clientID {
					found = true
				}
			}
			if !found {
				return keycloakIdentity{}, errors.New("wrong token audience")
			}
		}
	}
	req, err := http.NewRequest(http.MethodGet, v.issuer+"/protocol/openid-connect/certs", nil)
	if err != nil {
		return keycloakIdentity{}, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return keycloakIdentity{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return keycloakIdentity{}, fmt.Errorf("JWKS status %d", resp.StatusCode)
	}
	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&jwks) != nil {
		return keycloakIdentity{}, errors.New("invalid JWKS")
	}
	for _, key := range jwks.Keys {
		if key.Kid != header.Kid || key.Kty != "RSA" || (key.Alg != "" && key.Alg != "RS256") {
			continue
		}
		n, e1 := base64.RawURLEncoding.DecodeString(key.N)
		e, e2 := base64.RawURLEncoding.DecodeString(key.E)
		sig, e3 := base64.RawURLEncoding.DecodeString(parts[2])
		if e1 != nil || e2 != nil || e3 != nil || len(e) == 0 || len(e) > 4 {
			return keycloakIdentity{}, errors.New("invalid signing key")
		}
		exponent := 0
		for _, b := range e {
			exponent = exponent<<8 | int(b)
		}
		if exponent < 3 || len(n) < 256 {
			return keycloakIdentity{}, errors.New("invalid signing key")
		}
		pub := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}
		hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		if rsa.VerifyPKCS1v15(pub, crypto.SHA256, hash[:], sig) != nil {
			return keycloakIdentity{}, errors.New("invalid token signature")
		}
		displayName := strings.TrimSpace(strings.Join([]string{strings.TrimSpace(claims.GivenName), strings.TrimSpace(claims.FamilyName)}, " "))
		if displayName == "" {
			displayName = strings.TrimSpace(claims.Name)
		}
		return keycloakIdentity{Subject: claims.Subject, Username: claims.PreferredUsername, DisplayName: displayName, Email: claims.Email, FirstName: claims.GivenName, LastName: claims.FamilyName}, nil
	}
	return keycloakIdentity{}, errors.New("signing key not found")
}
