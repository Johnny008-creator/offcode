package skills

import "testing"

func TestCatalogAllLoad(t *testing.T) {
	cat := Catalog()
	if len(cat) < 15 {
		t.Fatalf("expected a real library, got only %d skills", len(cat))
	}
	seen := map[string]bool{}
	for _, e := range cat {
		if seen[e.Name] {
			t.Errorf("duplicate skill name %q", e.Name)
		}
		seen[e.Name] = true

		content, ok := Load(e.Name)
		if !ok {
			t.Errorf("skill %q: Load failed", e.Name)
			continue
		}
		if len(content) < 100 {
			t.Errorf("skill %q: suspiciously short content (%d bytes)", e.Name, len(content))
		}
		if e.Description == "" {
			t.Errorf("skill %q: empty description", e.Name)
		}
	}
}

func TestLoadUnknownSkill(t *testing.T) {
	if _, ok := Load("does-not-exist"); ok {
		t.Error("Load should report ok=false for an unknown skill name")
	}
}
