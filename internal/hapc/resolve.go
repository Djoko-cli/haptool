package hapc

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/hap"
)

// Resolver trouve l'adresse d'une caméra sans ouvrir de socket d'écoute :
// le mDNS passe par le démon système (dns-sd). Ses fonctions sont
// remplaçables pour les tests.
type Resolver struct {
	Browse    func(ctx context.Context) (string, error)                // sortie de « dns-sd -Z _hap._tcp local. »
	LookupIP  func(ctx context.Context, host string) ([]net.IP, error) // résolution d'un hôte .local
	ScanPorts []int                                                    // ports balayés sur l'hôte de l'URL
}

// DefaultResolver interroge le démon mDNS de macOS et balaie les ports
// 49150 à 49160, où les Circle View ouvrent leur serveur HAP.
func DefaultResolver() *Resolver {
	var ports []int
	for p := 49150; p <= 49160; p++ {
		ports = append(ports, p)
	}
	return &Resolver{Browse: browseDNSSD, LookupIP: lookupIPv4, ScanPorts: ports}
}

const browseDuration = 3 * time.Second

// browseDNSSD liste les annonces _hap._tcp pendant browseDuration. dns-sd
// ne s'arrête jamais seul : son interruption à l'échéance est normale.
func browseDNSSD(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, browseDuration)
	defer cancel()
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, "/usr/bin/dns-sd", "-Z", "_hap._tcp", "local.")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil && ctx.Err() == nil {
		return "", err
	}
	return out.String(), nil
}

func lookupIPv4(ctx context.Context, host string) ([]net.IP, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return net.DefaultResolver.LookupIP(ctx, "ip4", host)
}

// announce réunit les enregistrements SRV et TXT d'une instance _hap._tcp.
type announce struct {
	host string
	port int
	id   string
}

var txtID = regexp.MustCompile(`"id=([^"]*)"`)

// parseDNSSD lit, par nom d'instance, les enregistrements SRV
// (« nom SRV priorité poids port hôte ») et l'id du TXT dans la sortie de
// dns-sd -Z. Les doublons et les commentaires sont sans effet.
func parseDNSSD(out string) map[string]*announce {
	found := map[string]*announce{}
	get := func(name string) *announce {
		if found[name] == nil {
			found[name] = &announce{}
		}
		return found[name]
	}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || strings.HasPrefix(f[0], ";") {
			continue
		}
		switch f[1] {
		case "SRV":
			if len(f) < 6 {
				continue
			}
			if port, err := strconv.Atoi(f[4]); err == nil {
				a := get(f[0])
				a.port, a.host = port, f[5]
			}
		case "TXT":
			if m := txtID.FindStringSubmatch(line); m != nil {
				get(f[0]).id = m[1]
			}
		}
	}
	return found
}

// mdns renvoie les adresses que la caméra deviceID annonce en mDNS, toutes
// annonces confondues : un autre appareil peut s'annoncer avec le même id,
// public, et pair-verify départagera. La caméra est reconnue à son id, en
// toute casse, jamais à son nom ; un hôte non résolu est sauté.
func (r *Resolver) mdns(ctx context.Context, deviceID string) ([]string, error) {
	out, err := r.Browse(ctx)
	if err != nil {
		return nil, fmt.Errorf("dns-sd : %w", err)
	}
	var addrs, unresolved []string
	for _, a := range parseDNSSD(out) {
		if !strings.EqualFold(a.id, deviceID) || a.host == "" {
			continue
		}
		ips, err := r.LookupIP(ctx, a.host)
		if err != nil || len(ips) == 0 {
			unresolved = append(unresolved, a.host)
			continue
		}
		addrs = append(addrs, net.JoinHostPort(ips[0].String(), strconv.Itoa(a.port)))
	}
	if len(addrs) == 0 && len(unresolved) > 0 {
		return nil, fmt.Errorf("hôtes non résolus : %s", strings.Join(unresolved, ", "))
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("aucune annonce _hap._tcp avec l'id %s", deviceID)
	}
	return addrs, nil
}

// scan renvoie, dans l'ordre des ports, les adresses de host qui répondent
// comme un serveur HAP ; skip, déjà essayée, est sautée.
func (r *Resolver) scan(ctx context.Context, host, skip string, timeout time.Duration) []string {
	hits := make([]string, len(r.ScanPorts))
	var wg sync.WaitGroup
	for i, port := range r.ScanPorts {
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		if addr == skip {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if probeHAP(ctx, addr, timeout) {
				hits[i] = addr
			}
		}()
	}
	wg.Wait()
	var found []string
	for _, addr := range hits {
		if addr != "" {
			found = append(found, addr)
		}
	}
	return found
}

// probeHAP dit si addr répond 470 (session chiffrée exigée) à un
// GET /accessories en clair : la marque d'un serveur HAP.
func probeHAP(ctx context.Context, addr string, timeout time.Duration) bool {
	if allowed(http.MethodGet, hap.PathAccessories) != nil {
		return false
	}
	conn, err := dial(ctx, addr, timeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return false
	}
	if _, err := fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\n\r\n", hap.PathAccessories, addr); err != nil {
		return false
	}
	res, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return false
	}
	_ = res.Body.Close()
	return res.StatusCode == 470
}
