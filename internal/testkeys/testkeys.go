// Package testkeys fabrique, à l'exécution, des clés d'appairage factices
// au format de go2rtc. Réservé aux tests : aucune clé, réelle ou factice,
// n'est écrite dans le dépôt.
package testkeys

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"testing"
)

// Pairing réunit le matériel d'appairage factice d'une caméra :
// côté contrôleur (go2rtc) et côté accessoire (la caméra).
type Pairing struct {
	ClientID      string
	ClientPrivate ed25519.PrivateKey // 64 octets : graine puis clé publique
	DeviceID      string
	DevicePrivate ed25519.PrivateKey
}

// New tire un nouvel appairage factice.
func New(t testing.TB) Pairing {
	t.Helper()
	_, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, devicePriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return Pairing{
		ClientID:      uuid(t),
		ClientPrivate: clientPriv,
		DeviceID:      deviceID(t),
		DevicePrivate: devicePriv,
	}
}

// ClientPublic renvoie la clé publique du contrôleur.
func (p Pairing) ClientPublic() ed25519.PublicKey {
	return p.ClientPrivate.Public().(ed25519.PublicKey)
}

// DevicePublic renvoie la clé publique de la caméra.
func (p Pairing) DevicePublic() ed25519.PublicKey {
	return p.DevicePrivate.Public().(ed25519.PublicKey)
}

// URL écrit la source homekit:// telle que go2rtc la range dans son YAML.
func (p Pairing) URL(hostport string) string {
	return fmt.Sprintf(
		"homekit://%s?client_id=%s&client_private=%x&device_id=%s&device_public=%x#bitrate=1920K",
		hostport, p.ClientID, []byte(p.ClientPrivate), p.DeviceID, []byte(p.DevicePublic()),
	)
}

func uuid(t testing.TB) string {
	b := random(t, 16)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func deviceID(t testing.TB) string {
	b := random(t, 6)
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", b[0], b[1], b[2], b[3], b[4], b[5])
}

func random(t testing.TB, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}
