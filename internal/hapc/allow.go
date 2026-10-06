package hapc

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/AlexxIT/go2rtc/pkg/hap"
)

// allowed applique la liste blanche (docs/CONCEPTION.md) : pair-verify, lectures de
// /accessories et /characteristics, et écriture sur /characteristics seul,
// sans paramètre. Toute requête envoyée à une caméra passe par ce contrôle,
// en plus du test de clôture.
func allowed(method, path string) error {
	p, _, _ := strings.Cut(path, "?")
	switch {
	case method == http.MethodPost && p == hap.PathPairVerify,
		method == http.MethodGet && (p == hap.PathAccessories || p == hap.PathCharacteristics),
		method == http.MethodPut && path == hap.PathCharacteristics:
		return nil
	}
	return fmt.Errorf("requête %s %s hors du périmètre autorisé", strconv.QuoteToASCII(method), strconv.QuoteToASCII(p))
}
