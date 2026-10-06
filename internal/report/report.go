// Package report rend l'inventaire HomeKit d'une caméra : texte pour le
// terminal, JSON, et Markdown pour docs/inventaire.md avec le tableau des
// différences entre caméras. Les valeurs qui peuvent porter des clés de
// session sont masquées dans tous les rendus.
package report

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/hap/camera"
	"github.com/AlexxIT/go2rtc/pkg/hap/tlv8"

	"github.com/Djoko-cli/haptool/internal/catalog"
	"github.com/Djoko-cli/haptool/internal/hapc"
)

// Camera réunit ce que l'inventaire affiche d'une caméra.
type Camera struct {
	Name        string
	DeviceID    string
	Addr        string // adresse jointe
	Source      string // origine de l'adresse : imposée, mDNS, URL, balayage
	URLAddr     string // adresse de l'URL go2rtc
	Accessories []hapc.Accessory
}

// masked liste les types dont la valeur peut porter des clés de session :
// la réponse de Setup Endpoints (118) contient les clés SRTP maîtresses
// (camera.SetupEndpointsResponse de go2rtc) ; Setup Data Stream Transport
// (131) contient un sel de clé, masqué par prudence.
var masked = map[string]bool{"118": true, "131": true}

// decoders associe à un type la structure TLV8 que go2rtc en connaît
// (pkg/hap/camera).
var decoders = map[string]func() any{
	"114": func() any { return &camera.SupportedVideoStreamConfiguration{} },
	"115": func() any { return &camera.SupportedAudioStreamConfiguration{} },
	"116": func() any { return &camera.SupportedRTPConfiguration{} },
	"117": func() any { return &camera.SelectedStreamConfiguration{} },
	"120": func() any { return &camera.StreamingStatus{} },
	"130": func() any { return &camera.SupportedDataStreamTransportConfiguration{} },
	"205": func() any { return &camera.SupportedCameraRecordingConfiguration{} },
	"206": func() any { return &camera.SupportedVideoRecordingConfiguration{} },
	"207": func() any { return &camera.SupportedAudioRecordingConfiguration{} },
	"209": func() any { return &camera.SelectedCameraRecordingConfiguration{} },
}

// Value rend la valeur d'une caractéristique : statut HAP s'il n'est pas
// nul ; valeur JSON brute pour un scalaire ; TLV8 décodé quand go2rtc en
// connaît la structure ; sinon longueur et premiers octets.
func Value(c hapc.Characteristic) string {
	if c.Status != nil && *c.Status != 0 {
		if l := catalog.Status(*c.Status); l != "" {
			return fmt.Sprintf("statut %d : %s", *c.Status, l)
		}
		return fmt.Sprintf("statut %d (inconnu)", *c.Status)
	}
	if len(c.Value) == 0 || string(c.Value) == "null" {
		return "—"
	}
	typ := catalog.Short(c.Type)
	if typ == "" && (c.Format == "tlv8" || c.Format == "data" || c.Format == "") && strings.HasPrefix(string(c.Value), `"`) {
		// Sans type (caméra qui ignore type=1), rien ne dit que ce n'est
		// pas Setup Endpoints : masqué par prudence.
		return "<masqué : type inconnu>"
	}
	if !masked[typ] && c.Format != "tlv8" && c.Format != "data" {
		return string(c.Value)
	}

	var s string
	if err := json.Unmarshal(c.Value, &s); err != nil {
		if masked[typ] {
			return "<masqué>"
		}
		return string(c.Value)
	}
	if s == "" {
		return "(vide)"
	}
	b, err := base64.StdEncoding.DecodeString(s)
	switch {
	case masked[typ] && err != nil:
		return "<masqué>"
	case masked[typ]:
		return fmt.Sprintf("<masqué : clés de session possibles, %d o>", len(b))
	case err != nil:
		return fmt.Sprintf("base64 illisible (%d caractères)", len(s))
	}
	if newV, ok := decoders[typ]; ok && c.Format == "tlv8" {
		if s, ok := decode(b, newV()); ok {
			return s
		}
	}
	return fmt.Sprintf("%d o : %s", len(b), head(b))
}

// decode applique le décodeur TLV8 de go2rtc, qui panique sur certaines
// valeurs tronquées : une valeur mal formée ne doit pas coûter l'inventaire.
func decode(b []byte, v any) (s string, ok bool) {
	defer func() {
		if recover() != nil {
			s, ok = "", false
		}
	}()
	if err := tlv8.Unmarshal(b, v); err != nil {
		return "", false
	}
	return compact(reflect.ValueOf(v)), true
}

// head rend en hexadécimal les 16 premiers octets de b.
func head(b []byte) string {
	if len(b) <= 16 {
		return hex.EncodeToString(b)
	}
	return hex.EncodeToString(b[:16]) + "…"
}

