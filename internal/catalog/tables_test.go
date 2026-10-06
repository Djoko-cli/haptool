package catalog

import (
	"regexp"
	"strings"
	"testing"
)

var (
	// Clé attendue : majuscules, sans zéros de tête, 8 chiffres au plus.
	shortKey = regexp.MustCompile(`^(0|[1-9A-F][0-9A-F]{0,7})$`)

	// Libellé de statut attendu : texte, puis l'identifiant HAP-NodeJS.
	statusLabel = regexp.MustCompile(`^[^()]+ \([A-Z][A-Z0-9_]*\)$`)
)

// Défaut attrapé : une entrée mal normalisée (minuscules, zéros de tête)
// resterait introuvable, puisque Short ne produit jamais cette forme. Toutes
// les entrées sont vérifiées, pas seulement celles que les autres tests
// nomment.
func TestTablesKeysAreShortForms(t *testing.T) {
	tables := map[string]map[string]string{
		"services":         services,
		"caractéristiques": characteristics,
	}
	for name, table := range tables {
		if len(table) == 0 {
			t.Errorf("table %s vide", name)
		}
		for key, value := range table {
			if !shortKey.MatchString(key) {
				t.Errorf("%s : clé %q mal normalisée", name, key)
			}
			if Short(key) != key {
				t.Errorf("%s : Short(%q) = %q, la clé est introuvable", name, key, Short(key))
			}
			if value == "" || value != strings.TrimSpace(value) {
				t.Errorf("%s : nom %q vide ou entouré de blancs (clé %s)", name, value, key)
			}
		}
	}
}

// Défaut attrapé : une entrée atteignable seulement sous une forme. Chaque
// entrée doit se retrouver par son UUID complet, en majuscules comme en
// minuscules.
func TestTablesLookupByFullUUID(t *testing.T) {
	for key, want := range services {
		full := strings.Repeat("0", 8-len(key)) + key + appleSuffix
		for _, uuid := range []string{full, strings.ToLower(full)} {
			if got := Service(uuid); got != want {
				t.Errorf("Service(%q) = %q, want %q", uuid, got, want)
			}
		}
	}
	for key, want := range characteristics {
		full := strings.Repeat("0", 8-len(key)) + key + appleSuffix
		for _, uuid := range []string{full, strings.ToLower(full)} {
			if got := Characteristic(uuid); got != want {
				t.Errorf("Characteristic(%q) = %q, want %q", uuid, got, want)
			}
		}
	}
}

// Défaut attrapé : un libellé sans identifiant HAP-NodeJS, ou un code positif
// (HAP ne connaît que 0 et des codes négatifs).
func TestStatusesAreLabelledWithIdentifier(t *testing.T) {
	if len(statuses) == 0 {
		t.Error("table des statuts vide")
	}
	for code, label := range statuses {
		if code > 0 {
			t.Errorf("code %d positif", code)
		}
		if !statusLabel.MatchString(label) {
			t.Errorf("code %d : libellé %q sans la forme « texte (IDENTIFIANT) »", code, label)
		}
	}
}
