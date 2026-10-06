package hapc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/hap"

	"github.com/Djoko-cli/haptool/internal/creds"
)

// maxBody borne la lecture d'une réponse chiffrée.
const maxBody = 8 << 20

// DefaultTimeout borne chaque échange réseau.
const DefaultTimeout = 5 * time.Second

// Options règle l'ouverture d'une session.
type Options struct {
	Addr     string        // adresse imposée (--addr) ; vide : résolution
	Timeout  time.Duration // délai de chaque échange ; 0 : DefaultTimeout
	Resolver *Resolver     // nil : DefaultResolver()
}

// Session est une connexion de contrôle chiffrée avec une caméra, sans
// abonnement aux événements.
type Session struct {
	Addr   string // adresse jointe
	Source string // origine de l'adresse : imposée, mDNS, URL, balayage

	conn    net.Conn // canal chiffré
	br      *bufio.Reader
	timeout time.Duration
}

// Open joint la caméra et passe pair-verify. Sans adresse imposée, il
// essaie dans l'ordre l'annonce mDNS, l'adresse de l'URL go2rtc, puis les
// serveurs HAP trouvés par balayage de l'hôte de l'URL. Un échec, même de
// pair-verify, fait passer à la source suivante : face à un appareil qui
// n'a pas prouvé être la caméra, seul M1 (clé de session éphémère) part.
func Open(ctx context.Context, cam *creds.Camera, opts Options) (*Session, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if opts.Addr != "" {
		s, err := attempt(ctx, cam, opts.Addr, "imposée", timeout)
		if err != nil {
			return nil, fmt.Errorf("adresse imposée %s : %w", opts.Addr, err)
		}
		return s, nil
	}

	r := opts.Resolver
	if r == nil {
		r = DefaultResolver()
	}
	var failures []string
	try := func(addr, source string) *Session {
		s, err := attempt(ctx, cam, addr, source, timeout)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s %s : %v", source, addr, err))
		}
		return s
	}

	addrs, err := r.mdns(ctx, cam.DeviceID)
	if err != nil {
		failures = append(failures, "mDNS : "+err.Error())
	}
	for _, addr := range addrs {
		if s := try(addr, "mDNS"); s != nil {
			return s, nil
		}
	}
	if s := try(cam.URLAddr, "URL"); s != nil {
		return s, nil
	}
	host, _, err := net.SplitHostPort(cam.URLAddr)
	if err != nil {
		return nil, err
	}
	found := r.scan(ctx, host, cam.URLAddr, timeout)
	if len(found) == 0 {
		failures = append(failures, fmt.Sprintf("balayage de %s : aucun serveur HAP", host))
	}
	for _, addr := range found {
		if s := try(addr, "balayage"); s != nil {
			return s, nil
		}
	}
	return nil, fmt.Errorf("caméra %s injoignable : %s", cam.Name, strings.Join(failures, " ; "))
}

// attempt compose addr puis passe pair-verify ; la connexion est fermée en
// cas d'échec.
func attempt(ctx context.Context, cam *creds.Camera, addr, source string, timeout time.Duration) (*Session, error) {
	conn, err := dial(ctx, addr, timeout)
	if err != nil {
		return nil, err
	}
	secure, err := pairVerify(conn, cam, addr, timeout)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("pair-verify : %w", err)
	}
	return &Session{Addr: addr, Source: source, conn: secure, br: bufio.NewReader(secure), timeout: timeout}, nil
}

func dial(ctx context.Context, addr string, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{Timeout: timeout}
	return d.DialContext(ctx, "tcp", addr)
}

