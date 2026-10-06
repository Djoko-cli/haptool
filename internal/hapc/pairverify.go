package hapc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/hap"
	"github.com/AlexxIT/go2rtc/pkg/hap/chacha20poly1305"
	"github.com/AlexxIT/go2rtc/pkg/hap/curve25519"
	"github.com/AlexxIT/go2rtc/pkg/hap/hkdf"
	"github.com/AlexxIT/go2rtc/pkg/hap/tlv8"

	"github.com/Djoko-cli/haptool/internal/creds"
)

// pairVerify authentifie la caméra puis ouvre le canal chiffré sur conn.
//
// Séquence reprise de go2rtc v1.9.14, pkg/hap/client.go, Client.Dial
// (licence MIT, © 2022 Alexey Khit, texte dans THIRD_PARTY_NOTICES.md),
// avec trois différences :
//   - ni découverte mDNS ni composition TCP : conn est fourni ;
//   - un délai sur chaque échange ;
//   - l'identité de la caméra (identifiant et signature) est toujours
//     vérifiée avant l'envoi de M3, qui porte celle du contrôleur.
func pairVerify(conn net.Conn, cam *creds.Camera, addr string, timeout time.Duration) (net.Conn, error) {
	br := bufio.NewReader(conn)

	// M1 : notre clé publique de session.
	sessionPublic, sessionPrivate := curve25519.GenerateKeyPair()
	data, err := exchange(conn, br, addr, timeout, struct {
		PublicKey string `tlv8:"3"`
		State     byte   `tlv8:"6"`
	}{string(sessionPublic), hap.StateM1})
	if err != nil {
		return nil, fmt.Errorf("M1 : %w", err)
	}

	// M2 : clé publique de session de la caméra et preuve d'identité.
	var m2 struct {
		PublicKey     string `tlv8:"3"`
		EncryptedData string `tlv8:"5"`
		State         byte   `tlv8:"6"`
		Error         byte   `tlv8:"7"`
	}
	if err := tlv8.Unmarshal(data, &m2); err != nil {
		return nil, fmt.Errorf("M2 illisible : %w", err)
	}
	if m2.Error != 0 {
		return nil, fmt.Errorf("la caméra refuse pair-verify (erreur TLV %d)", m2.Error)
	}
	if m2.State != hap.StateM2 {
		return nil, fmt.Errorf("M2 : état %d inattendu", m2.State)
	}
	shared, err := curve25519.SharedSecret(sessionPrivate, []byte(m2.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("M2 : %w", err)
	}
	sessionKey, err := hkdf.Sha512(shared, "Pair-Verify-Encrypt-Salt", "Pair-Verify-Encrypt-Info")
	if err != nil {
		return nil, err
	}
	plain, err := chacha20poly1305.Decrypt(sessionKey, "PV-Msg02", []byte(m2.EncryptedData))
	if err != nil {
		return nil, fmt.Errorf("M2 indéchiffrable : %w", err)
	}
	var proof struct {
		Identifier string `tlv8:"1"`
		Signature  string `tlv8:"10"`
	}
	if err := tlv8.Unmarshal(plain, &proof); err != nil {
		return nil, fmt.Errorf("M2 illisible : %w", err)
	}
	if !strings.EqualFold(proof.Identifier, cam.DeviceID) {
		return nil, fmt.Errorf("l'appareil se présente comme %q, %s attendu", proof.Identifier, cam.DeviceID)
	}
	if !cam.VerifyDevice(hap.Append(m2.PublicKey, proof.Identifier, sessionPublic), []byte(proof.Signature)) {
		return nil, errors.New("signature de la caméra invalide : l'appareil joint n'est pas elle")
	}

	// M3 : notre identité, signée puis chiffrée.
	inner, err := tlv8.Marshal(struct {
		Identifier string `tlv8:"1"`
		Signature  string `tlv8:"10"`
	}{cam.ClientID(), string(cam.Sign(hap.Append(sessionPublic, cam.ClientID(), m2.PublicKey)))})
	if err != nil {
		return nil, err
	}
	encrypted, err := chacha20poly1305.Encrypt(sessionKey, "PV-Msg03", inner)
	if err != nil {
		return nil, err
	}
	data, err = exchange(conn, br, addr, timeout, struct {
		EncryptedData string `tlv8:"5"`
		State         byte   `tlv8:"6"`
	}{string(encrypted), hap.StateM3})
	if err != nil {
		return nil, fmt.Errorf("M3 : %w", err)
	}

	// M4 : accord de la caméra.
	var m4 struct {
		State byte `tlv8:"6"`
		Error byte `tlv8:"7"`
	}
	if err := tlv8.Unmarshal(data, &m4); err != nil {
		return nil, fmt.Errorf("M4 illisible : %w", err)
	}
	if m4.Error != 0 {
		return nil, fmt.Errorf("la caméra refuse notre identité (erreur TLV %d)", m4.Error)
	}
	if m4.State != hap.StateM4 {
		return nil, fmt.Errorf("M4 : état %d inattendu", m4.State)
	}

	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	return hap.NewConn(conn, bufio.NewReadWriter(br, bufio.NewWriter(conn)), shared, true)
}

// maxPairVerifyBody borne la lecture d'une réponse avant authentification.
const maxPairVerifyBody = 64 << 10

// exchange envoie en clair un message TLV8 de pair-verify et renvoie le
// corps de la réponse. Le corps a une longueur connue : jamais d'envoi
// « chunked », que les accessoires HAP ne comprennent pas.
func exchange(conn net.Conn, br *bufio.Reader, addr string, timeout time.Duration, msg any) ([]byte, error) {
	if err := allowed(http.MethodPost, hap.PathPairVerify); err != nil {
		return nil, err
	}
	body, err := tlv8.Marshal(msg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+addr+hap.PathPairVerify, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", hap.MimeTLV8)
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	if err := req.Write(conn); err != nil {
		return nil, err
	}
	res, err := http.ReadResponse(br, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, maxPairVerifyBody))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("statut HTTP %d %s", res.StatusCode, strconv.QuoteToASCII(statusText(res)))
	}
	return data, nil
}
