package redact

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode"
)

// Writer expurge ligne à ligne ce qui le traverse, neutralise les caractères
// de contrôle, puis écrit le résultat dans w.
// Une ligne n'est émise qu'une fois complète : une clé coupée entre deux
// appels à Write reste donc masquée. Flush émet la ligne incomplète qui
// reste ; il faut l'appeler avant de quitter.
type Writer struct {
	r *Redactor
	w io.Writer

	mu  sync.Mutex
	buf []byte
}

// NewWriter renvoie un Writer qui expurge vers w.
func (r *Redactor) NewWriter(w io.Writer) *Writer {
	return &Writer{r: r, w: w}
}

// Write met p en attente et émet, expurgées, les lignes devenues complètes.
func (lw *Writer) Write(p []byte) (int, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	lw.buf = append(lw.buf, p...)
	i := bytes.LastIndexByte(lw.buf, '\n')
	if i < 0 {
		return len(p), nil
	}
	// Les motifs et les secrets ne contiennent pas de retour à la ligne :
	// expurger le bloc d'un coup équivaut à l'expurger ligne par ligne.
	complete := string(lw.buf[:i+1])
	lw.buf = append(lw.buf[:0], lw.buf[i+1:]...)
	if _, err := io.WriteString(lw.w, neutralize(lw.r.String(complete))); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Flush émet, expurgée, la ligne incomplète en attente.
func (lw *Writer) Flush() error {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	if len(lw.buf) == 0 {
		return nil
	}
	rest := string(lw.buf)
	lw.buf = lw.buf[:0]
	_, err := io.WriteString(lw.w, neutralize(lw.r.String(rest)))
	return err
}

// neutralize remplace par leur forme échappée (\x1b, \u202e…) les
// caractères de contrôle et de mise en forme bidirectionnelle, sauf la
// tabulation et le retour à la ligne : une chaîne venue du réseau ne doit
// ni piloter le terminal ni tromper le lecteur.
func neutralize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, "\\x%02x", r)
		case unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r):
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
