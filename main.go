// Commande haptool : lit, avec les clés d'appairage de go2rtc, les
// services et caractéristiques HomeKit des Circle View (dump, get), et
// règle leur voyant d'état (set, à blanc par défaut). Conception :
// docs/CONCEPTION.md.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Djoko-cli/haptool/internal/catalog"
	"github.com/Djoko-cli/haptool/internal/creds"
	"github.com/Djoko-cli/haptool/internal/guard"
	"github.com/Djoko-cli/haptool/internal/hapc"
	"github.com/Djoko-cli/haptool/internal/redact"
	"github.com/Djoko-cli/haptool/internal/report"
)

// watchdog borne la durée totale d'une commande, en plus du délai de
// chaque échange réseau.
const watchdog = 2 * time.Minute

const usage = `usage :
  haptool dump [--config F] [--addr hôte:port] [--json | --md] <caméra>...
  haptool get  [--config F] [--addr hôte:port] <caméra> <aid.iid>[,<aid.iid>…]
  haptool set  [--config F] [--addr hôte:port] [--confirm --expect TYPE:ACTUELLE] <caméra> <aid.iid> <valeur>

Le fichier de go2rtc vient de --config, ou à défaut de la variable HAPTOOL_CONFIG.
<caméra> vaut cam1 ou cam2 : le flux go2rtc circleview_<caméra>.
Le fichier de go2rtc n'est jamais écrit. set est à blanc par défaut : il annonce
l'écriture et son retour arrière ; seul --confirm écrit, une caractéristique à la fois.`

// env porte les sorties et les dépendances remplaçables d'une exécution.
type env struct {
	red      *redact.Redactor // secrets à masquer, alimenté par creds
	stdout   io.Writer
	stderr   io.Writer
	resolver *hapc.Resolver // nil : résolution par défaut
	now      func() time.Time
	flush    func() // vide les sorties filtrées
}

// newEnv branche les sorties sur le filtre d'expurgation : tout ce que
// l'outil écrit, journal et paniques compris, passe par lui.
func newEnv(stdout, stderr io.Writer) env {
	red := redact.New()
	out, errw := red.NewWriter(stdout), red.NewWriter(stderr)
	return env{red: red, stdout: out, stderr: errw, now: time.Now,
		flush: func() { _ = out.Flush(); _ = errw.Flush() }}
}

func main() {
	e := newEnv(os.Stdout, os.Stderr)
	log.SetOutput(e.stderr)
	log.SetFlags(0)
	time.AfterFunc(watchdog, func() {
		fmt.Fprintf(e.stderr, "délai global de %s dépassé : abandon\n", watchdog)
		e.flush()
		os.Exit(3)
	})
	code := run(context.Background(), os.Args[1:], e)
	e.flush()
	os.Exit(code)
}

