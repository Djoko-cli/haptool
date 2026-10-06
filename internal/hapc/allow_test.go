package hapc

import "testing"

// Défaut attrapé (relecture Opus, point 4) : une requête hors périmètre
// (écriture, appairage, ressource) partirait vers la caméra.
func TestAllowedRequests(t *testing.T) {
	for _, ok := range [][2]string{
		{"POST", "/pair-verify"}, {"GET", "/accessories"}, {"GET", "/characteristics?id=1.10&meta=1"},
		{"PUT", "/characteristics"},
	} {
		if err := allowed(ok[0], ok[1]); err != nil {
			t.Errorf("%s %s refusée : %v", ok[0], ok[1], err)
		}
	}
	for _, bad := range [][2]string{
		{"PUT", "/characteristics?id=1.2"}, {"PUT", "/accessories"}, {"POST", "/pairings"}, {"POST", "/pair-setup"}, {"POST", "/resource"},
		{"GET", "/pairings"}, {"PUT", "/prepare"}, {"GET", "/accessories/../pairings"}, {"get", "/accessories"},
	} {
		if err := allowed(bad[0], bad[1]); err == nil {
			t.Errorf("%s %s acceptée", bad[0], bad[1])
		}
	}
}