// Close ferme la session.
func (s *Session) Close() error {
	if s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

// Accessories lit la base complète des accessoires (GET /accessories).
func (s *Session) Accessories() ([]Accessory, error) {
	res, body, err := s.get(hap.PathAccessories)
	if err != nil {
		return nil, fmt.Errorf("GET /accessories : %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET /accessories : statut %d %s", res.StatusCode, strconv.QuoteToASCII(statusText(res)))
	}
	var doc struct {
		Accessories []Accessory `json:"accessories"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("GET /accessories : JSON illisible : %w", err)
	}
	return doc.Accessories, nil
}

// Characteristics lit la valeur et le type de caractéristiques. Une réponse
// 207 porte un statut HAP par caractéristique, à lire dans Status. Les
// métadonnées (format, droits, bornes) ne sont pas demandées : la Circle
// View 8.2.64 sert un JSON invalide dès que meta, perms ou ev figure dans
// la requête (vérifié le 2026-10-06) ; elles se lisent dans /accessories.
func (s *Session) Characteristics(ids []ID) ([]Characteristic, error) {
	if len(ids) == 0 {
		return nil, errNoIDs
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	res, body, err := s.get(hap.PathCharacteristics + "?id=" + strings.Join(parts, ",") + "&type=1")
	if err != nil {
		return nil, fmt.Errorf("GET /characteristics : %w", err)
	}
	var doc struct {
		Characteristics []Characteristic `json:"characteristics"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		// Les erreurs de encoding/json citent champs et types, pas les valeurs.
		return nil, fmt.Errorf("GET /characteristics : statut %d, JSON illisible : %v", res.StatusCode, err)
	}
	if len(doc.Characteristics) == 0 {
		return nil, fmt.Errorf("GET /characteristics : statut %d %s, aucune caractéristique", res.StatusCode, strconv.QuoteToASCII(statusText(res)))
	}
	return doc.Characteristics, nil
}

// get envoie une requête GET sur le canal chiffré et renvoie la réponse et
// son corps. Une notification (EVENT/1.0) non sollicitée est ignorée.
func (s *Session) get(path string) (*http.Response, []byte, error) {
	return s.do(http.MethodGet, path, nil)
}

// do envoie une requête sur le canal chiffré, après contrôle par la liste
// blanche, et renvoie la réponse et son corps.
func (s *Session) do(method, path string, body []byte) (*http.Response, []byte, error) {
	if err := allowed(method, path); err != nil {
		return nil, nil, err
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body) // longueur connue : jamais « chunked »
	}
	req, err := http.NewRequest(method, "http://"+s.Addr+path, rd)
	if err != nil {
		return nil, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", hap.MimeJSON)
	}
	if err := s.conn.SetDeadline(time.Now().Add(s.timeout)); err != nil {
		return nil, nil, err
	}
	if err := req.Write(s.conn); err != nil {
		return nil, nil, err
	}
	for {
		res, err := hap.ReadResponse(s.br, req)
		if err != nil {
			return nil, nil, err
		}
		body, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
		_ = res.Body.Close()
		if err != nil {
			return nil, nil, err
		}
		if res.Proto != hap.ProtoEvent {
			return res, body, nil
		}
	}
}

// statusText renvoie le texte qui suit le code de statut, tel que l'a
// envoyé un pair peut-être non authentifié : à citer, jamais à afficher brut.
func statusText(res *http.Response) string {
	_, text, _ := strings.Cut(res.Status, " ")
	return text
}

// WriteResult rend compte d'une écriture : statut HTTP (204 : tout est
// accepté) et, sinon, statut HAP par caractéristique.
type WriteResult struct {
	HTTPStatus int
	Statuses   []Characteristic
}

// Write écrit une valeur déjà contrôlée par le paquet guard, une seule
// caractéristique à la fois, sans abonnement. Le statut est toujours lu :
// 204 si la caméra accepte, sinon un statut HAP par caractéristique.
func (s *Session) Write(id ID, value json.RawMessage) (WriteResult, error) {
	type item struct {
		AID   uint64          `json:"aid"`
		IID   uint64          `json:"iid"`
		Value json.RawMessage `json:"value"`
	}
	body, err := json.Marshal(struct {
		Characteristics []item `json:"characteristics"`
	}{[]item{{id.AID, id.IID, value}}})
	if err != nil {
		return WriteResult{}, err
	}
	res, data, err := s.do(http.MethodPut, hap.PathCharacteristics, body)
	if err != nil {
		return WriteResult{}, fmt.Errorf("écriture : %w", err)
	}
	r := WriteResult{HTTPStatus: res.StatusCode}
	if res.StatusCode == http.StatusNoContent {
		return r, nil
	}
	var doc struct {
		Characteristics []Characteristic `json:"characteristics"`
	}
	if err := json.Unmarshal(data, &doc); err != nil || len(doc.Characteristics) == 0 {
		return r, fmt.Errorf("écriture : statut %d %s, sans statut par caractéristique",
			res.StatusCode, strconv.QuoteToASCII(statusText(res)))
	}
	r.Statuses = doc.Characteristics
	return r, nil
}
