//go:build ignore

// Génère internal/catalog/tables.go à partir d'un clone de HAP-NodeJS
// (https://github.com/homebridge/HAP-NodeJS) pris sur un tag de version.
//
// Usage, depuis la racine du dépôt :
//
//	git clone --depth 1 --branch vX.Y.Z https://github.com/homebridge/HAP-NodeJS.git <dossier>
//	go run internal/catalog/gen/main.go <dossier> > internal/catalog/tables.go
//
// Fichiers lus, sous <dossier> :
//   - package.json : version, donc tag ;
//   - src/lib/definitions/ServiceDefinitions.ts : nom du service, d'après le
//     commentaire JSDoc de sa classe (l'appel super n'y porte pas de nom) ;
//   - src/lib/definitions/CharacteristicDefinitions.ts : nom de la
//     caractéristique, d'après le premier argument de super ;
//   - src/lib/HAPServer.ts : codes et identifiants de l'enum HAPStatus.
//
// Rien n'est inventé ni ignoré en silence. Une classe qui n'a pas la forme
// attendue, un UUID hors de la base Apple, un UUID porté par deux définitions
// sans arbitrage (serviceAliases), un statut sans libellé français ou un
// libellé sans statut arrêtent le programme, sans rien écrire sur la sortie
// standard. Seuls les libellés français des statuts (statusLabels) sont
// écrits à la main.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	servicesFile        = "src/lib/definitions/ServiceDefinitions.ts"
	characteristicsFile = "src/lib/definitions/CharacteristicDefinitions.ts"
	statusFile          = "src/lib/HAPServer.ts"

	// Suffixe de l'UUID de base d'Apple, après le premier groupe de 8 chiffres.
	appleSuffix = "-0000-1000-8000-0026BB765291"
)

// alias tranche un UUID que la source donne à deux définitions : keep est
// conservé, drop est écarté.
type alias struct{ keep, drop string }

// serviceAliases : TelevisionSpeaker reprend l'UUID 113 de Speaker, avec les
// mêmes caractéristiques plus deux optionnelles. On garde le nom le plus
// général, Speaker.
var serviceAliases = map[string]alias{
	"113": {keep: "Speaker", drop: "Television Speaker"},
}

// statusLabels donne le libellé français de chaque identifiant de HAPStatus,
// traduit du commentaire de l'enum.
var statusLabels = map[string]string{
	"SUCCESS":                       "succès",
	"INSUFFICIENT_PRIVILEGES":       "privilèges insuffisants",
	"SERVICE_COMMUNICATION_FAILURE": "échec de communication avec la caractéristique",
	"RESOURCE_BUSY":                 "ressource occupée, réessayer",
	"READ_ONLY_CHARACTERISTIC":      "écriture impossible, caractéristique en lecture seule",
	"WRITE_ONLY_CHARACTERISTIC":     "lecture impossible, caractéristique en écriture seule",
	"NOTIFICATION_NOT_SUPPORTED":    "notifications non prises en charge",
	"OUT_OF_RESOURCE":               "ressources insuffisantes pour traiter la requête",
	"OPERATION_TIMED_OUT":           "délai de l'opération dépassé",
	"RESOURCE_DOES_NOT_EXIST":       "ressource inexistante",
	"INVALID_VALUE_IN_REQUEST":      "valeur invalide dans la requête",
	"INSUFFICIENT_AUTHORIZATION":    "autorisation insuffisante",
	"NOT_ALLOWED_IN_CURRENT_STATE":  "opération non autorisée dans l'état actuel",
}

// definition est une classe de ServiceDefinitions.ts ou de
// CharacteristicDefinitions.ts.
type definition struct{ name, class, uuid string }

// row est une ligne d'une table de noms.
type row struct {
	key, name string
	note      string // alias écarté, le cas échéant
}

// status est un membre de l'enum HAPStatus.
type status struct {
	code  int
	ident string
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage : go run internal/catalog/gen/main.go <dossier du clone HAP-NodeJS> > internal/catalog/tables.go")
		os.Exit(2)
	}
	out, summary, err := generate(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen :", err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		fmt.Fprintln(os.Stderr, "gen :", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, summary)
}