// run exécute une commande et renvoie le code de sortie : 0 succès,
// 1 échec, 2 usage incorrect ou panique.
func run(ctx context.Context, args []string, e env) (code int) {
	defer func() {
		if r := recover(); r != nil {
			// Seules les erreurs d'exécution de Go ont un message sûr (indices,
			// pointeurs nuls) ; toute autre panique pourrait recopier une
			// donnée, voire une clé : message fixe.
			if rerr, ok := r.(runtime.Error); ok {
				fmt.Fprintf(e.stderr, "panique : %v\n", rerr)
			} else {
				fmt.Fprintln(e.stderr, "panique (détail masqué)")
			}
			code = 2
		}
	}()
	if e.red == nil {
		e.red = redact.New()
	}
	if e.now == nil {
		e.now = time.Now
	}
	if len(args) == 0 {
		fmt.Fprintln(e.stderr, usage)
		return 2
	}

	fs := flag.NewFlagSet("haptool "+args[0], flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	config := fs.String("config", os.Getenv("HAPTOOL_CONFIG"), "fichier de go2rtc, lu et jamais écrit (défaut : $HAPTOOL_CONFIG)")
	addr := fs.String("addr", "", "adresse hôte:port imposée, pour une seule caméra")
	asJSON := fs.Bool("json", false, "dump : inventaire en JSON")
	asMD := fs.Bool("md", false, "dump : inventaire en Markdown, différences comprises")
	confirm := fs.Bool("confirm", false, "set : écrire réellement (sinon, à blanc)")
	expect := fs.String("expect", "", "set --confirm : TYPE:VALEUR_ACTUELLE annoncés à blanc, vérifiés avant d'écrire")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *config == "" {
		fmt.Fprintln(e.stderr, "fichier de go2rtc inconnu : --config ou variable HAPTOOL_CONFIG")
		return 2
	}
	opts := hapc.Options{Addr: *addr, Resolver: e.resolver}

	switch {
	case args[0] == "dump" && !*confirm && *expect == "":
		return dump(ctx, e, *config, opts, fs.Args(), *asJSON, *asMD)
	case args[0] == "get" && !*asJSON && !*asMD && !*confirm && *expect == "":
		return get(ctx, e, *config, opts, fs.Args())
	case args[0] == "set" && !*asJSON && !*asMD:
		return set(ctx, e, *config, opts, fs.Args(), *confirm, *expect)
	}
	fmt.Fprintln(e.stderr, usage)
	return 2
}

func dump(ctx context.Context, e env, config string, opts hapc.Options, names []string, asJSON, asMD bool) int {
	switch {
	case len(names) == 0, asJSON && asMD:
		fmt.Fprintln(e.stderr, usage)
		return 2
	case opts.Addr != "" && len(names) > 1:
		fmt.Fprintln(e.stderr, "--addr ne vaut que pour une seule caméra")
		return 2
	}

	var cams []report.Camera
	for _, name := range names {
		cam, s, err := open(ctx, e, config, name, opts)
		if err != nil {
			fmt.Fprintf(e.stderr, "%s : %v\n", name, err)
			return 1
		}
		accs, err := readAccessories(s)
		if err != nil {
			fmt.Fprintf(e.stderr, "%s : %v\n", name, err)
			return 1
		}
		cams = append(cams, report.Camera{Name: cam.Name, DeviceID: cam.DeviceID, Addr: s.Addr,
			Source: s.Source, URLAddr: cam.URLAddr, Accessories: accs})
	}

	var err error
	switch {
	case asJSON:
		err = report.JSON(e.stdout, cams)
	case asMD:
		err = report.Markdown(e.stdout, cams, e.now())
	default:
		for i, c := range cams {
			if i > 0 {
				fmt.Fprintln(e.stdout)
			}
			if err = report.Text(e.stdout, c); err != nil {
				break
			}
		}
	}
	if err != nil {
		fmt.Fprintf(e.stderr, "écriture : %v\n", err)
		return 1
	}
	return 0
}

func get(ctx context.Context, e env, config string, opts hapc.Options, args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(e.stderr, usage)
		return 2
	}
	ids, err := hapc.ParseIDs(args[1])
	if err != nil {
		fmt.Fprintln(e.stderr, err)
		return 2
	}
	cam, s, err := open(ctx, e, config, args[0], opts)
	if err != nil {
		fmt.Fprintf(e.stderr, "%s : %v\n", args[0], err)
		return 1
	}
	defer s.Close()
	accs, err := s.Accessories()
	if err == nil {
		var chars []hapc.Characteristic
		if chars, err = s.Characteristics(ids); err == nil {
			err = report.Characteristics(e.stdout, report.Camera{Name: cam.Name, Addr: s.Addr, Source: s.Source},
				withMetadata(accs, chars))
		}
	}
	if err != nil {
		fmt.Fprintf(e.stderr, "%s : %v\n", args[0], err)
		return 1
	}
	return 0
}

// withMetadata complète des caractéristiques lues par GET /characteristics
// avec les métadonnées de /accessories (format, droits, unité, bornes).
func withMetadata(accs []hapc.Accessory, chars []hapc.Characteristic) []hapc.Characteristic {
	out := make([]hapc.Characteristic, len(chars))
	for i, c := range chars {
		if _, m, n := find(accs, hapc.ID{AID: c.AID, IID: c.IID}); n == 1 {
			if c.Type == "" {
				c.Type = m.Type
			}
			c.Format, c.Perms, c.Unit, c.Description = m.Format, m.Perms, m.Unit, m.Description
			c.MinValue, c.MaxValue, c.MinStep, c.MaxLen = m.MinValue, m.MaxValue, m.MinStep, m.MaxLen
			c.MaxDataLen, c.ValidValues, c.ValidRange = m.MaxDataLen, m.ValidValues, m.ValidRange
		}
		out[i] = c
	}
	return out
}

