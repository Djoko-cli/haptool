package hapc_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Djoko-cli/haptool/internal/fakeacc"
	"github.com/Djoko-cli/haptool/internal/hapc"
	"github.com/Djoko-cli/haptool/internal/testkeys"
)

// dnssdOutput imite la sortie de « dns-sd -Z _hap._tcp local. » : en-tête,
// commentaires, doublons, nom d'instance échappé, TXT avec espaces.
func dnssdOutput(deviceID, host string, port int) string {
	return `Browsing for _hap._tcp.local.
DATE: ---Tue 06 Oct 2026---
20:21:48.700  ...STARTING...

; To direct clients to browse a different domain, substitute that domain in place of '@'
lb._dns-sd._udp                                 PTR     @
_hap._tcp                                       PTR     Prise\032salon._hap._tcp
Prise\032salon._hap._tcp                        SRV     0 0 5010 prise-salon.local. ; Replace with unicast FQDN of target host
Prise\032salon._hap._tcp                        TXT     "c#=3" "id=AA:BB:CC:00:00:01" "md=MOD150" "ci=22"
_hap._tcp                                       PTR     Camera\032Test._hap._tcp
Camera\032Test._hap._tcp                        SRV     0 0 ` + strconv.Itoa(port) + ` ` + host + ` ; Replace with unicast FQDN of target host
Camera\032Test._hap._tcp                        TXT     "c#=2" "ff=1" "id=` + deviceID + `" "md=Circle View Camera" "pv=1.1" "sf=0" "ci=17"
Camera\032Test._hap._tcp                        TXT     "c#=2" "ff=1" "id=` + deviceID + `" "md=Circle View Camera" "pv=1.1" "sf=0" "ci=17"
`
}

func portOf(t *testing.T, addr string) int {
	t.Helper()
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := strconv.Atoi(p)
	return n
}

// closedPort renvoie un port de la boucle locale sur lequel rien n'écoute.
func closedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := portOf(t, ln.Addr().String())
	_ = ln.Close()
	return port
}

// resolver : mDNS simulé (sortie dns-sd fournie, hôte résolu en 127.0.0.1)
// et ports de balayage donnés.
func resolver(browse string, scan ...int) *hapc.Resolver {
	return &hapc.Resolver{
		Browse: func(context.Context) (string, error) { return browse, nil },
		LookupIP: func(_ context.Context, host string) ([]net.IP, error) {
			if host == "camera.local." {
				return []net.IP{net.IPv4(127, 0, 0, 1)}, nil
			}
			return nil, fmt.Errorf("hôte %s inconnu", host)
		},
		ScanPorts: scan,
	}
}

func openWith(t *testing.T, p testkeys.Pairing, urlAddr string, r *hapc.Resolver) (*hapc.Session, error) {
	t.Helper()
	s, err := hapc.Open(context.Background(), loadCamera(t, p, urlAddr), hapc.Options{Timeout: 2 * time.Second, Resolver: r})
	if err == nil {
		t.Cleanup(func() { _ = s.Close() })
	}
	return s, err
}

// Défaut attrapé : annonce mDNS ignorée ou mal lue (port, hôte), ou caméra
// reconnue à son nom plutôt qu'à son id, comparé en toute casse.
func TestOpenFindsCameraByMDNS(t *testing.T) {
	p := testkeys.New(t)
	fake := fakeacc.Start(t, fakeacc.Config{Pairing: p})
	urlAddr := "127.0.0.1:" + strconv.Itoa(closedPort(t)) // adresse de l'URL périmée
	r := resolver(dnssdOutput(strings.ToLower(p.DeviceID), "camera.local.", portOf(t, fake.Addr)))

	s, err := openWith(t, p, urlAddr, r)

	if err != nil {
		t.Fatalf("Open : %v", err)
	}
	if s.Addr != fake.Addr || s.Source != "mDNS" {
		t.Errorf("adresse %s (%s), attendu %s (mDNS)", s.Addr, s.Source, fake.Addr)
	}
}

// Défaut attrapé : sans annonce mDNS, l'adresse de l'URL go2rtc ne serait
// pas essayée.
func TestOpenFallsBackToURL(t *testing.T) {
	p := testkeys.New(t)
	fake := fakeacc.Start(t, fakeacc.Config{Pairing: p})
	r := resolver(dnssdOutput("AA:BB:CC:DD:EE:FF", "camera.local.", 1)) // autre caméra

	s, err := openWith(t, p, fake.Addr, r)

	if err != nil {
		t.Fatalf("Open : %v", err)
	}
	if s.Addr != fake.Addr || s.Source != "URL" {
		t.Errorf("adresse %s (%s), attendu %s (URL)", s.Addr, s.Source, fake.Addr)
	}
}

