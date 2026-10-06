// Package redact masque les clés d'appairage dans tout texte destiné
// à une sortie.
package redact

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

// Mask remplace chaque secret masqué.
const Mask = "<REDACTED>"

// En deçà de cette longueur, un « secret » masquerait du texte ordinaire.
const minSecretLen = 6

var (
	// La requête d'une URL homekit:// porte les clés : tout ce qui suit le
	// « ? » est masqué, jusqu'à un blanc, un guillemet ou un chevron.
	urlQuery = regexp.MustCompile("(?i)(homekit://[^?\\s\"'`<>]*)\\?[^\\s\"'`<>]*")

	// Paramètres de clé hors d'une URL complète, « = » encodé ou non.
	keyParam = regexp.MustCompile("(?i)\\b(client_private|device_public|client_id)(=|%3D)[^&\\s\"'`<>#]*")
)

// Redactor masque les secrets enregistrés et les motifs de clés connus.
// Il est sûr en accès concurrent.
type Redactor struct {
	mu      sync.RWMutex
	secrets map[string]struct{}
	values  *regexp.Regexp // nil tant qu'aucun secret n'est enregistré
}

// New renvoie un filtre sans secret enregistré.
func New() *Redactor {
	return &Redactor{secrets: map[string]struct{}{}}
}

// Add enregistre un secret, masqué ensuite partout et en toute casse.
// Les chaînes trop courtes pour être des clés sont ignorées. Une chaîne
// qui n'est pas de l'UTF-8 valide (par exemple « %FF » décodé) ne peut pas
// entrer dans une expression régulière : elle est enregistrée sous sa
// forme hexadécimale. Add ne panique jamais, car un message de panique
// recopierait les secrets.
func (r *Redactor) Add(secret string) {
	if !utf8.ValidString(secret) {
		secret = hex.EncodeToString([]byte(secret))
	}
	if len(secret) < minSecretLen {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.secrets[secret]; ok {
		return
	}
	r.secrets[secret] = struct{}{}

	// Les plus longs d'abord : une forme courte (base64 sans bourrage)
	// ne doit pas masquer seulement le début d'une forme longue.
	all := make([]string, 0, len(r.secrets))
	for s := range r.secrets {
		all = append(all, s)
	}
	sort.Slice(all, func(i, j int) bool { return len(all[i]) > len(all[j]) })
	for i, s := range all {
		all[i] = regexp.QuoteMeta(s)
	}
	re, err := regexp.Compile("(?i)" + strings.Join(all, "|"))
	if err != nil {
		// Impossible avec des chaînes UTF-8 échappées ; l'erreur, qui cite
		// l'expression, n'est jamais affichée.
		delete(r.secrets, secret)
		return
	}
	r.values = re
}

// AddBytes enregistre une clé binaire sous ses encodages usuels :
// hexadécimal, base64 (standard et URL, avec ou sans bourrage) et
// liste décimale, celle qu'affiche %v.
func (r *Redactor) AddBytes(b []byte) {
	r.Add(hex.EncodeToString(b))
	r.Add(base64.StdEncoding.EncodeToString(b))
	r.Add(base64.RawStdEncoding.EncodeToString(b))
	r.Add(base64.URLEncoding.EncodeToString(b))
	r.Add(base64.RawURLEncoding.EncodeToString(b))
	r.Add(fmt.Sprint(b))
}

// String renvoie s expurgé.
func (r *Redactor) String(s string) string {
	s = urlQuery.ReplaceAllString(s, "${1}?"+Mask)
	s = keyParam.ReplaceAllString(s, "${1}${2}"+Mask)

	r.mu.RLock()
	values := r.values
	r.mu.RUnlock()
	if values != nil {
		s = values.ReplaceAllLiteralString(s, Mask)
	}
	return s
}
