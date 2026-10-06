// Package fakeacc simule une caméra HomeKit sur la boucle locale, pour les
// tests : pair-verify réel côté serveur (pkg/hap de go2rtc), puis réponses
// chiffrées tirées d'une base d'accessoires JSON. Réservé aux tests.
package fakeacc

import (
	"bufio"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/AlexxIT/go2rtc/pkg/hap"

	"github.com/Djoko-cli/haptool/internal/testkeys"
)

// Config décrit la fausse caméra.
type Config struct {
	Pairing     testkeys.Pairing   // client autorisé et identité de la caméra
	SignWith    ed25519.PrivateKey // clé qui signe M2 ; nil : celle de Pairing, sinon c'est un imposteur
	Accessories string             // JSON servi par GET /accessories ; vide : Sample
	Silent      bool               // accepte les connexions sans jamais répondre
	Reject      map[uint64]int     // iid → statut HAP renvoyé à une écriture (ex. -70410)
	Ignore      map[uint64]bool    // iid → écriture répondue 204 mais non appliquée
	Lie         map[uint64]int     // iid → statut d'échec renvoyé, mais valeur appliquée
	BrokenMeta  bool               // JSON invalide si meta, perms ou ev est demandé, comme la Circle View 8.2.64
	GarbleWrite bool               // écriture appliquée, mais réponse 207 illisible
}

// Camera est une fausse caméra en service.
type Camera struct {
	Addr string

	cfg   Config
	srv   *hap.Server
	acc   []byte
	chars map[[2]uint64]map[string]json.RawMessage

	mu       sync.Mutex
	requests []string
	writes   []string // corps des PUT /characteristics reçus
	gotM3    bool

	wg sync.WaitGroup // connexions en cours de traitement
}

// Start démarre une fausse caméra, arrêtée à la fin du test.
func Start(t testing.TB, cfg Config) *Camera {
	t.Helper()
	if cfg.Accessories == "" {
		cfg.Accessories = Sample
	}
	signer := cfg.SignWith
	if signer == nil {
		signer = cfg.Pairing.DevicePrivate
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	c := &Camera{
		Addr: ln.Addr().String(),
		cfg:  cfg,
		acc:  []byte(cfg.Accessories),
		srv: &hap.Server{
			DeviceID:      cfg.Pairing.DeviceID,
			DevicePrivate: signer,
			GetClientPublic: func(id string) []byte {
				if id == cfg.Pairing.ClientID {
					return cfg.Pairing.ClientPublic()
				}
				return nil
			},
		},
	}
	if c.chars, err = index(c.acc); err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			c.wg.Add(1)
			go func() {
				defer c.wg.Done()
				c.serve(conn)
			}()
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return c
}

// Requests renvoie les requêtes reçues, « MÉTHODE /chemin?requête », dans
// l'ordre. Le M3 de pair-verify, lu par go2rtc, n'y figure pas.
func (c *Camera) Requests() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.requests...)
}

// Writes renvoie les corps des PUT /characteristics reçus, dans l'ordre.
func (c *Camera) Writes() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.writes...)
}

// Wait attend la fin du traitement des connexions déjà fermées par le
// client : à appeler avant de lire Requests ou ReceivedM3.
func (c *Camera) Wait() { c.wg.Wait() }

// ReceivedM3 dit si un client a envoyé le message M3 de pair-verify, qui
// porte son identifiant et sa signature.
func (c *Camera) ReceivedM3() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gotM3
}

func (c *Camera) record(req *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, req.Method+" "+req.URL.RequestURI())
}

// watch repère les octets lus après la première écriture du serveur. Le
// client n'envoie rien entre M1 et la réception de M2 : tout octet lu
// après l'envoi de M2 appartient à M3.
type watch struct {
	net.Conn
	wrote atomic.Bool
	m3    atomic.Bool
}

func (w *watch) Read(p []byte) (int, error) {
	n, err := w.Conn.Read(p)
	if n > 0 && w.wrote.Load() {
		w.m3.Store(true)
	}
	return n, err
}

func (w *watch) Write(p []byte) (int, error) {
	n, err := w.Conn.Write(p)
	w.wrote.Store(true)
	return n, err
}

func (c *Camera) serve(conn net.Conn) {
	defer conn.Close()
	if c.cfg.Silent {
		_, _ = io.Copy(io.Discard, conn)
		return
	}
	w := &watch{Conn: conn}
	rw := bufio.NewReadWriter(bufio.NewReader(w), bufio.NewWriter(w))
	for {
		req, err := http.ReadRequest(rw.Reader)
		if err != nil {
			return
		}
		c.record(req)
		if req.Method != http.MethodPost || req.URL.Path != hap.PathPairVerify {
			// Ressource protégée demandée sans session chiffrée.
			_, _ = io.Copy(io.Discard, req.Body)
			_, _ = rw.WriteString("HTTP/1.1 470 Connection Authorization Required\r\nContent-Length: 0\r\n\r\n")
			_ = rw.Flush()
			continue
		}
		w.wrote.Store(false)
		w.m3.Store(false)
		_, key, err := c.srv.PairVerify(req, rw)
		if w.m3.Load() {
			c.mu.Lock()
			c.gotM3 = true
			c.mu.Unlock()
		}
		if err != nil {
			return
		}
		sc, err := hap.NewConn(w, rw, key, false)
		if err != nil {
			return
		}
		c.serveSecure(sc)
		return
	}
}