// open lit les clés de la caméra, puis ouvre une session de contrôle.
func open(ctx context.Context, e env, config, name string, opts hapc.Options) (*creds.Camera, *hapc.Session, error) {
	cam, err := creds.Load(config, name, e.red)
	if err != nil {
		return nil, nil, err
	}
	s, err := hapc.Open(ctx, cam, opts)
	if err != nil {
		return nil, nil, err
	}
	return cam, s, nil
}

// readAccessories referme la session dans tous les cas, panique comprise :
// pas de socket laissé ouvert vers la caméra.
func readAccessories(s *hapc.Session) ([]hapc.Accessory, error) {
	defer s.Close()
	return s.Accessories()
}

// set annonce, puis avec confirm effectue, l'écriture d'une seule
// caractéristique : contrôle par guard, valeur actuelle, nouvelle valeur,
// retour arrière, statut HTTP et HAP, relecture. --confirm exige --expect
// TYPE:ACTUELLE, tel qu'annoncé à blanc : la cible écrite est celle que
// le propriétaire a approuvée. Codes : 0 succès ou à blanc, 1 échec de la caméra ou
// du réseau, 2 refus ou usage incorrect.
func set(ctx context.Context, e env, config string, opts hapc.Options, args []string, confirm bool, expect string) int {
	if len(args) != 3 {
		fmt.Fprintln(e.stderr, usage)
		return 2
	}
	ids, err := hapc.ParseIDs(args[1])
	if err != nil || len(ids) != 1 {
		fmt.Fprintln(e.stderr, "set : une seule caractéristique, sous la forme aid.iid")
		return 2
	}
	id := ids[0]
	cam, s, err := open(ctx, e, config, args[0], opts)
	if err != nil {
		fmt.Fprintf(e.stderr, "%s : %v\n", args[0], err)
		return 1
	}
	defer s.Close()

	accs, err := s.Accessories()
	if err != nil {
		fmt.Fprintf(e.stderr, "%s : %v\n", args[0], err)
		return 1
	}
	svc, c, n := find(accs, id)
	if n != 1 {
		fmt.Fprintf(e.stderr, "%s : caractéristique %s trouvée %d fois, une seule attendue\n", args[0], id, n)
		return 2
	}
	value, err := guard.Value(svc, c, args[2])
	if err != nil {
		fmt.Fprintf(e.stderr, "%s %s : %v\n", args[0], id, err)
		return 2
	}
	current, ok := readOne(s, id)
	if !ok {
		fmt.Fprintf(e.stderr, "%s : lecture de la valeur actuelle de %s impossible\n", args[0], id)
		return 1
	}
	typ := catalog.Short(c.Type)

	fmt.Fprintf(e.stdout, "caméra %s · adresse %s (%s)\n", cam.Name, s.Addr, s.Source)
	fmt.Fprintf(e.stdout, "caractéristique %s : %s · service %s\n", id, report.CharLabel(c.Type), report.ServiceLabel(svc.Type))
	fmt.Fprintf(e.stdout, "format %s · droits %s · min %s · max %s · pas %s\n",
		c.Format, strings.Join(c.Perms, ","), orDash(c.MinValue), orDash(c.MaxValue), orDash(c.MinStep))
	fmt.Fprintf(e.stdout, "valeur actuelle : %s\n", report.Value(current))
	fmt.Fprintf(e.stdout, "nouvelle valeur : %s\n", value)
	if same(c.Format, current.Value, value) {
		fmt.Fprintln(e.stdout, "la caractéristique a déjà cette valeur : rien à écrire")
		return 0
	}
	cur, curOK := cliValue(c.Format, current.Value)
	if !curOK {
		fmt.Fprintln(e.stderr, "valeur actuelle non représentable en ligne de commande : écriture refusée")
		return 2
	}
	newV, _ := cliValue(c.Format, value)
	rollback := fmt.Sprintf("haptool set --confirm --expect %s:%s %s %s %s", typ, newV, cam.Name, id, cur)
	wanted := typ + ":" + cur
	fmt.Fprintf(e.stdout, "pour écrire : haptool set --confirm --expect %s %s %s %s\n", wanted, cam.Name, id, newV)
	fmt.Fprintf(e.stdout, "retour arrière : %s\n", rollback)
	if !confirm {
		fmt.Fprintln(e.stdout, "à blanc : rien n'a été écrit ; ajouter --confirm --expect pour écrire")
		return 0
	}
	if expect != wanted {
		fmt.Fprintf(e.stderr, "--expect %s attendu (reçu %s) : la cible ou sa valeur n'est pas celle annoncée ; rien n'a été écrit\n",
			wanted, strconv.QuoteToASCII(expect))
		return 2
	}

	res, werr := s.Write(id, value)
	if werr == nil {
		fmt.Fprintf(e.stdout, "réponse : HTTP %d\n", res.HTTPStatus)
		for _, st := range res.Statuses {
			code := 0
			if st.Status != nil {
				code = *st.Status
			}
			fmt.Fprintf(e.stdout, "statut HAP %d.%d : %d %s\n", st.AID, st.IID, code, catalog.Status(code))
		}
	}
	after, readOK := readOne(s, id)
	switch {
	case werr != nil:
		fmt.Fprintf(e.stdout, "état inconnu : %v\n", werr)
	case !readOK:
		fmt.Fprintln(e.stdout, "état inconnu : relecture impossible")
	}
	if readOK {
		fmt.Fprintf(e.stdout, "valeur relue : %s\n", report.Value(after))
	}
	if werr != nil || !readOK || !accepted(res, id) || !same(c.Format, after.Value, value) {
		fmt.Fprintln(e.stdout, "échec : la nouvelle valeur n'est pas confirmée par la caméra")
		fmt.Fprintf(e.stdout, "retour arrière : %s\n", rollback)
		return 1
	}
	fmt.Fprintln(e.stdout, "écriture faite")
	return 0
}