func generate(root string) (out []byte, summary string, err error) {
	version, err := readVersion(root)
	if err != nil {
		return nil, "", err
	}
	srcServices, err := readSource(root, servicesFile)
	if err != nil {
		return nil, "", err
	}
	srcCharacteristics, err := readSource(root, characteristicsFile)
	if err != nil {
		return nil, "", err
	}
	srcStatus, err := readSource(root, statusFile)
	if err != nil {
		return nil, "", err
	}

	serviceDefs, err := parseServices(srcServices)
	if err != nil {
		return nil, "", fmt.Errorf("%s : %w", servicesFile, err)
	}
	characteristicDefs, err := parseCharacteristics(srcCharacteristics)
	if err != nil {
		return nil, "", fmt.Errorf("%s : %w", characteristicsFile, err)
	}
	statuses, err := parseStatuses(srcStatus)
	if err != nil {
		return nil, "", fmt.Errorf("%s : %w", statusFile, err)
	}

	serviceRows, err := buildRows("service", serviceDefs, serviceAliases)
	if err != nil {
		return nil, "", err
	}
	characteristicRows, err := buildRows("caractéristique", characteristicDefs, nil)
	if err != nil {
		return nil, "", err
	}
	labels, err := statusLines(statuses)
	if err != nil {
		return nil, "", err
	}

	src := render(version, serviceRows, characteristicRows, statuses, labels)
	out, err = format.Source(src)
	if err != nil {
		return nil, "", fmt.Errorf("code généré invalide : %w", err)
	}
	summary = fmt.Sprintf("gen : HAP-NodeJS %s : %d services (%d définitions), %d caractéristiques, %d statuts",
		version, len(serviceRows), len(serviceDefs), len(characteristicRows), len(statuses))
	return out, summary, nil
}

// readVersion renvoie « vX.Y.Z », d'après package.json. Un clone qui n'est pas
// HAP-NodeJS, ou pas sur une version stable, est refusé.
func readVersion(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return "", err
	}
	var pkg struct{ Name, Version string }
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("package.json illisible : %w", err)
	}
	if pkg.Name != "@homebridge/hap-nodejs" {
		return "", fmt.Errorf("package.json : paquet %q, HAP-NodeJS attendu", pkg.Name)
	}
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(pkg.Version) {
		return "", fmt.Errorf("package.json : version %q, une version stable X.Y.Z est attendue", pkg.Version)
	}
	return "v" + pkg.Version, nil
}

func readSource(root, rel string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// parseServices lit ServiceDefinitions.ts. Le nom du service est celui du
// commentaire JSDoc ; on vérifie que chaque classe appelle bien
// super(displayName, …), sans nom littéral.
func parseServices(src string) ([]definition, error) {
	defs, err := parseDefinitions(src, "Service")
	if err != nil {
		return nil, err
	}
	calls := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^    super\(displayName, (\w+)\.UUID, subtype\);`).FindAllStringSubmatch(src, -1) {
		calls[m[1]] = true
	}
	for _, d := range defs {
		if !calls[d.class] {
			return nil, fmt.Errorf("classe %s : pas d'appel super(displayName, %s.UUID, subtype), le nom du service est peut-être ailleurs", d.class, d.class)
		}
	}
	return defs, nil
}

// parseCharacteristics lit CharacteristicDefinitions.ts. Le nom est le premier
// argument de super ; on vérifie qu'il est identique à celui du commentaire
// JSDoc.
func parseCharacteristics(src string) ([]definition, error) {
	defs, err := parseDefinitions(src, "Characteristic")
	if err != nil {
		return nil, err
	}
	supers := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^    super\("([^"\n]+)", (\w+)\.UUID,`).FindAllStringSubmatch(src, -1) {
		supers[m[2]] = m[1]
	}
	for _, d := range defs {
		name, ok := supers[d.class]
		if !ok {
			return nil, fmt.Errorf("classe %s : appel super(\"…\", %s.UUID, …) introuvable", d.class, d.class)
		}
		if name != d.name {
			return nil, fmt.Errorf("classe %s : le commentaire JSDoc dit %q, l'appel super dit %q", d.class, d.name, name)
		}
	}
	return defs, nil
}