// compact rend une structure TLV8 décodée sur une ligne, sans ses listes
// ni chaînes vides : {Champ:valeur …}.
func compact(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return "nil"
		}
		return compact(v.Elem())
	case reflect.Struct:
		var parts []string
		t := v.Type()
		for i := range v.NumField() {
			f := v.Field(i)
			if !t.Field(i).IsExported() {
				continue
			}
			if (f.Kind() == reflect.Slice || f.Kind() == reflect.String) && f.Len() == 0 {
				continue
			}
			parts = append(parts, t.Field(i).Name+":"+compact(f))
		}
		return "{" + strings.Join(parts, " ") + "}"
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
			return fmt.Sprint(v.Bytes()) // listes d'énumérations : [0 1]
		}
		items := make([]string, v.Len())
		for i := range items {
			items[i] = compact(v.Index(i))
		}
		return "[" + strings.Join(items, " ") + "]"
	case reflect.String:
		return printable(v.String())
	default:
		return fmt.Sprint(v.Interface())
	}
}

// printable cite s s'il est en ASCII imprimable, le rend en hexadécimal
// sinon (identifiants de session binaires).
func printable(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return hex.EncodeToString([]byte(s))
		}
	}
	return strconv.Quote(s)
}

// ServiceLabel et CharLabel nomment un type d'après le catalogue :
// « Nom (TYPE) », ou « inconnu (TYPE) ».
func ServiceLabel(t string) string { return label(catalog.Service(t), t) }

func CharLabel(t string) string { return label(catalog.Characteristic(t), t) }

// charName nomme une caractéristique pour la colonne « nom » : nom du
// catalogue, ou « inconnu », précisé « (fabricant) » hors de la base Apple.
func charName(t string) string {
	switch {
	case catalog.Characteristic(t) != "":
		return catalog.Characteristic(t)
	case catalog.IsApple(t):
		return "inconnu"
	default:
		return "inconnu (fabricant)"
	}
}

func label(name, t string) string {
	short := catalog.Short(t)
	switch {
	case name != "":
		return name + " (" + short + ")"
	case catalog.IsApple(t):
		return "inconnu (" + short + ")"
	default:
		return "inconnu, fabricant (" + short + ")"
	}
}

// bounds rend les métadonnées d'une caractéristique, dans l'ordre :
// unité, min, max, pas, valid-values, valid-values-range, maxLen.
func bounds(c hapc.Characteristic) []string {
	return []string{c.Unit, raw(c.MinValue), raw(c.MaxValue), raw(c.MinStep),
		raw(c.ValidValues), raw(c.ValidRange), raw(c.MaxLen)}
}

func raw(m json.RawMessage) string { return string(m) }

func others(c hapc.Characteristic) string {
	var parts []string
	if c.Description != "" {
		parts = append(parts, "description="+strconv.Quote(c.Description))
	}
	if len(c.MaxDataLen) > 0 {
		parts = append(parts, "maxDataLen="+raw(c.MaxDataLen))
	}
	if len(c.TTL) > 0 {
		parts = append(parts, "TTL="+raw(c.TTL))
	}
	if len(c.PID) > 0 {
		parts = append(parts, "pid="+raw(c.PID))
	}
	for k, v := range c.Extra {
		parts = append(parts, k+"="+string(v))
	}
	return strings.Join(parts, " ")
}

// Text écrit l'inventaire d'une caméra pour le terminal.
func Text(w io.Writer, cam Camera) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "caméra %s · device_id %s · adresse %s (%s) · URL go2rtc %s\n",
		cam.Name, cam.DeviceID, cam.Addr, cam.Source, cam.URLAddr)
	for _, a := range cam.Accessories {
		for _, s := range a.Services {
			fmt.Fprintf(tw, "\n[%d.%d] %s%s\n", a.AID, s.IID, ServiceLabel(s.Type), serviceFlags(s))
			fmt.Fprintln(tw, "  iid\ttype\tnom\tvaleur\tdroits\tformat\tunité\tmin\tmax\tpas\tvalides\tplage\tmaxLen\tautres")
			for _, c := range s.Characteristics {
				fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", c.IID, catalog.Short(c.Type), charName(c.Type),
					Value(c), strings.Join(c.Perms, ","), c.Format, strings.Join(bounds(c), "\t"), others(c))
			}
		}
	}
	return tw.Flush()
}

// Characteristics écrit le résultat d'une lecture ciblée (get).
func Characteristics(w io.Writer, cam Camera, chars []hapc.Characteristic) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "caméra %s · adresse %s (%s)\n", cam.Name, cam.Addr, cam.Source)
	fmt.Fprintln(tw, "id\ttype\tnom\tvaleur\tdroits\tformat\tunité\tmin\tmax\tpas\tvalides\tplage\tmaxLen\tautres")
	for _, c := range chars {
		fmt.Fprintf(tw, "%d.%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", c.AID, c.IID, catalog.Short(c.Type), charName(c.Type),
			Value(c), strings.Join(c.Perms, ","), c.Format, strings.Join(bounds(c), "\t"), others(c))
	}
	return tw.Flush()
}

