// Package creds lit, dans le fichier de go2rtc, le matériel d'appairage
// HomeKit d'une caméra.
//
// Les clés de toutes les sources homekit:// du fichier sont enregistrées
// dans le filtre d'expurgation avant tout autre traitement. Elles restent
// ensuite dans des champs non exportés, et aucun message d'erreur ne
// recopie de contenu du fichier.
package creds

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Djoko-cli/haptool/internal/redact"
)

// StreamPrefix précède le nom de la caméra dans le nom de son flux go2rtc.
const StreamPrefix = "circleview_"

// Camera porte le matériel d'appairage d'une caméra. Son affichage, quel
// que soit le verbe de fmt, se limite à la partie publique.
type Camera struct {
	Name     string // cam1
	Stream   string // circleview_cam1
	URLAddr  string // hôte:port de l'URL go2rtc, parfois périmé
	DeviceID string // diffusé en mDNS, non secret

	clientID      string
	clientPrivate []byte // 64 octets : graine puis clé publique
	devicePublic  []byte // 32 octets
}

// Load lit path en lecture seule et renvoie la caméra name, soit le flux
// circleview_<name>. Les secrets de toutes les caméras du fichier sont
// d'abord enregistrés dans red.
func Load(path, name string, red *redact.Redactor) (*Camera, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err // chemin et cause, sans contenu
	}
	var root any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, yamlError(err)
	}
	doc, _ := root.(map[string]any)
	streams, ok := doc["streams"].(map[string]any)
	if !ok {
		return nil, errors.New("go2rtc.yaml : section streams absente ou mal formée")
	}

	sources := map[string][]string{} // flux → sources homekit://
	for stream, v := range streams {
		for _, src := range sourcesOf(v) {
			if isHomekit(src) {
				registerSecrets(red, src)
				sources[stream] = append(sources[stream], src)
			}
		}
	}

	stream := StreamPrefix + name
	if _, ok := streams[stream]; !ok {
		return nil, fmt.Errorf("flux %s absent de go2rtc.yaml ; caméras disponibles : %s",
			stream, strings.Join(cameraNames(sources), ", "))
	}
	switch n := len(sources[stream]); {
	case n == 0:
		return nil, fmt.Errorf("%s : aucune source homekit://", stream)
	case n > 1:
		return nil, fmt.Errorf("%s : %d sources homekit://, une seule attendue", stream, n)
	}

	cam, err := parse(sources[stream][0])
	if err != nil {
		return nil, fmt.Errorf("%s : %w", stream, err)
	}
	cam.Name, cam.Stream = name, stream
	return cam, nil
}

// ClientID renvoie l'identifiant du contrôleur (go2rtc). Il part chiffré
// dans le message M3 de pair-verify ; ne pas l'afficher.
func (c *Camera) ClientID() string { return c.clientID }

// Sign signe data avec la clé privée du contrôleur, qui ne quitte jamais
// ce paquet.
func (c *Camera) Sign(data []byte) []byte {
	return ed25519.Sign(c.clientPrivate, data)
}

// VerifyDevice dit si sig est une signature de data par la caméra
// (device_public, 32 octets garantis par Load).
func (c *Camera) VerifyDevice(data, sig []byte) bool {
	return ed25519.Verify(c.devicePublic, data, sig)
}

// String décrit la caméra sans ses clés.
func (c Camera) String() string {
	return fmt.Sprintf("%s (flux %s, device_id %s, adresse de l'URL %s)",
		c.Name, c.Stream, c.DeviceID, c.URLAddr)
}

// Format impose String à tous les verbes de fmt : sans lui, %v et %+v
// afficheraient les champs non exportés, clés comprises.
func (c Camera) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, c.String())
}

var deviceIDPattern = regexp.MustCompile(`^[0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}$`)