// parseDefinitions extrait le nom, la classe et l'UUID de chaque classe de
// kind (« Service » ou « Characteristic »). Toute classe qui n'a pas la forme
// attendue fait échouer le décompte.
func parseDefinitions(src, kind string) ([]definition, error) {
	classes := regexp.MustCompile(`(?m)^export class `).FindAllString(src, -1)
	block := regexp.MustCompile(`(?m)^ \* ` + kind + ` "([^"\n]+)"\n` +
		`(?: \* @[^\n]*\n)*` +
		` \*/\n` +
		`export class (\w+) extends ` + kind + ` \{\n\n` +
		`  public static readonly UUID: string = "([0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12})";\n`)
	matches := block.FindAllStringSubmatch(src, -1)
	if len(matches) != len(classes) {
		return nil, fmt.Errorf("%d classes, dont %d seulement ont la forme attendue : la mise en forme de la source a changé", len(classes), len(matches))
	}
	defs := make([]definition, len(matches))
	for i, m := range matches {
		defs[i] = definition{name: m[1], class: m[2], uuid: m[3]}
	}
	return defs, nil
}

// shortKey renvoie le type court (majuscules, sans zéros de tête) d'un UUID
// sur la base Apple, et refuse tout autre UUID.
func shortKey(uuid string) (string, error) {
	u := strings.ToUpper(uuid)
	if len(u) != 8+len(appleSuffix) || !strings.HasSuffix(u, appleSuffix) {
		return "", fmt.Errorf("UUID %s hors de la base Apple", uuid)
	}
	if key := strings.TrimLeft(u[:8], "0"); key != "" {
		return key, nil
	}
	return "0", nil
}

// buildRows regroupe les définitions par type court et les trie par valeur.
// Un type porté par plusieurs définitions doit être tranché dans aliases.
func buildRows(kind string, defs []definition, aliases map[string]alias) ([]row, error) {
	byKey := map[string][]definition{}
	for _, d := range defs {
		key, err := shortKey(d.uuid)
		if err != nil {
			return nil, fmt.Errorf("%s %s : %w", kind, d.class, err)
		}
		byKey[key] = append(byKey[key], d)
	}

	used := map[string]bool{}
	rows := make([]row, 0, len(byKey))
	for key, group := range byKey {
		if len(group) == 1 {
			rows = append(rows, row{key: key, name: group[0].name})
			continue
		}
		names := make([]string, len(group))
		for i, d := range group {
			names[i] = d.name
		}
		sort.Strings(names)
		a, ok := aliases[key]
		want := []string{a.keep, a.drop}
		sort.Strings(want)
		if !ok || strings.Join(names, "\n") != strings.Join(want, "\n") {
			return nil, fmt.Errorf("%s : l'UUID %s est porté par %q, sans arbitrage correspondant dans le générateur", kind, key, names)
		}
		used[key] = true
		rows = append(rows, row{key: key, name: a.keep, note: "alias écarté : " + a.drop})
	}
	for key := range aliases {
		if !used[key] {
			return nil, fmt.Errorf("%s : arbitrage de l'UUID %s sans doublon dans la source, à retirer du générateur", kind, key)
		}
	}

	value := func(r row) uint64 {
		n, _ := strconv.ParseUint(r.key, 16, 32) // clé issue de shortKey : toujours valide
		return n
	}
	sort.Slice(rows, func(i, j int) bool { return value(rows[i]) < value(rows[j]) })
	return rows, nil
}

