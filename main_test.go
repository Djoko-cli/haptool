package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Djoko-cli/haptool/internal/fakeacc"
	"github.com/Djoko-cli/haptool/internal/hapc"
	"github.com/Djoko-cli/haptool/internal/testkeys"
)

// bench : deux fausses caméras, et le YAML de go2rtc qui pointe vers
// elles. Le firmware de cam2 diffère.
type bench struct {
	deg, cours testkeys.Pairing
	fd, fc     *fakeacc.Camera
	config     string
}

func newBench(t *testing.T) bench { return newBenchWith(t, fakeacc.Config{}) }

func newBenchRejecting(t *testing.T, reject map[uint64]int) bench {
	return newBenchWith(t, fakeacc.Config{Reject: reject})
}

// newBenchWith : la fausse cam1 prend les réglages d'écriture de cfg.
func newBenchWith(t *testing.T, cfg fakeacc.Config) bench {
	t.Helper()
	deg, cours := testkeys.New(t), testkeys.New(t)
	cfg.Pairing, cfg.BrokenMeta = deg, true // comme la vraie Circle View
	fd := fakeacc.Start(t, cfg)
	fc := fakeacc.Start(t, fakeacc.Config{Pairing: cours, BrokenMeta: true,
		Accessories: strings.Replace(fakeacc.Sample, `"value":"5.0.0"`, `"value":"5.1.0"`, 1)})
	yaml := fmt.Sprintf("streams:\n  circleview_cam1:\n      - \"%s\"\n  circleview_cam2:\n      - \"%s\"\n",
		deg.URL(fd.Addr), cours.URL(fc.Addr))
	path := filepath.Join(t.TempDir(), "go2rtc.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	return bench{deg: deg, cours: cours, fd: fd, fc: fc, config: path}
}

// exec lance run avec des sorties brutes, sans filtre : ce qui en sort
// prouve que le code n'écrit pas de secret, avant même l'expurgation.
func (b bench) exec(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	e := env{
		stdout: &out,
		stderr: &errb,
		// Ni mDNS ni balayage : l'adresse de l'URL désigne la fausse caméra.
		resolver: &hapc.Resolver{
			Browse:   func(context.Context) (string, error) { return "", nil },
			LookupIP: func(context.Context, string) ([]net.IP, error) { return nil, nil },
		},
		now: func() time.Time { return time.Date(2026, 10, 6, 21, 0, 0, 0, time.UTC) },
	}
	full := append([]string{args[0], "--config", b.config}, args[1:]...)
	code = run(context.Background(), full, e)
	return code, out.String(), errb.String()
}

func secretForms(p testkeys.Pairing) []string {
	forms := []string{p.ClientID, strings.ToUpper(p.ClientID)}
	for _, k := range [][]byte{p.ClientPrivate, p.DevicePublic()} {
		h := hex.EncodeToString(k)
		forms = append(forms, h, strings.ToUpper(h), base64.StdEncoding.EncodeToString(k),
			base64.RawURLEncoding.EncodeToString(k), fmt.Sprint(k))
	}
	return forms
}

func assertNoSecret(t *testing.T, text string, ps ...testkeys.Pairing) {
	t.Helper()
	for _, p := range ps {
		for _, s := range secretForms(p) {
			if strings.Contains(text, s) {
				t.Fatalf("un secret apparaît dans la sortie : %.80q…", text)
			}
		}
	}
}

// Défaut attrapé : un rendu (texte, JSON, Markdown, lecture ciblée)
// écrirait une clé, même sans filtre d'expurgation.
func TestCommandsWriteNoSecret(t *testing.T) {
	b := newBench(t)
	for _, args := range [][]string{
		{"dump", "cam1"},
		{"dump", "--json", "cam1", "cam2"},
		{"dump", "--md", "cam1", "cam2"},
		{"get", "cam2", "1.50,1.35,1.999"},
		{"set", "cam1", "1.68", "1"},
		{"set", "--confirm", "--expect", "21D:0", "cam1", "1.68", "1"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, out, errOut := b.exec(t, args...)
			if code != 0 {
				t.Fatalf("code %d, stderr : %s", code, errOut)
			}
			assertNoSecret(t, out+errOut, b.deg, b.cours)
			if !strings.Contains(out, "Mute") && !strings.Contains(out, "Volume") && !strings.Contains(out, "Indicator") {
				t.Errorf("sortie sans les caractéristiques attendues :\n%s", out)
			}
		})
	}
}

