package persistence

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestKeycloakSessionDocument(t *testing.T) {
	input := keycloakSessionDoc{TokenHash: sessionHash("browser-cookie"), Tokens: KeycloakSession{AccessToken: "access", RefreshToken: "refresh", IDToken: "id", AccessExpiry: time.Now()}, ExpiresAt: time.Now().Add(time.Hour)}
	encoded, err := bson.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var restored keycloakSessionDoc
	if err := bson.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Tokens.RefreshToken != "refresh" || restored.TokenHash == "browser-cookie" {
		t.Fatalf("incorrect session document: %+v", restored)
	}
}