// parse extrait et valide le matériel d'une source homekit://.
// Format (go2rtc, pkg/hap/client.go) :
// homekit://IP:PORT?client_id=…&client_private=<hex>&device_id=…&device_public=<hex>#…
func parse(src string) (*Camera, error) {
	u, err := url.Parse(src)
	if err != nil {
		return nil, errors.New("URL homekit:// illisible (détail masqué)")
	}
	if u.Hostname() == "" || u.Port() == "" {
		return nil, errors.New("URL homekit:// sans hôte:port")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, errors.New("paramètres de l'URL homekit:// illisibles (détail masqué)")
	}
	get := func(key string) (string, error) {
		if vs := q[key]; len(vs) == 1 && vs[0] != "" {
			return vs[0], nil
		}
		return "", fmt.Errorf("paramètre %s absent, vide ou répété", key)
	}

	clientID, err := get("client_id")
	if err != nil {
		return nil, err
	}
	deviceID, err := get("device_id")
	if err != nil {
		return nil, err
	}
	if !deviceIDPattern.MatchString(deviceID) {
		return nil, errors.New("device_id mal formé (attendu XX:XX:XX:XX:XX:XX)")
	}
	priv, err := decodeKey(q, "client_private", ed25519.PrivateKeySize)
	if err != nil {
		return nil, err
	}
	// go2rtc signe avec la clé entière (pkg/hap/ed25519) : une moitié
	// publique qui ne correspond pas à la graine donne des signatures
	// que la caméra refuserait.
	if !bytes.Equal(ed25519.NewKeyFromSeed(priv[:ed25519.SeedSize]), priv) {
		return nil, errors.New("client_private incohérent : la moitié publique ne correspond pas à la graine")
	}
	// Sans device_public, go2rtc ne vérifie pas la signature de la caméra
	// (pkg/hap/client.go) : refus.
	pub, err := decodeKey(q, "device_public", ed25519.PublicKeySize)
	if err != nil {
		return nil, err
	}

	return &Camera{
		URLAddr:       u.Host,
		DeviceID:      deviceID,
		clientID:      clientID,
		clientPrivate: priv,
		devicePublic:  pub,
	}, nil
}

func decodeKey(q url.Values, key string, size int) ([]byte, error) {
	vs := q[key]
	if len(vs) != 1 || vs[0] == "" {
		return nil, fmt.Errorf("paramètre %s absent, vide ou répété", key)
	}
	b, err := hex.DecodeString(vs[0])
	if err != nil {
		return nil, fmt.Errorf("%s n'est pas hexadécimal", key)
	}
	if len(b) != size {
		return nil, fmt.Errorf("%s fait %d octets, %d attendus", key, len(b), size)
	}
	return b, nil
}

// registerSecrets enregistre les secrets d'une source sans l'analyser
// strictement : même une source mal formée doit rester masquée.
func registerSecrets(red *redact.Redactor, src string) {
	i := strings.IndexByte(src, '?')
	if i < 0 {
		return
	}
	query, _, _ := strings.Cut(src[i+1:], "#")
	red.Add(query)
	for _, kv := range strings.Split(query, "&") {
		key, value, _ := strings.Cut(kv, "=")
		switch strings.ToLower(key) {
		case "client_id", "client_private", "device_public":
		default:
			continue
		}
		red.Add(value)
		if unescaped, err := url.QueryUnescape(value); err == nil {
			red.Add(unescaped)
		}
		if b, err := hex.DecodeString(value); err == nil {
			red.AddBytes(b)
		}
	}
}

func sourcesOf(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func isHomekit(src string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(src)), "homekit://")
}

func cameraNames(sources map[string][]string) []string {
	var names []string
	for stream := range sources {
		if name, ok := strings.CutPrefix(stream, StreamPrefix); ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

var yamlLine = regexp.MustCompile(`line (\d+)`)

// yamlError ne garde de l'erreur YAML que le numéro de ligne : le reste
// peut citer un extrait du fichier.
func yamlError(err error) error {
	if m := yamlLine.FindStringSubmatch(err.Error()); m != nil {
		return fmt.Errorf("go2rtc.yaml : YAML illisible ligne %s (détail masqué)", m[1])
	}
	return errors.New("go2rtc.yaml : YAML illisible (détail masqué)")
}
