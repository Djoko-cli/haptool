package report_test

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/hap/camera"
	"github.com/AlexxIT/go2rtc/pkg/hap/tlv8"

	"github.com/Djoko-cli/haptool/internal/fakeacc"
	"github.com/Djoko-cli/haptool/internal/hapc"
	"github.com/Djoko-cli/haptool/internal/report"
)

func sample(t *testing.T) []hapc.Accessory {
	t.Helper()
	var doc struct {
		Accessories []hapc.Accessory `json:"accessories"`
	}
	if err := json.Unmarshal([]byte(fakeacc.Sample), &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Accessories
}

func intp(v int) *int { return &v }

// Défaut attrapé : la réponse de Setup Endpoints (118), qui porte les clés
// SRTP de la session de go2rtc (camera.SetupEndpointsResponse), serait
// affichée, en base64 ou décodée.
func TestValueMasksSessionKeys(t *testing.T) {
	key := []byte("0123456789abcdef") // clé SRTP factice de 16 octets
	b, err := tlv8.Marshal(camera.SetupEndpointsResponse{
		VideoCrypto: camera.SRTPCryptoSuite{MasterKey: string(key), MasterSalt: "salt-factice-14"},
	})
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(b)
	for _, typ := range []string{"118", "00000118-0000-1000-8000-0026BB765291", "131"} {
		c := hapc.Characteristic{Type: typ, Format: "tlv8", Value: json.RawMessage(`"` + b64 + `"`)}

		got := report.Value(c)

		if strings.Contains(got, b64) || strings.Contains(got, string(key)) || strings.Contains(got, hex.EncodeToString(key)) {
			t.Errorf("type %s : clé visible dans %q", typ, got)
		}
		if !strings.Contains(got, "masqué") {
			t.Errorf("type %s : %q ne signale pas le masquage", typ, got)
		}
	}
}

// Défaut attrapé : valeur TLV8 affichée brute au lieu d'être décodée, ou
// décodée avec la mauvaise structure ; statut d'erreur ignoré au profit
// d'une valeur absente.
func TestValueRendering(t *testing.T) {
	audio, err := tlv8.MarshalBase64(camera.SupportedAudioStreamConfiguration{
		Codecs:              []camera.AudioCodecConfiguration{{CodecType: camera.AudioCodecTypeOpus}},
		ComfortNoiseSupport: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		c    hapc.Characteristic
		want string
	}{
		{"booléen", hapc.Characteristic{Type: "11A", Format: "bool", Value: json.RawMessage(`false`)}, "false"},
		{"chaîne", hapc.Characteristic{Type: "52", Format: "string", Value: json.RawMessage(`"5.0.0"`)}, `"5.0.0"`},
		{"absente", hapc.Characteristic{Type: "14", Format: "bool"}, "—"},
		{"statut", hapc.Characteristic{Type: "14", Status: intp(-70405)}, "statut -70405"},
		{"tlv8 vide", hapc.Characteristic{Type: "117", Format: "tlv8", Value: json.RawMessage(`""`)}, "(vide)"},
		{"tlv8 120", hapc.Characteristic{Type: "120", Format: "tlv8", Value: json.RawMessage(`"AQEA"`)}, "{Status:0}"},
		{"tlv8 115", hapc.Characteristic{Type: "115", Format: "tlv8", Value: json.RawMessage(`"` + audio + `"`)},
			"{Codecs:[{CodecType:3}] ComfortNoiseSupport:1}"},
		{"données", hapc.Characteristic{Type: "8C6E0A3E-1A2B-4C5D-8E9F-0A1B2C3D4E5F", Format: "data", Value: json.RawMessage(`"AAEC"`)},
			"3 o : 000102"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := report.Value(tt.c); !strings.HasPrefix(got, tt.want) {
				t.Errorf("Value = %q, attendu en tête %q", got, tt.want)
			}
		})
	}
}

// Défaut attrapé : une différence de valeur, de droits ou de présence
// entre les deux caméras manquerait au tableau, ou des caractéristiques
// identiques y figureraient.
func TestDiff(t *testing.T) {
	a := report.Camera{Name: "cam1", Accessories: sample(t)}
	b := report.Camera{Name: "cam2", Accessories: sample(t)}
	// b : micro coupé, firmware différent, Volume en lecture seule, voyant absent.
	svc := b.Accessories[0].Services
	svc[0].Characteristics[5].Value = json.RawMessage(`"5.1.0"`) // 52
	svc[3].Characteristics[0].Value = json.RawMessage(`true`)    // 11A
	svc[3].Characteristics[1].Perms = []string{"pr", "ev"}       // 119
	svc[4].Characteristics = svc[4].Characteristics[:2]          // 21D (dernière) retirée

	got := report.Diff(a, b)

	want := []report.Difference{
		{Service: "Accessory Information (3E)", Characteristic: "Firmware Revision (52)", Field: "valeur", A: `"5.0.0"`, B: `"5.1.0"`},
		{Service: "Microphone (112)", Characteristic: "Mute (11A)", Field: "valeur", A: "false", B: "true"},
		{Service: "Microphone (112)", Characteristic: "Volume (119)", Field: "droits", A: "pr,pw,ev", B: "pr,ev"},
		{Service: "Camera Operating Mode (21A)", Characteristic: "Camera Operating Mode Indicator (21D)", Field: "présence", A: "oui", B: "non"},
	}
	if len(got) != len(want) {
		t.Fatalf("%d différences, %d attendues : %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("différence %d :\ngot  %+v\nwant %+v", i, got[i], want[i])
		}
	}
}

// Défaut attrapé : un « | » dans une valeur lue sur la caméra casserait
// le tableau Markdown de l'inventaire.
func TestMarkdownEscapesPipes(t *testing.T) {
	accs := sample(t)
	accs[0].Services[0].Characteristics[3].Value = json.RawMessage(`"Caméra | entrée"`) // Name (23)
	var out bytes.Buffer

	err := report.Markdown(&out, []report.Camera{{Name: "cam1", Accessories: accs}}, time.Date(2026, 10, 6, 21, 0, 0, 0, time.UTC))

	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"Caméra \| entrée"`) {
		t.Errorf("« | » non échappé :\n%s", out.String())
	}
}

// Défaut attrapé (relecture Opus, point 2) : un accent grave ou un « < »
// lu sur la caméra ouvrirait du code ou du HTML dans inventaire.md.
func TestMarkdownEscapesCodeAndHTML(t *testing.T) {
	accs := sample(t)
	accs[0].Services[0].Characteristics[3].Value = json.RawMessage("\"a`b<img>\"")
	var out bytes.Buffer

	if err := report.Markdown(&out, []report.Camera{{Name: "x", Accessories: accs}}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "\"a\\`b&lt;img>\"") {
		t.Errorf("accent grave ou « < » non échappé")
	}
}