func serviceFlags(s hapc.Service) string {
	var f []string
	if s.Primary {
		f = append(f, "principal")
	}
	if s.Hidden {
		f = append(f, "masqué")
	}
	if len(s.Linked) > 0 {
		f = append(f, "liés : "+fmt.Sprint(s.Linked))
	}
	for k, v := range s.Extra {
		f = append(f, k+"="+string(v))
	}
	if len(f) == 0 {
		return ""
	}
	return " · " + strings.Join(f, " · ")
}

// jsonChar ajoute le nom et le rendu à une caractéristique, valeur masquée
// le cas échéant.
type jsonChar struct {
	hapc.Characteristic
	Name    string                     `json:"name"`
	Display string                     `json:"display"`
	Extra   map[string]json.RawMessage `json:"extra,omitempty"`
}

type jsonService struct {
	IID             uint64                     `json:"iid"`
	Type            string                     `json:"type"`
	Name            string                     `json:"name"`
	Primary         bool                       `json:"primary,omitempty"`
	Hidden          bool                       `json:"hidden,omitempty"`
	Linked          []uint64                   `json:"linked,omitempty"`
	Extra           map[string]json.RawMessage `json:"extra,omitempty"`
	Characteristics []jsonChar                 `json:"characteristics"`
}

type jsonAccessory struct {
	AID      uint64        `json:"aid"`
	Services []jsonService `json:"services"`
}

type jsonCamera struct {
	Name        string          `json:"name"`
	DeviceID    string          `json:"device_id"`
	Addr        string          `json:"addr"`
	Source      string          `json:"source"`
	URLAddr     string          `json:"url_addr"`
	Accessories []jsonAccessory `json:"accessories"`
}