// Défaut attrapé : port changé (coupure de courant) sans balayage, ou
// balayage qui retiendrait un serveur qui n'est pas HAP (pas de 470).
func TestOpenFallsBackToScan(t *testing.T) {
	p := testkeys.New(t)
	fake := fakeacc.Start(t, fakeacc.Config{Pairing: p})
	var webPairVerify atomic.Bool // pair-verify tenté sur un serveur qui n'est pas HAP
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/pair-verify" {
			webPairVerify.Store(true)
		}
	}))
	t.Cleanup(web.Close)
	urlAddr := "127.0.0.1:" + strconv.Itoa(closedPort(t))
	r := resolver("", closedPort(t), portOf(t, web.Listener.Addr().String()), portOf(t, fake.Addr))

	s, err := openWith(t, p, urlAddr, r)

	if err != nil {
		t.Fatalf("Open : %v", err)
	}
	if s.Addr != fake.Addr || s.Source != "balayage" {
		t.Errorf("adresse %s (%s), attendu %s (balayage)", s.Addr, s.Source, fake.Addr)
	}
	if webPairVerify.Load() {
		t.Error("pair-verify tenté sur un serveur web retenu par le balayage")
	}
}

// Défaut attrapé : un imposteur annoncé en mDNS arrêterait la recherche,
// ou recevrait M3, alors que la vraie caméra est à l'adresse de l'URL.
func TestOpenSkipsImpostorAndContinues(t *testing.T) {
	p, other := testkeys.New(t), testkeys.New(t)
	impostor := fakeacc.Start(t, fakeacc.Config{Pairing: p, SignWith: other.DevicePrivate})
	genuine := fakeacc.Start(t, fakeacc.Config{Pairing: p})
	r := resolver(dnssdOutput(p.DeviceID, "camera.local.", portOf(t, impostor.Addr)))

	s, err := openWith(t, p, genuine.Addr, r)
	impostor.Wait()

	if err != nil {
		t.Fatalf("Open : %v", err)
	}
	if s.Addr != genuine.Addr || s.Source != "URL" {
		t.Errorf("adresse %s (%s), attendu %s (URL)", s.Addr, s.Source, genuine.Addr)
	}
	if impostor.ReceivedM3() {
		t.Error("M3 envoyé à l'imposteur")
	}
}

// Défaut attrapé : un échec sans le détail de chaque source essayée.
func TestOpenReportsEverySourceTried(t *testing.T) {
	p := testkeys.New(t)
	urlAddr := "127.0.0.1:" + strconv.Itoa(closedPort(t))

	_, err := openWith(t, p, urlAddr, resolver("", closedPort(t)))

	if err == nil {
		t.Fatal("Open a réussi sans aucune caméra")
	}
	for _, want := range []string{"mDNS", "URL " + urlAddr, "balayage"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("l'erreur ne mentionne pas %q : %v", want, err)
		}
	}
}

// Défaut attrapé (relecture Opus, point 5) : seule la première annonce
// correspondante, dans un ordre aléatoire, était essayée ; un imposteur ou
// un hôte non résolu annoncés avec l'id de la caméra la rendaient
// injoignable quand l'URL est périmée.
func TestOpenTriesEveryMatchingAnnouncement(t *testing.T) {
	p, other := testkeys.New(t), testkeys.New(t)
	impostor := fakeacc.Start(t, fakeacc.Config{Pairing: p, SignWith: other.DevicePrivate})
	genuine := fakeacc.Start(t, fakeacc.Config{Pairing: p})
	line := func(name, host string, port int) string {
		return fmt.Sprintf("%s._hap._tcp  SRV  0 0 %d %s ; x\n%s._hap._tcp  TXT  \"id=%s\" \"ci=17\"\n", name, port, host, name, p.DeviceID)
	}
	browse := line("Fantome", "fantome.local.", 1) + line("Imposteur", "camera.local.", portOf(t, impostor.Addr)) +
		line("Vraie", "camera.local.", portOf(t, genuine.Addr))
	urlAddr := "127.0.0.1:" + strconv.Itoa(closedPort(t))

	for i := range 10 { // l'ordre de parcours varie d'une exécution à l'autre
		s, err := openWith(t, p, urlAddr, resolver(browse))
		if err != nil {
			t.Fatalf("essai %d : %v", i, err)
		}
		if s.Addr != genuine.Addr || s.Source != "mDNS" {
			t.Fatalf("essai %d : %s (%s)", i, s.Addr, s.Source)
		}
	}
	impostor.Wait()
	if impostor.ReceivedM3() {
		t.Error("M3 envoyé à l'imposteur")
	}
}