func (c *Camera) serveSecure(sc net.Conn) {
	br := bufio.NewReader(sc)
	for {
		req, err := http.ReadRequest(br)
		if err != nil {
			return
		}
		c.record(req)
		status, body := http.StatusNotFound, []byte(nil)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == hap.PathAccessories:
			status, body = http.StatusOK, c.acc
		case req.Method == http.MethodGet && req.URL.Path == hap.PathCharacteristics:
			status, body = c.characteristics(req)
		case req.Method == http.MethodPut && req.URL.Path == hap.PathCharacteristics:
			status, body = c.write(req)
		}
		head := fmt.Sprintf("HTTP/1.1 %d %s\r\nContent-Type: %s\r\nContent-Length: %d\r\n\r\n",
			status, http.StatusText(status), hap.MimeJSON, len(body))
		if _, err := sc.Write(append([]byte(head), body...)); err != nil {
			return
		}
	}
}

// characteristics répond comme un accessoire HAP : valeur toujours, type,
// droits et métadonnées sur demande ; statut -70409 pour un iid inconnu,
// -70405 pour une caractéristique sans « pr » ; 207 dès qu'un statut
// n'est pas nul.
func (c *Camera) characteristics(req *http.Request) (int, []byte) {
	q := req.URL.Query()
	if c.cfg.BrokenMeta && (q.Has("meta") || q.Has("perms") || q.Has("ev")) {
		return http.StatusOK, []byte(`{"characteristics":[{"aid":1,"iid":2,"perms":["pr"]:1}]}`)
	}
	var out []map[string]json.RawMessage
	multi := false
	for _, id := range strings.Split(q.Get("id"), ",") {
		a, i, _ := strings.Cut(id, ".")
		aid, _ := strconv.ParseUint(a, 10, 64)
		iid, _ := strconv.ParseUint(i, 10, 64)
		o := map[string]json.RawMessage{"aid": raw(aid), "iid": raw(iid)}
		c.mu.Lock()
		ch, ok := c.chars[[2]uint64{aid, iid}]
		if ok {
			ch = copyMap(ch)
		}
		c.mu.Unlock()
		switch {
		case !ok:
			o["status"], multi = raw(-70409), true
		case !hasPerm(ch, "pr"):
			o["status"], multi = raw(-70405), true
		default:
			o["value"] = ch["value"]
			copyIf(o, ch, q.Get("type") == "1", "type")
			copyIf(o, ch, q.Get("perms") == "1", "perms")
			copyIf(o, ch, q.Get("meta") == "1", "format", "unit", "minValue", "maxValue", "minStep",
				"maxLen", "maxDataLen", "valid-values", "valid-values-range")
			if q.Get("ev") == "1" {
				o["ev"] = raw(false)
			}
		}
		out = append(out, o)
	}
	status := http.StatusOK
	if multi {
		status = http.StatusMultiStatus
		for _, o := range out {
			if _, ok := o["status"]; !ok {
				o["status"] = raw(0)
			}
		}
	}
	body, _ := json.Marshal(map[string]any{"characteristics": out})
	return status, body
}

// write applique un PUT /characteristics : 204 si tout est accepté,
// sinon 207 avec un statut par caractéristique (Config.Reject, -70409 pour
// un iid inconnu, -70404 sans « pw »).
func (c *Camera) write(req *http.Request) (int, []byte) {
	raw, _ := io.ReadAll(req.Body)
	var doc struct {
		Characteristics []struct {
			AID   uint64          `json:"aid"`
			IID   uint64          `json:"iid"`
			Value json.RawMessage `json:"value"`
		} `json:"characteristics"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return http.StatusBadRequest, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes = append(c.writes, string(raw))
	var out []map[string]json.RawMessage
	failed := false
	for _, w := range doc.Characteristics {
		st := 0
		ch, ok := c.chars[[2]uint64{w.AID, w.IID}]
		switch {
		case !ok:
			st = -70409
		case !hasPerm(ch, "pw"):
			st = -70404
		case c.cfg.Reject[w.IID] != 0:
			st = c.cfg.Reject[w.IID]
		case c.cfg.Ignore[w.IID]:
		case c.cfg.Lie[w.IID] != 0:
			st = c.cfg.Lie[w.IID]
			ch["value"] = w.Value
		default:
			ch["value"] = w.Value
		}
		failed = failed || st != 0
		out = append(out, map[string]json.RawMessage{"aid": raw2(w.AID), "iid": raw2(w.IID), "status": raw2(st)})
	}
	if c.cfg.GarbleWrite {
		return http.StatusMultiStatus, []byte(`{"characteristics":[`)
	}
	if !failed {
		return http.StatusNoContent, nil
	}
	body, _ := json.Marshal(map[string]any{"characteristics": out})
	return http.StatusMultiStatus, body
}

func raw2(v any) json.RawMessage { return raw(v) }

func index(acc []byte) (map[[2]uint64]map[string]json.RawMessage, error) {
	var doc struct {
		Accessories []struct {
			AID      uint64 `json:"aid"`
			Services []struct {
				Characteristics []map[string]json.RawMessage `json:"characteristics"`
			} `json:"services"`
		} `json:"accessories"`
	}
	if err := json.Unmarshal(acc, &doc); err != nil {
		return nil, err
	}
	chars := map[[2]uint64]map[string]json.RawMessage{}
	for _, a := range doc.Accessories {
		for _, s := range a.Services {
			for _, ch := range s.Characteristics {
				var iid uint64
				if err := json.Unmarshal(ch["iid"], &iid); err != nil {
					return nil, err
				}
				chars[[2]uint64{a.AID, iid}] = ch
			}
		}
	}
	return chars, nil
}

func hasPerm(ch map[string]json.RawMessage, perm string) bool {
	var perms []string
	_ = json.Unmarshal(ch["perms"], &perms)
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

func copyIf(dst, src map[string]json.RawMessage, cond bool, keys ...string) {
	if !cond {
		return
	}
	for _, k := range keys {
		if v, ok := src[k]; ok {
			dst[k] = v
		}
	}
}

func copyMap(m map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func raw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