// Défaut attrapé : l'inventaire Markdown des deux caméras omettrait leurs
// différences, ou le masquage de Setup Endpoints.
func TestDumpMarkdownComparesCameras(t *testing.T) {
	b := newBench(t)

	code, out, errOut := b.exec(t, "dump", "--md", "cam1", "cam2")

	if code != 0 {
		t.Fatalf("code %d, stderr : %s", code, errOut)
	}
	for _, want := range []string{
		"## Différences entre cam1 et cam2",
		`| Accessory Information (3E) | Firmware Revision (52) | valeur | "5.0.0" | "5.1.0" |`,
		"&lt;masqué : clés de session possibles, 3 o>",
		"## cam1", "## cam2",
		"firmware `\"5.0.0\"`", "firmware `\"5.1.0\"`",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Markdown sans %q", want)
		}
	}
}

// Défaut attrapé : un statut HAP par caractéristique ne serait pas
// affiché par get.
func TestGetShowsStatuses(t *testing.T) {
	b := newBench(t)

	code, out, errOut := b.exec(t, "get", "cam2", "1.50,1.999")

	if code != 0 {
		t.Fatalf("code %d, stderr : %s", code, errOut)
	}
	if !strings.Contains(out, "statut -70409") {
		t.Errorf("statut -70409 absent :\n%s", out)
	}
	if !strings.Contains(out, "percentage") || !strings.Contains(out, "pr,pw,ev") {
		t.Errorf("métadonnées de /accessories absentes :\n%s", out)
	}
}

