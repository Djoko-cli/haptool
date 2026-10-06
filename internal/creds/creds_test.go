package creds_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Djoko-cli/haptool/internal/creds"
	"github.com/Djoko-cli/haptool/internal/redact"
	"github.com/Djoko-cli/haptool/internal/testkeys"
)

// go2rtcYAML reproduit la structure du fichier réel : sources homekit://
// en éléments de liste entre guillemets, flux _nvr voisins.
func go2rtcYAML(degSource, coursSource string) string {
	return `log:
  level: info
api:
  listen: ":1984"
streams:
  circleview_cam1:
      - "` + degSource + `"
  circleview_cam1_nvr:
      - ffmpeg:circleview_cam1#video=copy
  circleview_cam2:
      - "` + coursSource + `"
  circleview_cam2_nvr:
      - ffmpeg:circleview_cam2#video=copy
rtsp:
  listen: ":8554"
`
}

func writeYAML(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "go2rtc.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// homekitURL assemble une source homekit:// à partir de paramètres
// ordonnés, pour fabriquer des variantes mal formées.
func homekitURL(hostport string, params [][2]string) string {
	var q []string
	for _, kv := range params {
		q = append(q, kv[0]+"="+kv[1])
	}
	return "homekit://" + hostport + "?" + strings.Join(q, "&") + "#bitrate=1920K"
}

func params(p testkeys.Pairing) [][2]string {
	return [][2]string{
		{"client_id", p.ClientID},
		{"client_private", hex.EncodeToString(p.ClientPrivate)},
		{"device_id", p.DeviceID},
		{"device_public", hex.EncodeToString(p.DevicePublic())},
	}
}

func with(ps [][2]string, key, value string) [][2]string {
	out := make([][2]string, 0, len(ps))
	for _, kv := range ps {
		if kv[0] == key {
			kv[1] = value
		}
		out = append(out, kv)
	}
	return out
}

func without(ps [][2]string, key string) [][2]string {
	out := make([][2]string, 0, len(ps))
	for _, kv := range ps {
		if kv[0] != key {
			out = append(out, kv)
		}
	}
	return out
}

// maskedForms liste les formes que le filtre doit masquer une fois les
// clés enregistrées : hex des deux casses, base64, liste décimale (%v),
// client_id.
func maskedForms(p testkeys.Pairing) []string {
	var forms []string
	for _, b := range [][]byte{p.ClientPrivate, p.DevicePublic()} {
		h := hex.EncodeToString(b)
		forms = append(forms, h, strings.ToUpper(h), base64.StdEncoding.EncodeToString(b), fmt.Sprint(b))
	}
	return append(forms, p.ClientID)
}

// leakForms ajoute les octets bruts (%s) : aucun message ne doit les
// contenir, mais seul notre code pourrait les produire, et le filtre ne
// les couvre pas.
func leakForms(p testkeys.Pairing) []string {
	return append(maskedForms(p), string(p.ClientPrivate), string(p.DevicePublic()))
}

// Un fragment de clé recopié (valeur tronquée, erreur de décodage) se
// repère à sa suite hexadécimale ; le device_id, lui, a des « : ».
var hexRun = regexp.MustCompile(`[0-9a-fA-F]{16,}`)

func assertNoSecret(t *testing.T, text string, ps ...testkeys.Pairing) {
	t.Helper()
	for _, p := range ps {
		for _, s := range leakForms(p) {
			if strings.Contains(text, s) {
				t.Fatalf("un secret apparaît dans : %.80q…", text)
			}
		}
	}
	if hexRun.MatchString(text) {
		t.Fatalf("une suite hexadécimale apparaît dans : %.80q…", text)
	}
}

// Défaut attrapé : mauvais flux retenu (par exemple la première source
// homekit:// du fichier), ou clés d'une autre caméra utilisées pour
// signer ou pour vérifier l'identité de la caméra.
func TestLoadReadsRequestedCamera(t *testing.T) {
	deg, cours := testkeys.New(t), testkeys.New(t)
	path := writeYAML(t, go2rtcYAML(deg.URL("192.0.2.46:49154"), cours.URL("192.0.2.45:49152")))

	cam, err := creds.Load(path, "cam2", redact.New())
	if err != nil {
		t.Fatalf("Load : %v", err)
	}

	if cam.Name != "cam2" || cam.Stream != "circleview_cam2" ||
		cam.URLAddr != "192.0.2.45:49152" || cam.DeviceID != cours.DeviceID {
		t.Errorf("caméra lue : %v", cam)
	}
	if cam.ClientID() != cours.ClientID {
		t.Error("ClientID n'est pas celui de cam2")
	}
	data := []byte("M3 de test")
	if !ed25519.Verify(cours.ClientPublic(), data, cam.Sign(data)) {
		t.Error("Sign ne signe pas avec la clé de contrôleur de cam2")
	}
	if !cam.VerifyDevice(data, ed25519.Sign(cours.DevicePrivate, data)) {
		t.Error("VerifyDevice refuse une signature de cam2")
	}
	if cam.VerifyDevice(data, ed25519.Sign(deg.DevicePrivate, data)) {
		t.Error("VerifyDevice accepte la signature d'une autre caméra")
	}
}

// Défaut attrapé : seules les clés de la caméra demandée seraient
// masquées ; celles de l'autre caméra, lues dans le même fichier,
// pourraient sortir en clair.
func TestLoadRegistersSecretsOfAllCameras(t *testing.T) {
	deg, cours := testkeys.New(t), testkeys.New(t)
	path := writeYAML(t, go2rtcYAML(deg.URL("192.0.2.46:49154"), cours.URL("192.0.2.45:49152")))
	red := redact.New()

	if _, err := creds.Load(path, "cam2", red); err != nil {
		t.Fatalf("Load : %v", err)
	}

	for _, p := range []testkeys.Pairing{deg, cours} {
		for _, s := range maskedForms(p) {
			if got := red.String("x " + s); got != "x <REDACTED>" {
				t.Errorf("secret non masqué : %.40q…", got)
			}
		}
	}
}

// Défaut attrapé : une entrée mal formée serait acceptée, ou son message
// d'erreur recopierait une clé (valeur lue, erreur de url.Parse, extrait
// YAML).
func TestLoadRejectsBadInputWithoutLeaking(t *testing.T) {
	p := testkeys.New(t)
	other := testkeys.New(t)
	good := params(p)
	privHex := hex.EncodeToString(p.ClientPrivate)
	// Graine de p, moitié publique d'une autre clé : signature invalide.
	mixed := hex.EncodeToString(append(append([]byte{}, p.ClientPrivate[:32]...), other.ClientPrivate[32:]...))
	ok := other.URL("192.0.2.45:49152")

	tests := []struct {
		name, yaml, wantIn string
	}{
		{"flux absent", strings.Replace(go2rtcYAML(homekitURL("192.0.2.46:49154", good), ok), "circleview_cam1:", "autre:", 1), "circleview_cam1"},
		{"aucune source homekit", go2rtcYAML("ffmpeg:rtsp://exemple", ok), "homekit"},
		{"deux sources homekit", strings.Replace(go2rtcYAML(homekitURL("192.0.2.46:49154", good), ok),
			"  circleview_cam1_nvr:", `      - "`+homekitURL("192.0.2.47:49154", good)+`"`+"\n  circleview_cam1_nvr:", 1), "homekit"},
		{"port invalide", go2rtcYAML(homekitURL("192.0.2.46:49154x", good), ok), "URL"},
		{"client_id absent", go2rtcYAML(homekitURL("192.0.2.46:49154", without(good, "client_id")), ok), "client_id"},
		{"device_id absent", go2rtcYAML(homekitURL("192.0.2.46:49154", without(good, "device_id")), ok), "device_id"},
		{"client_private non hexadécimal", go2rtcYAML(homekitURL("192.0.2.46:49154", with(good, "client_private", "zz"+privHex[2:])), ok), "client_private"},
		{"client_private trop court", go2rtcYAML(homekitURL("192.0.2.46:49154", with(good, "client_private", privHex[:64])), ok), "client_private"},
		{"client_private incohérent", go2rtcYAML(homekitURL("192.0.2.46:49154", with(good, "client_private", mixed)), ok), "client_private"},
		{"device_public absent", go2rtcYAML(homekitURL("192.0.2.46:49154", without(good, "device_public")), ok), "device_public"},
		{"device_public trop court", go2rtcYAML(homekitURL("192.0.2.46:49154", with(good, "device_public", privHex[:32])), ok), "device_public"},
		{"YAML invalide", "streams:\n  circleview_cam1: [\"" + homekitURL("192.0.2.46:49154", good) + "\"\n\tx: :\n", "YAML"},
		{"streams scalaire", "streams: \"" + homekitURL("192.0.2.46:49154", good) + "\"\n", "streams"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cam, err := creds.Load(writeYAML(t, tt.yaml), "cam1", redact.New())
			if err == nil {
				t.Fatalf("Load a accepté une entrée invalide : %v", cam)
			}
			assertNoSecret(t, err.Error(), p, other)
			if !strings.Contains(err.Error(), tt.wantIn) {
				t.Errorf("l'erreur %q ne désigne pas %q", err, tt.wantIn)
			}
		})
	}
}

// Défaut attrapé : fmt afficherait les champs non exportés d'une
// caméra, clés comprises, avec %v, %+v, %#v ou %s.
func TestCameraFormattingHidesKeys(t *testing.T) {
	deg, cours := testkeys.New(t), testkeys.New(t)
	path := writeYAML(t, go2rtcYAML(deg.URL("192.0.2.46:49154"), cours.URL("192.0.2.45:49152")))
	cam, err := creds.Load(path, "cam1", redact.New())
	if err != nil {
		t.Fatalf("Load : %v", err)
	}

	out := fmt.Sprintf("%v|%+v|%#v|%s|%v|%+v|%#v", cam, cam, cam, cam, *cam, *cam, *cam)

	assertNoSecret(t, out, deg)
	if !strings.Contains(out, deg.DeviceID) {
		t.Errorf("le device_id, public, devrait rester visible : %q", out)
	}
}