// parseStatuses lit l'enum HAPStatus de HAPServer.ts. Toute ligne de l'enum
// qui n'est ni un commentaire ni « IDENTIFIANT = code, » fait échouer.
func parseStatuses(src string) ([]status, error) {
	m := regexp.MustCompile(`(?s)export const enum HAPStatus \{\n(.*?)\n\}\n`).FindStringSubmatch(src)
	if m == nil {
		return nil, errors.New("enum HAPStatus introuvable")
	}
	member := regexp.MustCompile(`^  ([A-Z][A-Z0-9_]*) = (-?[0-9]+),$`)

	var out []status
	seenCode, seenIdent := map[int]bool{}, map[string]bool{}
	for _, line := range strings.Split(m[1], "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "/**") || strings.HasPrefix(t, "*") || strings.HasPrefix(t, "//") {
			continue
		}
		mm := member.FindStringSubmatch(line)
		if mm == nil {
			return nil, fmt.Errorf("HAPStatus : ligne non reconnue %q", line)
		}
		code, err := strconv.Atoi(mm[2])
		if err != nil {
			return nil, fmt.Errorf("HAPStatus %s : %w", mm[1], err)
		}
		if seenCode[code] || seenIdent[mm[1]] {
			return nil, fmt.Errorf("HAPStatus : %s = %d en double", mm[1], code)
		}
		seenCode[code], seenIdent[mm[1]] = true, true
		out = append(out, status{code: code, ident: mm[1]})
	}
	if len(out) == 0 {
		return nil, errors.New("HAPStatus : aucun membre")
	}
	// Du code le plus proche de zéro au plus négatif, comme dans la source.
	sort.Slice(out, func(i, j int) bool { return out[i].code > out[j].code })
	return out, nil
}

// statusLines associe à chaque statut son libellé français suivi de
// l'identifiant, et vérifie qu'aucun libellé n'est orphelin.
func statusLines(statuses []status) ([]string, error) {
	lines := make([]string, len(statuses))
	seen := map[string]bool{}
	for i, s := range statuses {
		label, ok := statusLabels[s.ident]
		if !ok {
			return nil, fmt.Errorf("HAPStatus %s (%d) : pas de libellé français dans statusLabels", s.ident, s.code)
		}
		seen[s.ident] = true
		lines[i] = label + " (" + s.ident + ")"
	}
	for ident := range statusLabels {
		if !seen[ident] {
			return nil, fmt.Errorf("statusLabels : libellé de %s, absent de l'enum HAPStatus", ident)
		}
	}
	return lines, nil
}

func render(version string, services, characteristics []row, statuses []status, labels []string) []byte {
	var b bytes.Buffer
	w := func(f string, args ...any) { fmt.Fprintf(&b, f, args...) }

	w("// Code generated by internal/catalog/gen/main.go; DO NOT EDIT.\n")
	w("//\n")
	w("// Tables tirées de HAP-NodeJS %s, licence Apache-2.0\n", version)
	w("// (https://github.com/homebridge/HAP-NodeJS). Fichiers lus :\n")
	w("//   - %s\n", servicesFile)
	w("//   - %s\n", characteristicsFile)
	w("//   - %s\n", statusFile)
	w("//\n")
	w("// Noms de services : commentaire JSDoc de chaque classe. Noms de\n")
	w("// caractéristiques : premier argument de super. Statuts : enum HAPStatus. Les\n")
	w("// noms sont recopiés tels quels, définitions dépréciées comprises ; les\n")
	w("// libellés français des statuts traduisent les commentaires de l'enum.\n")
	w("//\n")
	w("// Les clés sont les types courts : majuscules, sans zéros de tête. Un UUID\n")
	w("// porté par deux définitions est tranché dans le générateur (serviceAliases) ;\n")
	w("// l'alias écarté est noté sur la ligne du nom conservé.\n")
	w("//\n")
	w("// Pour régénérer, depuis la racine du dépôt, avec un clone de HAP-NodeJS pris\n")
	w("// au même tag :\n")
	w("//\n")
	w("//\tgo run internal/catalog/gen/main.go <dossier du clone> > internal/catalog/tables.go\n")
	w("\n")
	w("package catalog\n\n")

	w("// Source nomme le tag de HAP-NodeJS et les fichiers dont les tables sont tirées.\n")
	w("const Source = %q\n\n", "HAP-NodeJS "+version+" (Apache-2.0) : "+
		strings.Join([]string{servicesFile, characteristicsFile, statusFile}, ", "))

	names := func(doc, name string, rows []row) {
		w("// %s (%d entrées).\n", doc, len(rows))
		w("var %s = map[string]string{\n", name)
		for _, r := range rows {
			w("\t%q: %q,", r.key, r.name)
			if r.note != "" {
				w(" // %s", r.note)
			}
			w("\n")
		}
		w("}\n\n")
	}
	names("services associe le type court d'un service Apple à son nom", "services", services)
	names("characteristics fait de même pour les caractéristiques", "characteristics", characteristics)

	w("// statuses associe un code de statut HAP à son libellé français, suivi de\n")
	w("// l'identifiant HAP-NodeJS (%d entrées).\n", len(statuses))
	w("var statuses = map[int]string{\n")
	for i, s := range statuses {
		w("\t%d: %q,\n", s.code, labels[i])
	}
	w("}\n")
	return b.Bytes()
}
