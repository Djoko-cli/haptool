package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Clôture : la frontière de sécurité (docs/CONCEPTION.md), vérifiée comme un linter.
// Le code de production ne doit citer ni l'appairage HAP (/pair-setup,
// /pairings : ajout, suppression ou liste des contrôleurs, qui peuvent
// désappairer go2rtc), ni /resource, ni Client.Dial de go2rtc (mDNS dans
// le processus), ni, en phase A, la moindre écriture. Le test
// TestSessionSendsOnlyAllowedRequests vérifie le même périmètre en
// fonctionnement ; celui-ci couvre aussi le code qu'aucun test n'exécute.
var forbidden = []string{
	`PathPairSetup`, `PathPairings`, `PathResource`,
	`"/pair-setup"`, `"/pairings"`, `"/resource"`,
	`hap.Pair(`, `hap.Unpair(`, `.Pair(`, `.ListPairings(`, `.PairingsAdd(`, `.DeletePairing(`,
	`hap.Dial(`, `.GetImage(`, `.PutCharacters(`, `hap.Client{`,
	`go2rtc/pkg/mdns`,
	`"PUT"`, `.Put(`, // écritures : http.MethodPut, via la liste blanche seulement
	// Motifs nus, pour attraper aussi une requête écrite à la main
	// (fmt.Fprintf(conn, "PUT %s …")) ou une construction indirecte.
	`/pairings`, `/pair-setup`, `/resource`, `/prepare`, `PUT `, `hap.Client`, `NewStream(`,
}

func TestProductionCodeStaysInsideHAPPerimeter(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Hors production : utilitaires de test et générateur.
			switch path {
			case ".git", "internal/fakeacc", "internal/testkeys", "internal/catalog/gen":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, f := range forbidden {
			if strings.Contains(string(src), f) {
				t.Errorf("%s cite %s, hors du périmètre HAP autorisé", path, f)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
