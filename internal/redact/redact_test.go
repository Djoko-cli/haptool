package redact_test

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/Djoko-cli/haptool/internal/redact"
	"github.com/Djoko-cli/haptool/internal/testkeys"
)

// Défaut attrapé : la requête d'une URL homekit://, qui porte les clés,
// sortirait en clair, même sans aucun secret enregistré.
func TestStringMasksHomekitURLQuery(t *testing.T) {
	p := testkeys.New(t)
	url := p.URL("192.0.2.45:49152")
	r := redact.New()

	tests := []struct{ name, in, want string }{
		{"texte libre", "source : " + url + " fin", "source : homekit://192.0.2.45:49152?<REDACTED> fin"},
		{"entre guillemets", `- "` + url + `"`, `- "homekit://192.0.2.45:49152?<REDACTED>"`},
		{"majuscules", "HOMEKIT://" + url[len("homekit://"):], "HOMEKIT://192.0.2.45:49152?<REDACTED>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.String(tt.in); got != tt.want {
				t.Errorf("\ngot  %q\nwant %q", got, tt.want)
			}
		})
	}
}

// Défaut attrapé : une erreur de url.Parse recopie l'URL entière, clés
// comprises (net/url, Error.Error) ; le motif d'URL doit la couvrir.
func TestStringMasksURLParseError(t *testing.T) {
	p := testkeys.New(t)
	bad := strings.Replace(p.URL("192.0.2.45:49152"), ":49152", ":49152x", 1)
	_, err := url.Parse(bad)
	if err == nil {
		t.Fatal("url.Parse aurait dû échouer sur un port invalide")
	}

	got := redact.New().String(err.Error())

	want := `parse "homekit://192.0.2.45:49152x?<REDACTED>": invalid port ":49152x" after host`
	if got != want {
		t.Errorf("\ngot  %q\nwant %q", got, want)
	}
}

// Défaut attrapé : un paramètre de clé affiché hors d'une URL homekit://
// sortirait en clair. device_id n'est pas secret et reste visible.
func TestStringMasksKeyParams(t *testing.T) {
	p := testkeys.New(t)
	in := fmt.Sprintf("device_id=%s&device_public=%x&client_id=%s&client_private=%x",
		p.DeviceID, []byte(p.DevicePublic()), p.ClientID, []byte(p.ClientPrivate))

	got := redact.New().String(in)

	want := "device_id=" + p.DeviceID + "&device_public=<REDACTED>&client_id=<REDACTED>&client_private=<REDACTED>"
	if got != want {
		t.Errorf("\ngot  %q\nwant %q", got, want)
	}
}

// Défaut attrapé : une clé enregistrée sortirait en clair sous une autre
// casse ou un autre encodage, ou une forme courte masquerait seulement
// le début d'une forme longue (base64 avec bourrage).
func TestStringMasksRegisteredSecrets(t *testing.T) {
	p := testkeys.New(t)
	priv := []byte(p.ClientPrivate)
	pub := []byte(p.DevicePublic())
	r := redact.New()
	r.AddBytes(priv)
	r.AddBytes(pub)
	r.Add(p.ClientID)

	tests := []struct{ name, in, want string }{
		{"privée, hex", "k " + hex.EncodeToString(priv) + ".", "k <REDACTED>."},
		{"privée, hex majuscule", "k " + strings.ToUpper(hex.EncodeToString(priv)), "k <REDACTED>"},
		{"privée, base64", "k " + base64.StdEncoding.EncodeToString(priv), "k <REDACTED>"},
		{"privée, base64 sans bourrage", "k " + base64.RawStdEncoding.EncodeToString(priv), "k <REDACTED>"},
		{"privée, base64 URL", "k " + base64.URLEncoding.EncodeToString(priv), "k <REDACTED>"},
		{"privée, liste décimale", fmt.Sprintf("k %v", priv), "k <REDACTED>"},
		{"publique, hex", "pub " + hex.EncodeToString(pub), "pub <REDACTED>"},
		{"client_id", "id " + p.ClientID + " ok", "id <REDACTED> ok"},
		{"client_id majuscule", strings.ToUpper(p.ClientID), "<REDACTED>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.String(tt.in); got != tt.want {
				t.Errorf("\ngot  %q\nwant %q", got, tt.want)
			}
		})
	}
}

// Défaut attrapé : un « secret » trop court masquerait n'importe quel
// texte (ici, chaque « 1 »).
func TestAddIgnoresTooShortSecrets(t *testing.T) {
	r := redact.New()
	r.Add("1")

	if got, want := r.String("version 1.0"), "version 1.0"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Défaut attrapé (relecture Opus, point 1) : une chaîne en UTF-8 invalide
// faisait paniquer Add, dont le message recopiait les secrets ; elle doit
// rester masquée sous sa forme hexadécimale.
func TestAddAcceptsInvalidUTF8(t *testing.T) {
	p := testkeys.New(t)
	r := redact.New()
	r.AddBytes(p.ClientPrivate)
	bad := "\xff" + string(p.DevicePublic())

	r.Add(bad) // ne doit pas paniquer

	h := hex.EncodeToString([]byte(bad))
	if got := r.String("x " + h); got != "x <REDACTED>" {
		t.Errorf("forme hexadécimale non masquée : %.40q", got)
	}
	if got := r.String("y " + hex.EncodeToString(p.ClientPrivate)); got != "y <REDACTED>" {
		t.Errorf("secret antérieur perdu : %.40q", got)
	}
}
