package guard_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Djoko-cli/haptool/internal/guard"
	"github.com/Djoko-cli/haptool/internal/hapc"
)

func svc(t string) hapc.Service { return hapc.Service{Type: t} }

func ch(typ, format string, perms []string, extra ...string) hapc.Characteristic {
	c := hapc.Characteristic{Type: typ, Format: format, Perms: perms}
	for i := 0; i+1 < len(extra); i += 2 {
		v := json.RawMessage(extra[i+1])
		switch extra[i] {
		case "min":
			c.MinValue = v
		case "max":
			c.MaxValue = v
		case "step":
			c.MinStep = v
		case "valid":
			c.ValidValues = v
		case "maxLen":
			c.MaxLen = v
		}
	}
	return c
}

var rw = []string{"pr", "pw", "ev"}

// Défaut attrapé : une écriture partirait vers le service de streaming de
// go2rtc (110, 117, 118), une valeur opaque (TLV8, données), une
// caractéristique sans « pw », en écriture temporisée (PUT /prepare, hors
// périmètre), à autorisation additionnelle, ou de fabricant (non
// identifiable, peut-être une réinitialisation).
func TestRefusedTargets(t *testing.T) {
	tests := []struct {
		name string
		s    hapc.Service
		c    hapc.Characteristic
	}{
		{"service 110", svc("110"), ch("B0", "uint8", rw, "min", "0", "max", "1")},
		{"117", svc("21A"), ch("117", "uint8", rw)},
		{"118 en UUID complet", svc("21A"), ch("00000118-0000-1000-8000-0026BB765291", "uint8", rw)},
		{"131", svc("129"), ch("131", "uint8", rw)},
		{"tlv8", svc("21A"), ch("21D", "tlv8", rw)},
		{"data", svc("21A"), ch("21D", "data", rw)},
		{"sans pw", svc("3E"), ch("23", "string", []string{"pr"})},
		{"tw", svc("21A"), ch("11B", "bool", []string{"pr", "pw", "ev", "tw"})},
		{"aa", svc("21A"), ch("11B", "bool", []string{"pr", "pw", "aa"})},
		{"fabricant", svc("68163DDE-599E-C8BC-7B45-77DA153E1AE7"), ch("27BE48D7-AB48-1684-1540-1F1E3C9BA0FA", "uint8", []string{"pr", "pw"})},
		{"service de fabricant", svc("68163DDE-599E-C8BC-7B45-77DA153E1AE7"), ch("21D", "uint8", rw)},
		{"format inconnu", svc("21A"), ch("21D", "", rw)},
		{"21B HomeKit Camera Active", svc("21A"), ch("21B", "uint8", rw, "min", "0", "max", "1")},
		{"11A Mute", svc("112"), ch("11A", "bool", rw)},
		{"pw en majuscules mais TW", svc("21A"), ch("21D", "uint8", []string{"pr", "pw", "TW"})},
		{"11B sans pw", svc("21A"), ch("11B", "bool", []string{"pr", "ev"})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if v, err := guard.Value(tt.s, tt.c, "1"); err == nil {
				t.Errorf("écriture acceptée : %s", v)
			}
		})
	}
}

// Défaut attrapé : une valeur hors du type, des bornes, du pas, des
// valeurs admises ou de la longueur maximale partirait vers la caméra, ou
// une valeur valide serait mal encodée.
func TestValueParsing(t *testing.T) {
	ind := ch("21D", "uint8", rw, "min", "0", "max", "1", "step", "1")
	vol := ch("119", "uint8", rw, "min", "0", "max", "100", "step", "5")
	act := ch("B0", "uint8", rw, "valid", "[0,1]")
	tests := []struct {
		name  string
		c     hapc.Characteristic
		input string
		want  string // "" : refus attendu
	}{
		{"uint8 1", ind, "1", "1"},
		{"uint8 0", ind, "0", "0"},
		{"au-dessus du max", ind, "2", ""},
		{"négatif", ind, "-1", ""},
		{"pas un nombre", ind, "un", ""},
		{"décimal pour un entier", ind, "1.0", ""},
		{"hors du pas", vol, "52", ""},
		{"dans le pas", vol, "55", "55"},
		{"hors de uint8", ch("119", "uint8", rw), "256", ""},
		{"valid-values", act, "1", "1"},
		{"hors valid-values", act, "3", ""},
		{"bool vrai", ch("11A", "bool", rw), "1", "true"},
		{"bool faux", ch("11A", "bool", rw), "false", "false"},
		{"bool invalide", ch("11A", "bool", rw), "2", ""},
		{"float", ch("6B", "float", rw, "min", "0", "max", "10"), "2.5", "2.5"},
		{"float hors bornes", ch("6B", "float", rw, "min", "0", "max", "10"), "11", ""},
		{"chaîne", ch("23", "string", rw), "Entrée", `"Entrée"`},
		{"chaîne trop longue", ch("23", "string", rw, "maxLen", "3"), "abcd", ""},
		{"chaîne de 64 par défaut", ch("23", "string", rw), strings.Repeat("a", 65), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := guard.Encode(tt.c, tt.input)
			switch {
			case tt.want == "" && err == nil:
				t.Errorf("valeur %q acceptée : %s", tt.input, got)
			case tt.want != "" && err != nil:
				t.Errorf("valeur %q refusée : %v", tt.input, err)
			case tt.want != "" && string(got) != tt.want:
				t.Errorf("valeur %q encodée %s, attendu %s", tt.input, got, tt.want)
			}
		})
	}
}

// Défaut attrapé (relecture Opus, phase B, point 1) : la politique était
// une liste noire ; seule la caractéristique du voyant doit être
// inscriptible.
func TestOnlyStatusLightIsWritable(t *testing.T) {
	got, err := guard.Value(svc("21A"), ch("0000021d-0000-1000-8000-0026bb765291", "uint8", rw, "min", "0", "max", "1"), "1")
	if err != nil || string(got) != "1" {
		t.Errorf("21D refusé : %s, %v", got, err)
	}
}

// Défaut attrapé : Night Vision (11B), ajouté à la liste blanche à la
// demande du propriétaire, serait refusé dans son service (21A), ou
// accepté hors de lui ou avec une écriture temporisée.
func TestNightVisionWritableOnlyInOperatingMode(t *testing.T) {
	nv := ch("11B", "bool", rw)
	if got, err := guard.Value(svc("21A"), nv, "0"); err != nil || string(got) != "false" {
		t.Errorf("11B dans 21A refusé : %s, %v", got, err)
	}
	for name, tt := range map[string]struct {
		s hapc.Service
		c hapc.Characteristic
	}{
		"autre service": {svc("110"), nv},
		"service 112":   {svc("112"), nv},
		"tw":            {svc("21A"), ch("11B", "bool", []string{"pr", "pw", "ev", "tw"})},
	} {
		if v, err := guard.Value(tt.s, tt.c, "0"); err == nil {
			t.Errorf("%s : écriture acceptée : %s", name, v)
		}
	}
}
