package redact_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Djoko-cli/haptool/internal/redact"
	"github.com/Djoko-cli/haptool/internal/testkeys"
)

// Défaut attrapé : un filtre appliqué écriture par écriture laisserait
// passer une clé coupée entre deux appels à Write.
func TestWriterMasksSecretSplitAcrossWrites(t *testing.T) {
	p := testkeys.New(t)
	r := redact.New()
	r.AddBytes(p.ClientPrivate)
	var out bytes.Buffer
	w := r.NewWriter(&out)

	line := "clé " + hex.EncodeToString(p.ClientPrivate) + "\n"
	for _, part := range []string{line[:40], line[40:]} {
		n, err := w.Write([]byte(part))
		if err != nil || n != len(part) {
			t.Fatalf("Write = %d, %v ; attendu %d, nil", n, err, len(part))
		}
	}

	if got, want := out.String(), "clé <REDACTED>\n"; got != want {
		t.Errorf("\ngot  %q\nwant %q", got, want)
	}
}

// Défaut attrapé : une dernière ligne sans retour à la ligne serait
// perdue, ou émise sans expurgation, à la fermeture.
func TestWriterFlushEmitsMaskedPartialLine(t *testing.T) {
	p := testkeys.New(t)
	r := redact.New()
	r.AddBytes(p.ClientPrivate)
	var out bytes.Buffer
	w := r.NewWriter(&out)

	if _, err := w.Write([]byte("fin " + hex.EncodeToString(p.ClientPrivate))); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	if got, want := out.String(), "fin <REDACTED>"; got != want {
		t.Errorf("\ngot  %q\nwant %q", got, want)
	}
}

// Défaut attrapé (relecture Opus, point 2) : une chaîne venue du réseau
// (statut HTTP d'un pair non authentifié, unité ou type lus sur la caméra)
// pourrait piloter le terminal ou tromper le lecteur avec des séquences de
// contrôle ou des caractères bidirectionnels. Tabulations et retours à la
// ligne restent intacts.
func TestWriterNeutralizesControlCharacters(t *testing.T) {
	var out bytes.Buffer
	w := redact.New().NewWriter(&out)

	_, _ = w.Write([]byte("a\x1b]0;T\x07b\rc\u202ed\u0085e\tf\n"))

	if got, want := out.String(), "a\\x1b]0;T\\x07b\\x0dc\\u202ed\\u0085e\tf\n"; got != want {
		t.Errorf("\ngot  %q\nwant %q", got, want)
	}
}