// Défaut attrapé : un fichier de go2rtc mal formé ferait sortir une clé
// dans le message d'erreur.
func TestErrorsWriteNoSecret(t *testing.T) {
	p := testkeys.New(t)
	bad := strings.Replace(p.URL("192.0.2.1:49152"), ":49152", ":49152x", 1)
	path := filepath.Join(t.TempDir(), "go2rtc.yaml")
	if err := os.WriteFile(path, []byte("streams:\n  circleview_cam1:\n      - \""+bad+"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	b := bench{config: path}

	code, out, errOut := b.exec(t, "dump", "cam1")

	if code == 0 {
		t.Fatal("dump a réussi sur une URL invalide")
	}
	assertNoSecret(t, out+errOut, p)
}

// Défaut attrapé : un usage incorrect lancerait quand même une connexion,
// ou finirait sans code d'erreur.
func TestUsageErrors(t *testing.T) {
	b := newBench(t)
	for _, args := range [][]string{
		{"inconnu"},
		{"dump"},
		{"dump", "--json", "--md", "cam1"},
		{"dump", "--addr", "127.0.0.1:1", "cam1", "cam2"},
		{"get", "cam1"},
		{"get", "cam1", "1.x"},
		{"dump", "--expect", "21D:0", "cam1"},
		{"get", "--expect", "21D:0", "cam1", "1.50"},
	} {
		if code, _, _ := b.exec(t, args...); code != 2 {
			t.Errorf("%q : code %d, attendu 2", args, code)
		}
	}
}

// Défaut attrapé : main brancherait les sorties sans le filtre
// d'expurgation, qui est la seconde barrière.
func TestNewEnvRedactsOutputs(t *testing.T) {
	p := testkeys.New(t)
	var out, errb bytes.Buffer
	e := newEnv(&out, &errb)
	e.red.AddBytes(p.ClientPrivate)
	secret := hex.EncodeToString(p.ClientPrivate)

	fmt.Fprintln(e.stdout, "sortie "+secret)
	fmt.Fprint(e.stderr, "erreur "+secret)
	e.flush()

	if out.String() != "sortie <REDACTED>\n" || errb.String() != "erreur <REDACTED>" {
		t.Errorf("sorties non expurgées : %q, %q", out.String(), errb.String())
	}
}

// Défaut attrapé (relecture Opus, point 1) : un %XX qui décode en UTF-8
// invalide dans une source homekit:// faisait paniquer le filtre, dont le
// message de panique recopiait tous les secrets, échappés hors de portée
// du filtre. Testé à travers newEnv, comme dans main.
func TestInvalidEscapeInSourceLeaksNothing(t *testing.T) {
	deg, cours := testkeys.New(t), testkeys.New(t)
	badCours := strings.Replace(cours.URL("192.0.2.2:49152"), "device_public=", "device_public=%FF", 1)
	yaml := fmt.Sprintf("streams:\n  circleview_cam1:\n      - \"%s\"\n  circleview_cam2:\n      - \"%s\"\n",
		deg.URL("127.0.0.1:1"), badCours)
	path := filepath.Join(t.TempDir(), "go2rtc.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	e := newEnv(&out, &errb)
	e.resolver = &hapc.Resolver{
		Browse:   func(context.Context) (string, error) { return "", nil },
		LookupIP: func(context.Context, string) ([]net.IP, error) { return nil, nil },
	}

	run(context.Background(), []string{"dump", "--config", path, "cam1"}, e)
	e.flush()

	all := out.String() + errb.String()
	assertNoSecret(t, all, deg, cours)
	for _, p := range []testkeys.Pairing{deg, cours} {
		dec := strings.Trim(fmt.Sprint([]byte(p.ClientPrivate)), "[]")
		if strings.Contains(all, dec[len(dec)/2:]) {
			t.Fatalf("liste décimale partielle de client_private dans : %.120q", all)
		}
	}
}

// Défaut attrapé : run afficherait le contenu d'une panique qui transporte
// une clé (seconde couche du correctif du point 1 de la relecture).
func TestRecoverHidesPanicPayload(t *testing.T) {
	b := newBench(t)
	secret := hex.EncodeToString(b.deg.ClientPrivate)
	var out, errb bytes.Buffer
	e := env{stdout: &out, stderr: &errb, resolver: &hapc.Resolver{
		Browse: func(context.Context) (string, error) { panic("clé " + secret) },
	}}

	code := run(context.Background(), []string{"dump", "--config", b.config, "cam1"}, e)

	if code != 2 || strings.Contains(errb.String(), secret) || !strings.Contains(errb.String(), "détail masqué") {
		t.Errorf("code %d, stderr %q", code, errb.String())
	}
}

// Défaut attrapé : la commande à blanc écrirait quand même, ou n'annoncerait
// pas la valeur actuelle, la nouvelle et le retour arrière.
func TestSetDryRunWritesNothing(t *testing.T) {
	b := newBench(t)

	code, out, errOut := b.exec(t, "set", "cam1", "1.68", "1")
	b.fd.Wait()

	if code != 0 {
		t.Fatalf("code %d, stderr : %s", code, errOut)
	}
	for _, want := range []string{"Camera Operating Mode Indicator (21D)", "Camera Operating Mode (21A)",
		"valeur actuelle : 0", "nouvelle valeur : 1", "pour écrire : haptool set --confirm --expect 21D:0 cam1 1.68 1",
		"retour arrière : haptool set --confirm --expect 21D:1 cam1 1.68 0", "à blanc"} {
		if !strings.Contains(out, want) {
			t.Errorf("sortie sans %q :\n%s", want, out)
		}
	}
	if w := b.fd.Writes(); len(w) != 0 {
		t.Errorf("écriture envoyée à blanc : %q", w)
	}
}

// Défaut attrapé : --confirm n'écrirait pas, écrirait plusieurs fois ou un
// autre corps, ou ne relirait pas la valeur.
func TestSetConfirmWritesOnceAndRereads(t *testing.T) {
	b := newBench(t)

	code, out, errOut := b.exec(t, "set", "--confirm", "--expect", "21D:0", "cam1", "1.68", "1")
	b.fd.Wait()

	if code != 0 {
		t.Fatalf("code %d, stderr : %s", code, errOut)
	}
	if w := b.fd.Writes(); len(w) != 1 || w[0] != `{"characteristics":[{"aid":1,"iid":68,"value":1}]}` {
		t.Errorf("écritures : %q", w)
	}
	if !strings.Contains(out, "HTTP 204") || !strings.Contains(out, "valeur relue : 1") {
		t.Errorf("sortie sans statut ou relecture :\n%s", out)
	}
}

// Défaut attrapé : un refus de la caméra (207, -70410) passerait pour un
// succès, comme avec PutCharacters de go2rtc.
func TestSetReportsRejection(t *testing.T) {
	b := newBenchRejecting(t, map[uint64]int{68: -70410})

	code, out, _ := b.exec(t, "set", "--confirm", "--expect", "21D:0", "cam1", "1.68", "1")

	if code != 1 || !strings.Contains(out, "-70410") || !strings.Contains(out, "valeur relue : 0") {
		t.Errorf("code %d, sortie :\n%s", code, out)
	}
}

// Défaut attrapé : une valeur déjà en place serait réécrite.
func TestSetSameValueWritesNothing(t *testing.T) {
	b := newBench(t)

	code, out, _ := b.exec(t, "set", "--confirm", "--expect", "21D:0", "cam1", "1.68", "0")
	b.fd.Wait()

	if code != 0 || !strings.Contains(out, "déjà") || len(b.fd.Writes()) != 0 {
		t.Errorf("code %d, écritures %q, sortie :\n%s", code, b.fd.Writes(), out)
	}
}

// Défaut attrapé : set écrirait dans une cible que la politique interdit,
// ou avec une valeur invalide ; aucune écriture ne doit partir.
func TestSetRefusesForbiddenTargets(t *testing.T) {
	b := newBench(t)
	for _, args := range [][]string{
		{"1.36", "1"},  // Active du service 110
		{"1.35", "1"},  // 118
		{"1.3", "x"},   // sans pw
		{"1.67", "0"},  // tw
		{"1.81", "1"},  // fabricant
		{"1.68", "2"},  // hors bornes
		{"1.999", "1"}, // iid inconnu
		{"1.68,1.49", "1"},
	} {
		code, out, errOut := b.exec(t, append([]string{"set", "--confirm", "--expect", "21D:0", "cam1"}, args...)...)
		if code != 2 {
			t.Errorf("%v : code %d, sortie %s %s", args, code, out, errOut)
		}
	}
	b.fd.Wait()
	if w := b.fd.Writes(); len(w) != 0 {
		t.Errorf("écritures envoyées : %q", w)
	}
}

// Défaut attrapé : une caméra qui répond 204 sans appliquer la valeur
// passerait pour un succès faute de relecture.
func TestSetDetectsIgnoredWrite(t *testing.T) {
	b := newBenchWith(t, fakeacc.Config{Ignore: map[uint64]bool{68: true}})

	code, out, _ := b.exec(t, "set", "--confirm", "--expect", "21D:0", "cam1", "1.68", "1")

	if code != 1 || !strings.Contains(out, "HTTP 204") || !strings.Contains(out, "valeur relue : 0") {
		t.Errorf("code %d, sortie :\n%s", code, out)
	}
}

// Défaut attrapé : un statut HAP d'échec serait ignoré dès lors que la
// relecture donne la valeur voulue.
func TestSetHonoursFailureStatus(t *testing.T) {
	b := newBenchWith(t, fakeacc.Config{Lie: map[uint64]int{68: -70410}})

	code, out, _ := b.exec(t, "set", "--confirm", "--expect", "21D:0", "cam1", "1.68", "1")

	if code != 1 || !strings.Contains(out, "-70410") {
		t.Errorf("code %d, sortie :\n%s", code, out)
	}
}

// Défaut attrapé (relecture Opus, phase B, point 2) : --confirm écrirait
// sans vérifier que la cible et sa valeur actuelle sont celles annoncées à
// blanc (faute de frappe, base renumérotée).
func TestSetConfirmRequiresMatchingExpect(t *testing.T) {
	b := newBench(t)
	for _, expect := range [][]string{{}, {"--expect", "21B:0"}, {"--expect", "21D:1"}, {"--expect", "21D"}} {
		args := append(append([]string{"set", "--confirm"}, expect...), "cam1", "1.68", "1")
		if code, _, _ := b.exec(t, args...); code != 2 {
			t.Errorf("%v : code %d, attendu 2", expect, code)
		}
	}
	b.fd.Wait()
	if w := b.fd.Writes(); len(w) != 0 {
		t.Errorf("écritures envoyées : %q", w)
	}
}

// Défaut attrapé (relecture Opus, phase B, point 4) : une erreur après
// l'envoi de l'écriture (réponse illisible) laisserait l'état inconnu sans
// relecture ni retour arrière.
func TestSetRereadsAfterUnreadableWriteResponse(t *testing.T) {
	b := newBenchWith(t, fakeacc.Config{GarbleWrite: true})

	code, out, errOut := b.exec(t, "set", "--confirm", "--expect", "21D:0", "cam1", "1.68", "1")

	all := out + errOut
	if code != 1 || !strings.Contains(all, "état inconnu") || !strings.Contains(all, "valeur relue : 1") ||
		!strings.Contains(all, "retour arrière") {
		t.Errorf("code %d, sortie :\n%s", code, all)
	}
}

// Défaut attrapé (relecture finale) : sans --config ni HAPTOOL_CONFIG,
// l'outil lirait un chemin par défaut propre à une installation ; avec
// HAPTOOL_CONFIG, il doit l'utiliser.
func TestConfigComesFromFlagOrEnvironment(t *testing.T) {
	b := newBench(t)
	newEnvFor := func() (env, *bytes.Buffer, *bytes.Buffer) {
		var out, errb bytes.Buffer
		return env{stdout: &out, stderr: &errb, resolver: &hapc.Resolver{
			Browse:   func(context.Context) (string, error) { return "", nil },
			LookupIP: func(context.Context, string) ([]net.IP, error) { return nil, nil },
		}}, &out, &errb
	}

	t.Setenv("HAPTOOL_CONFIG", "")
	e, _, errb := newEnvFor()
	if code := run(context.Background(), []string{"dump", "cam1"}, e); code != 2 || !strings.Contains(errb.String(), "HAPTOOL_CONFIG") {
		t.Errorf("sans configuration : code %d, stderr %q", code, errb.String())
	}

	t.Setenv("HAPTOOL_CONFIG", b.config)
	e, out, errb := newEnvFor()
	if code := run(context.Background(), []string{"dump", "cam1"}, e); code != 0 || !strings.Contains(out.String(), "Mute") {
		t.Errorf("avec HAPTOOL_CONFIG : code %d, stderr %q", code, errb.String())
	}
}

// Défaut attrapé : la caméra renvoie un booléen sous forme de nombre (1)
// alors que l'outil écrit true ou false ; la commande pour écrire et le
// retour arrière doivent employer 0 ou 1, sans quoi --expect ne
// correspondrait plus à la valeur relue.
func TestSetBoolUsesZeroOneOnCommandLine(t *testing.T) {
	b := newBench(t)

	_, out, _ := b.exec(t, "set", "cam1", "1.69", "0")
	for _, want := range []string{
		"pour écrire : haptool set --confirm --expect 11B:1 cam1 1.69 0",
		"retour arrière : haptool set --confirm --expect 11B:0 cam1 1.69 1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("sortie sans %q :\n%s", want, out)
		}
	}

	code, out, errOut := b.exec(t, "set", "--confirm", "--expect", "11B:1", "cam1", "1.69", "0")
	if code != 0 {
		t.Fatalf("écriture : code %d\n%s%s", code, out, errOut)
	}
	code, out, errOut = b.exec(t, "set", "--confirm", "--expect", "11B:0", "cam1", "1.69", "1")
	if code != 0 {
		t.Errorf("retour arrière refusé : code %d\n%s%s", code, out, errOut)
	}
}