// JSON écrit l'inventaire des caméras en JSON indenté.
func JSON(w io.Writer, cams []Camera) error {
	out := make([]jsonCamera, 0, len(cams))
	for _, cam := range cams {
		jc := jsonCamera{Name: cam.Name, DeviceID: cam.DeviceID, Addr: cam.Addr, Source: cam.Source, URLAddr: cam.URLAddr}
		for _, a := range cam.Accessories {
			ja := jsonAccessory{AID: a.AID}
			for _, s := range a.Services {
				js := jsonService{IID: s.IID, Type: s.Type, Name: catalog.Service(s.Type), Primary: s.Primary,
					Hidden: s.Hidden, Linked: s.Linked, Extra: s.Extra}
				for _, c := range s.Characteristics {
					display := Value(c)
					if strings.HasPrefix(display, "<masqué") && len(c.Value) > 0 {
						c.Value = json.RawMessage(strconv.Quote(display))
					}
					js.Characteristics = append(js.Characteristics, jsonChar{Characteristic: c,
						Name: catalog.Characteristic(c.Type), Display: display, Extra: c.Extra})
				}
				ja.Services = append(ja.Services, js)
			}
			jc.Accessories = append(jc.Accessories, ja)
		}
		out = append(out, jc)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(map[string]any{"source_des_noms": catalog.Source, "cameras": out})
}

// Difference est une ligne du tableau des différences entre deux caméras.
type Difference struct{ Service, Characteristic, Field, A, B string }

type entry struct {
	service, char string
	c             hapc.Characteristic
}

// index range les caractéristiques d'une caméra par clé « type de
// service#rang / type de caractéristique#rang », dans l'ordre de lecture.
func index(cam Camera) ([]string, map[string]entry) {
	var keys []string
	m := map[string]entry{}
	for _, a := range cam.Accessories {
		seenS := map[string]int{}
		for _, s := range a.Services {
			st := catalog.Short(s.Type)
			sk := fmt.Sprintf("%d/%s#%d", a.AID, st, seenS[st])
			seenS[st]++
			seenC := map[string]int{}
			for _, c := range s.Characteristics {
				ct := catalog.Short(c.Type)
				k := fmt.Sprintf("%s/%s#%d", sk, ct, seenC[ct])
				seenC[ct]++
				keys = append(keys, k)
				m[k] = entry{service: ServiceLabel(s.Type), char: CharLabel(c.Type), c: c}
			}
		}
	}
	return keys, m
}

// Diff compare deux caméras. Services et caractéristiques sont appariés
// par type et par rang parmi ceux de même type. Champs comparés :
// présence, valeur, droits, format, unité, min, max, pas, valid-values,
// valid-values-range, maxLen.
func Diff(a, b Camera) []Difference {
	keysA, ma := index(a)
	keysB, mb := index(b)
	keys := keysA
	for _, k := range keysB {
		if _, ok := ma[k]; !ok {
			keys = append(keys, k)
		}
	}
	names := []string{"valeur", "droits", "format", "unité", "min", "max", "pas", "valid-values", "valid-values-range", "maxLen"}
	fields := func(c hapc.Characteristic) []string {
		return append([]string{Value(c), strings.Join(c.Perms, ","), c.Format}, bounds(c)...)
	}
	var diffs []Difference
	for _, k := range keys {
		ea, okA := ma[k]
		eb, okB := mb[k]
		e := ea
		if !okA {
			e = eb
		}
		if okA != okB {
			diffs = append(diffs, Difference{e.service, e.char, "présence", yesNo(okA), yesNo(okB)})
			continue
		}
		fa, fb := fields(ea.c), fields(eb.c)
		for i, name := range names {
			if fa[i] != fb[i] {
				diffs = append(diffs, Difference{e.service, e.char, name, fa[i], fb[i]})
			}
		}
	}
	return diffs
}

func yesNo(b bool) string {
	if b {
		return "oui"
	}
	return "non"
}

// cell protège une valeur pour une cellule de tableau Markdown : barre,
// accent grave (code), « < » (HTML) et sauts de ligne.
var cellEscaper = strings.NewReplacer("|", `\|`, "`", "\\`", "<", "&lt;", "\n", " ", "\r", " ")

func cell(s string) string { return cellEscaper.Replace(s) }

// Markdown écrit l'inventaire : en-tête, différences entre les deux
// premières caméras, puis une section par caméra.
func Markdown(w io.Writer, cams []Camera, generated time.Time) error {
	var b strings.Builder
	names := make([]string, len(cams))
	for i, c := range cams {
		names[i] = c.Name
	}
	b.WriteString("# Inventaire HomeKit des Circle View\n\n")
	fmt.Fprintf(&b, "Généré par `haptool dump --md %s` le %s.\n\n", strings.Join(names, " "), generated.Format("2006-01-02 à 15:04 MST"))
	fmt.Fprintf(&b, "- Noms des types : %s.\n", catalog.Source)
	b.WriteString("- Valeurs masquées : `118` (Setup Endpoints) et `131` (Setup Data Stream Transport), qui peuvent porter des clés de session.\n")
	b.WriteString("- Valeurs TLV8 décodées avec les structures de go2rtc v1.9.14 (`pkg/hap/camera`) quand elles existent.\n\n")

	if len(cams) >= 2 {
		fmt.Fprintf(&b, "## Différences entre %s et %s\n\n", cams[0].Name, cams[1].Name)
		if diffs := Diff(cams[0], cams[1]); len(diffs) == 0 {
			b.WriteString("Aucune différence.\n\n")
		} else {
			fmt.Fprintf(&b, "| Service | Caractéristique | Champ | %s | %s |\n|---|---|---|---|---|\n", cams[0].Name, cams[1].Name)
			for _, d := range diffs {
				fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", cell(d.Service), cell(d.Characteristic), d.Field, cell(d.A), cell(d.B))
			}
			b.WriteString("\n")
		}
	}

	for _, cam := range cams {
		fmt.Fprintf(&b, "## %s\n\n", cam.Name)
		fmt.Fprintf(&b, "device_id `%s` · firmware `%s` · adresse jointe `%s` (%s) · URL go2rtc `%s`\n\n",
			cam.DeviceID, cell(firmware(cam)), cam.Addr, cam.Source, cam.URLAddr)
		for _, a := range cam.Accessories {
			for _, s := range a.Services {
				fmt.Fprintf(&b, "### [%d.%d] %s%s\n\n", a.AID, s.IID, cell(ServiceLabel(s.Type)), cell(serviceFlags(s)))
				b.WriteString("| iid | type | nom | valeur | droits | format | unité | min | max | pas | valides | plage | maxLen | autres |\n")
				b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
				for _, c := range s.Characteristics {
					row := append([]string{strconv.FormatUint(c.IID, 10), catalog.Short(c.Type), charName(c.Type),
						Value(c), strings.Join(c.Perms, ","), c.Format}, bounds(c)...)
					row = append(row, others(c))
					for i := range row {
						row[i] = cell(row[i])
					}
					fmt.Fprintf(&b, "| %s |\n", strings.Join(row, " | "))
				}
				b.WriteString("\n")
			}
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// firmware renvoie la valeur de Firmware Revision (52) du service
// Accessory Information, ou « ? ».
func firmware(cam Camera) string {
	for _, a := range cam.Accessories {
		for _, s := range a.Services {
			if catalog.Short(s.Type) != "3E" {
				continue
			}
			for _, c := range s.Characteristics {
				if catalog.Short(c.Type) == "52" {
					return Value(c)
				}
			}
		}
	}
	return "?"
}
