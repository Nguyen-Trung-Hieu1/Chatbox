package config

import "testing"

func TestDefaultModelCatalog(t *testing.T) {
	t.Setenv("AI_FREE_MODELS_JSON", "")
	t.Setenv("AI_PAID_MODELS_JSON", "")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.FreeModels) != 4 || len(c.PaidModels) != 10 {
		t.Fatalf("free=%d paid=%d", len(c.FreeModels), len(c.PaidModels))
	}
	count := 0
	for _, m := range c.PaidModels {
		if m.ID == "ag/gpt-oss-120b-medium" {
			count++
			if m.DisplayName != "gpt-oss-120b" {
				t.Fatalf("label = %q", m.DisplayName)
			}
		}
	}
	if count != 1 {
		t.Fatalf("gpt-oss model count = %d", count)
	}
	for _, id := range []string{"cx/gpt-5.6-terra", "cx/gpt-5.6-sol", "cx/gpt-5.6-luna"} {
		found := false
		for _, m := range c.PaidModels {
			if m.ID == id {
				found = true
			}
		}
		if !found {
			t.Fatalf("paid model missing: %s", id)
		}
		for _, m := range c.FreeModels {
			if m.ID == id {
				t.Fatalf("paid model in free catalog: %s", id)
			}
		}
	}
}
