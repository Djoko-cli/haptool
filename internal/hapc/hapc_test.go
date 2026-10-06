package hapc_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Djoko-cli/haptool/internal/creds"
	"github.com/Djoko-cli/haptool/internal/fakeacc"
	"github.com/Djoko-cli/haptool/internal/hapc"
	"github.com/Djoko-cli/haptool/internal/redact"
	"github.com/Djoko-cli/haptool/internal/testkeys"
)

// loadCamera passe par le vrai chemin de lecture des clés : YAML
// temporaire, puis creds.Load.
func loadCamera(t *testing.T, p testkeys.Pairing, urlAddr string) *creds.Camera {
	t.Helper()
	path := filepath.Join(t.TempDir(), "go2rtc.yaml")
	yaml := "streams:\n  circleview_test:\n      - \"" + p.URL(urlAddr) + "\"\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cam, err := creds.Load(path, "test", redact.New())
	if err != nil {
		t.Fatal(err)
	}
	return cam
}

func open(t *testing.T, cam *creds.Camera, addr string) *hapc.Session {
	t.Helper()
	s, err := hapc.Open(context.Background(), cam, hapc.Options{Addr: addr, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("Open : %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// openAndClose tente une session et la referme aussitôt si elle s'ouvre :
// un échec attendu qui n'a pas lieu ne doit pas bloquer fakeacc.Wait.
func openAndClose(cam *creds.Camera, addr string) error {
	s, err := hapc.Open(context.Background(), cam, hapc.Options{Addr: addr, Timeout: 2 * time.Second})
	if err == nil {
		_ = s.Close()
	}
	return err
}

func findChar(t *testing.T, accs []hapc.Accessory, iid uint64) hapc.Characteristic {
	t.Helper()
	for _, a := range accs {
		for _, s := range a.Services {
			for _, c := range s.Characteristics {
				if c.IID == iid {
					return c
				}
			}
		}
	}
	t.Fatalf("caractéristique %d absente", iid)
	return hapc.Characteristic{}
}

func findService(t *testing.T, accs []hapc.Accessory, iid uint64) hapc.Service {
	t.Helper()
	for _, a := range accs {
		for _, s := range a.Services {
			if s.IID == iid {
				return s
			}
		}
	}
	t.Fatalf("service %d absent", iid)
	return hapc.Service{}
}

// Défaut attrapé : pair-verify incompatible avec un accessoire HAP réel
// (ici le serveur de go2rtc), ou métadonnées perdues au décodage, comme
// avec la structure Character de go2rtc.
func TestOpenReadsFullAccessoryDatabase(t *testing.T) {
	p := testkeys.New(t)
	fake := fakeacc.Start(t, fakeacc.Config{Pairing: p})
	s := open(t, loadCamera(t, p, "192.0.2.1:49152"), fake.Addr)

	accs, err := s.Accessories()
	if err != nil {
		t.Fatalf("Accessories : %v", err)
	}

	if s.Addr != fake.Addr || s.Source != "imposée" {
		t.Errorf("adresse %s (%s), attendu %s (imposée)", s.Addr, s.Source, fake.Addr)
	}
	if len(accs) != 1 || accs[0].AID != 1 || len(accs[0].Services) != 6 {
		t.Fatalf("base lue : %d accessoires", len(accs))
	}
	vol := findChar(t, accs, 50)
	if vol.Type != "119" || vol.Format != "uint8" || vol.Unit != "percentage" ||
		string(vol.Value) != "100" || string(vol.MinValue) != "0" || string(vol.MaxValue) != "100" ||
		string(vol.MinStep) != "1" || !slices.Equal(vol.Perms, []string{"pr", "pw", "ev"}) {
		t.Errorf("volume décodé : %+v", vol)
	}
	if active := findChar(t, accs, 36); string(active.ValidValues) != "[0,1]" {
		t.Errorf("valid-values de B0 : %q", active.ValidValues)
	}
	if !findService(t, accs, 32).Primary || !findService(t, accs, 80).Hidden ||
		!slices.Equal(findService(t, accs, 64).Linked, []uint64{32}) {
		t.Error("primary, hidden ou linked perdu")
	}
	if extra := findChar(t, accs, 81).Extra; string(extra["x-vendor"]) != `"z"` {
		t.Errorf("champ hors spécification perdu : %v", extra)
	}
}

// Défaut attrapé : le client enverrait M3 (son identifiant et sa
// signature) à un appareil qui n'a pas prouvé être la caméra, par exemple
// celui qui occupe aujourd'hui une adresse périmée.
func TestOpenRefusesImpostorBeforeSendingIdentity(t *testing.T) {
	p, other := testkeys.New(t), testkeys.New(t)
	cam := loadCamera(t, p, "192.0.2.1:49152")

	// Témoin : face à la vraie caméra, M3 part bien.
	genuine := fakeacc.Start(t, fakeacc.Config{Pairing: p})
	s := open(t, cam, genuine.Addr)
	_ = s.Close()
	genuine.Wait()
	if !genuine.ReceivedM3() {
		t.Fatal("témoin : M3 non détecté face à la vraie caméra")
	}

	impostor := fakeacc.Start(t, fakeacc.Config{Pairing: p, SignWith: other.DevicePrivate})
	err := openAndClose(cam, impostor.Addr)
	impostor.Wait()

	if err == nil {
		t.Fatal("Open a accepté un imposteur")
	}
	if impostor.ReceivedM3() {
		t.Error("M3 envoyé à un imposteur")
	}
}

// Défaut attrapé : un appareil qui se présente sous un autre device_id
// serait accepté ; M3 ne doit pas partir.
func TestOpenRefusesUnexpectedDeviceID(t *testing.T) {
	p := testkeys.New(t)
	cam := loadCamera(t, p, "192.0.2.1:49152")
	renamed := p
	renamed.DeviceID = "AA:BB:CC:DD:EE:FF"
	fake := fakeacc.Start(t, fakeacc.Config{Pairing: renamed})

	err := openAndClose(cam, fake.Addr)
	fake.Wait()

	if err == nil {
		t.Fatal("Open a accepté un device_id inattendu")
	}
	if fake.ReceivedM3() {
		t.Error("M3 envoyé à un appareil au device_id inattendu")
	}
}

// Défaut attrapé : un statut HAP par caractéristique (207 Multi-Status)
// serait ignoré, comme dans PutCharacters de go2rtc ; ou la requête
// demanderait meta, perms ou ev, que la Circle View 8.2.64 sert en JSON
// invalide (vérifié sur la caméra le 2026-10-06).
func TestCharacteristicsReportsValuesAndStatuses(t *testing.T) {
	p := testkeys.New(t)
	fake := fakeacc.Start(t, fakeacc.Config{Pairing: p, BrokenMeta: true})
	s := open(t, loadCamera(t, p, "192.0.2.1:49152"), fake.Addr)

	chars, err := s.Characteristics([]hapc.ID{{AID: 1, IID: 50}, {AID: 1, IID: 999}, {AID: 1, IID: 2}})
	if err != nil {
		t.Fatalf("Characteristics : %v", err)
	}

	if len(chars) != 3 {
		t.Fatalf("%d réponses, 3 attendues", len(chars))
	}
	vol := chars[0]
	if vol.IID != 50 || string(vol.Value) != "100" || vol.Type != "119" || vol.Status == nil || *vol.Status != 0 {
		t.Errorf("volume : %+v", vol)
	}
	if c := chars[1]; c.IID != 999 || c.Status == nil || *c.Status != -70409 {
		t.Errorf("iid inconnu : %+v", c)
	}
	if c := chars[2]; c.IID != 2 || c.Status == nil || *c.Status != -70405 {
		t.Errorf("Identify, en écriture seule : %+v", c)
	}
}

// Défaut attrapé : la session toucherait une ressource hors périmètre
// (/pairings, /pair-setup, /resource, ou une écriture).
func TestSessionSendsOnlyAllowedRequests(t *testing.T) {
	p := testkeys.New(t)
	fake := fakeacc.Start(t, fakeacc.Config{Pairing: p})
	s := open(t, loadCamera(t, p, "192.0.2.1:49152"), fake.Addr)
	if _, err := s.Accessories(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Characteristics([]hapc.ID{{AID: 1, IID: 50}}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	fake.Wait()

	var got []string
	for _, r := range fake.Requests() {
		path, _, _ := strings.Cut(r, "?")
		got = append(got, path)
	}
	want := []string{"POST /pair-verify", "GET /accessories", "GET /characteristics"}
	if !slices.Equal(got, want) {
		t.Errorf("requêtes : %q\nattendu  : %q", got, want)
	}
}

// Défaut attrapé : sans délai pendant pair-verify (le défaut de go2rtc),
// une caméra qui accepte la connexion sans répondre bloquerait l'outil.
func TestOpenTimesOutOnSilentCamera(t *testing.T) {
	p := testkeys.New(t)
	fake := fakeacc.Start(t, fakeacc.Config{Pairing: p, Silent: true})
	cam := loadCamera(t, p, "192.0.2.1:49152")

	done := make(chan error, 1)
	go func() {
		_, err := hapc.Open(context.Background(), cam, hapc.Options{Addr: fake.Addr, Timeout: 300 * time.Millisecond})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Open a réussi face à une caméra muette")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Open bloqué face à une caméra muette")
	}
}

// Défaut attrapé : un identifiant mal formé partirait tel quel dans la
// requête, ou un aid ou iid nul serait accepté.
func TestParseIDs(t *testing.T) {
	got, err := hapc.ParseIDs("1.10,1.11")
	if err != nil || !slices.Equal(got, []hapc.ID{{AID: 1, IID: 10}, {AID: 1, IID: 11}}) {
		t.Errorf("ParseIDs(1.10,1.11) = %v, %v", got, err)
	}
	for _, bad := range []string{"", "1", "1.", ".2", "a.b", "1.0", "0.1", "1.2,", "1.2;3.4", "1.-2"} {
		if ids, err := hapc.ParseIDs(bad); err == nil {
			t.Errorf("ParseIDs(%q) accepté : %v", bad, ids)
		}
	}
}

// Défaut attrapé : le statut HAP d'une écriture serait ignoré, comme dans
// PutCharacters de go2rtc (client.go:341), ou le corps envoyé ne serait
// pas exactement l'écriture demandée.
func TestWriteReportsStatus(t *testing.T) {
	p := testkeys.New(t)
	fake := fakeacc.Start(t, fakeacc.Config{Pairing: p, Reject: map[uint64]int{49: -70410}})
	s := open(t, loadCamera(t, p, "192.0.2.1:49152"), fake.Addr)

	ok, err := s.Write(hapc.ID{AID: 1, IID: 68}, json.RawMessage("1"))
	if err != nil || ok.HTTPStatus != 204 || len(ok.Statuses) != 0 {
		t.Errorf("écriture acceptée : %+v, %v", ok, err)
	}
	ko, err := s.Write(hapc.ID{AID: 1, IID: 49}, json.RawMessage("true"))
	if err != nil || ko.HTTPStatus != 207 || len(ko.Statuses) != 1 || ko.Statuses[0].Status == nil || *ko.Statuses[0].Status != -70410 {
		t.Errorf("écriture refusée : %+v, %v", ko, err)
	}
	_ = s.Close()
	fake.Wait()

	want := []string{`{"characteristics":[{"aid":1,"iid":68,"value":1}]}`, `{"characteristics":[{"aid":1,"iid":49,"value":true}]}`}
	if !slices.Equal(fake.Writes(), want) {
		t.Errorf("corps envoyés : %q", fake.Writes())
	}
}