// Défaut attrapé (relecture Opus, point 3) : si la caméra ignore type=1,
// get recevrait une valeur sans type ; une valeur Setup Endpoints sortirait
// alors en clair, clé SRTP comprise.
func TestValueMasksUntypedOpaqueValues(t *testing.T) {
	for _, format := range []string{"tlv8", "data", ""} {
		c := hapc.Characteristic{Format: format, Value: json.RawMessage(`"AQIDBAUGBwgJCgsMDQ4PEA=="`)}
		if got := report.Value(c); !strings.Contains(got, "masqué") || strings.Contains(got, "AQIDBAUG") || strings.Contains(got, "010203") {
			t.Errorf("format %q : %q", format, got)
		}
	}
}

// Défaut attrapé (relecture Opus, point 6) : le décodeur TLV8 de go2rtc
// panique sur un flottant tronqué ; tout l'inventaire serait perdu.
func TestValueSurvivesMalformedTLV8(t *testing.T) {
	inner := []byte{4, 1, 0}                             // RTCPInterval (float32) sur 1 octet
	rtp := append([]byte{3, byte(len(inner))}, inner...) // RTPParams
	audio := append([]byte{3, byte(len(rtp))}, rtp...)   // AudioCodec
	c := hapc.Characteristic{Type: "117", Format: "tlv8",
		Value: json.RawMessage(`"` + base64.StdEncoding.EncodeToString(audio) + `"`)}

	got := report.Value(c) // ne doit pas paniquer

	if !strings.HasPrefix(got, "7 o : ") {
		t.Errorf("Value = %q", got)
	}
}

// Défaut attrapé : une caractéristique de fabricant aurait une colonne
// « nom » vide, alors que chaque type doit être nommé ou marqué
// « inconnu ».
func TestUnknownTypesAreMarked(t *testing.T) {
	cam := report.Camera{Name: "x", Accessories: sample(t)}
	var txt, md bytes.Buffer
	if err := report.Text(&txt, cam); err != nil {
		t.Fatal(err)
	}
	if err := report.Markdown(&md, []report.Camera{cam}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	for name, out := range map[string]string{"texte": txt.String(), "markdown": md.String()} {
		if !strings.Contains(out, "inconnu (fabricant)") {
			t.Errorf("%s : caractéristique de fabricant non marquée « inconnu »", name)
		}
	}
}

// Défaut attrapé (relecture finale) : le JSON gardait en clair une valeur
// opaque sans type, que le texte et le Markdown masquent.
func TestJSONMasksUntypedOpaqueValues(t *testing.T) {
	accs := sample(t)
	accs[0].Services[0].Characteristics = append(accs[0].Services[0].Characteristics,
		hapc.Characteristic{IID: 99, Format: "tlv8", Value: json.RawMessage(`"AQIDBAUGBwgJCgsMDQ4PEA=="`)})
	var out bytes.Buffer

	if err := report.JSON(&out, []report.Camera{{Name: "x", Accessories: accs}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "AQIDBAUG") {
		t.Error("valeur opaque sans type en clair dans le JSON")
	}
}