// readOne relit une caractéristique et vérifie que la réponse la désigne.
func readOne(s *hapc.Session, id hapc.ID) (hapc.Characteristic, bool) {
	cs, err := s.Characteristics([]hapc.ID{id})
	if err != nil || len(cs) != 1 || cs[0].AID != id.AID || cs[0].IID != id.IID ||
		(cs[0].Status != nil && *cs[0].Status != 0) {
		return hapc.Characteristic{}, false
	}
	return cs[0], true
}

// accepted : 204, ou 207 avec un seul statut, présent, nul et visant id.
func accepted(r hapc.WriteResult, id hapc.ID) bool {
	if r.HTTPStatus == 204 && len(r.Statuses) == 0 {
		return true
	}
	if r.HTTPStatus != 207 || len(r.Statuses) != 1 {
		return false
	}
	st := r.Statuses[0]
	return st.AID == id.AID && st.IID == id.IID && st.Status != nil && *st.Status == 0
}

// find renvoie le service et la caractéristique désignés par id, et le
// nombre d'occurrences de id dans la base (1 attendu).
func find(accs []hapc.Accessory, id hapc.ID) (hapc.Service, hapc.Characteristic, int) {
	var svc hapc.Service
	var ch hapc.Characteristic
	n := 0
	for _, a := range accs {
		if a.AID != id.AID {
			continue
		}
		for _, s := range a.Services {
			for _, c := range s.Characteristics {
				if c.IID == id.IID {
					if n == 0 {
						svc, ch = s, c
					}
					n++
				}
			}
		}
	}
	return svc, ch, n
}

// same compare deux valeurs JSON au sens de HAP : un booléen peut se lire
// 0 ou 1, un nombre s'écrire 1 ou 1.0.
func same(format string, a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	norm := func(v any) any {
		switch v := v.(type) {
		case bool:
			if v {
				return 1.0
			}
			return 0.0
		}
		return v
	}
	if format == "bool" {
		x, y = norm(x), norm(y)
	}
	return x == y
}

// cliValue rend une valeur JSON telle qu'on la repasserait à set, sûre à
// coller dans un shell : un booléen en 0 ou 1, quelle que soit la forme que
// la caméra renvoie (true ou 1) ; un nombre tel quel ; une chaîne entre
// guillemets simples POSIX. Faux si la valeur est nulle ou porte un
// caractère de contrôle.
func cliValue(format string, v json.RawMessage) (string, bool) {
	var x any
	if json.Unmarshal(v, &x) != nil || x == nil {
		return "", false
	}
	if format == "bool" {
		switch x {
		case true, 1.0:
			return "1", true
		case false, 0.0:
			return "0", true
		}
		return "", false
	}
	str, isStr := x.(string)
	if !isStr {
		return string(v), true
	}
	for _, r := range str {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return "'" + strings.ReplaceAll(str, "'", `'\''`) + "'", true
}

func orDash(m json.RawMessage) string {
	if len(m) == 0 {
		return "—"
	}
	return string(m)
}
