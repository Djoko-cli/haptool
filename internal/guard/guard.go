// Package guard décide si une écriture est permise et encode sa valeur.
//
// Politique (frontière de sécurité, docs/CONCEPTION.md) :
//   - liste blanche : seule la caractéristique du voyant (21D, Camera
//     Operating Mode Indicator) est inscriptible ; toute autre, comme 21B
//     (HomeKit Camera Active, qui couperait le flux de go2rtc) ou 11A
//     (Mute), demande une décision du propriétaire et un ajout ici ;
//   - jamais le service de streaming de go2rtc (110), ni 117, 118 ou 131,
//     où qu'ils soient ;
//   - jamais un type de fabricant, service ou caractéristique : impossible
//     à identifier, il peut s'agir d'une réinitialisation ;
//   - jamais une valeur opaque (tlv8, data) ni un format inconnu ;
//   - droit « pw » exigé ; refus de « tw » (écriture temporisée, qui exige
//     une préparation préalable, hors périmètre) et de « aa » (autorisation
//     additionnelle) ;
//   - valeur conforme au format, aux bornes, au pas, aux valeurs admises et
//     à la longueur maximale annoncés par la caméra.
package guard

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Djoko-cli/haptool/internal/catalog"
	"github.com/Djoko-cli/haptool/internal/hapc"
)

var forbiddenTypes = map[string]string{
	"110": "Camera RTP Stream Management, réservé à go2rtc",
	"117": "Selected RTP Stream Configuration, réservé à go2rtc",
	"118": "Setup Endpoints, réservé à go2rtc",
	"131": "Setup Data Stream Transport",
}

// writable est la liste blanche des types inscriptibles.
var writable = map[string]string{
	"21D": "Camera Operating Mode Indicator, le voyant d'état",
}

// intRange donne les bornes des formats entiers de HAP.
var intRange = map[string][2]float64{
	"uint8":  {0, math.MaxUint8},
	"uint16": {0, math.MaxUint16},
	"uint32": {0, math.MaxUint32},
	"uint64": {0, math.MaxUint64},
	"int":    {math.MinInt32, math.MaxInt32},
	"int32":  {math.MinInt32, math.MaxInt32},
}

// Value contrôle l'écriture de input dans la caractéristique c du service
// s, et renvoie la valeur JSON à envoyer.
func Value(s hapc.Service, c hapc.Characteristic, input string) (json.RawMessage, error) {
	st, ct := catalog.Short(s.Type), catalog.Short(c.Type)
	for _, t := range []string{st, ct} {
		if why, ok := forbiddenTypes[t]; ok {
			return nil, fmt.Errorf("écriture interdite : %s (%s)", t, why)
		}
	}
	if !catalog.IsApple(s.Type) || !catalog.IsApple(c.Type) {
		return nil, fmt.Errorf("écriture interdite : type de fabricant, non identifiable")
	}
	if _, ok := writable[ct]; !ok {
		return nil, fmt.Errorf("écriture interdite : %s n'est pas dans la liste blanche (voyant 21D seulement)", ct)
	}
	perms := make([]string, len(c.Perms))
	for i, p := range c.Perms {
		perms[i] = strings.ToLower(p)
	}
	switch {
	case !slices.Contains(perms, "pw"):
		return nil, fmt.Errorf("écriture interdite : la caméra n'annonce pas le droit pw")
	case slices.Contains(perms, "tw"):
		return nil, fmt.Errorf("écriture interdite : écriture temporisée (tw), hors périmètre")
	case slices.Contains(perms, "aa"):
		return nil, fmt.Errorf("écriture interdite : autorisation additionnelle (aa)")
	}
	return Encode(c, input)
}

// Encode contrôle input au regard du format, des bornes, du pas, des
// valeurs admises et de la longueur maximale de c, et renvoie la valeur
// JSON à envoyer. Il ne juge pas si l'écriture est permise : c'est Value.
func Encode(c hapc.Characteristic, input string) (json.RawMessage, error) {

	switch f := c.Format; {
	case f == "bool":
		switch input {
		case "1", "true":
			return json.RawMessage("true"), nil
		case "0", "false":
			return json.RawMessage("false"), nil
		}
		return nil, fmt.Errorf("valeur %s invalide pour un booléen : 0, 1, true ou false", strconv.QuoteToASCII(input))
	case f == "float":
		v, err := strconv.ParseFloat(input, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("valeur %s invalide pour un flottant", strconv.QuoteToASCII(input))
		}
		if err := bounds(c, v); err != nil {
			return nil, err
		}
		return json.RawMessage(strconv.FormatFloat(v, 'g', -1, 64)), nil
	case f == "string":
		max := 64 // longueur par défaut de la spécification HAP
		if len(c.MaxLen) > 0 {
			if err := json.Unmarshal(c.MaxLen, &max); err != nil {
				return nil, fmt.Errorf("maxLen illisible")
			}
		}
		if utf8.RuneCountInString(input) > max {
			return nil, fmt.Errorf("chaîne de %d caractères, %d au plus", utf8.RuneCountInString(input), max)
		}
		b, _ := json.Marshal(input)
		return b, nil
	}

	r, ok := intRange[c.Format]
	if !ok {
		return nil, fmt.Errorf("écriture interdite : format %s non pris en charge", strconv.QuoteToASCII(c.Format))
	}
	v, err := strconv.ParseInt(input, 10, 64)
	if c.Format == "uint64" {
		var u uint64
		u, err = strconv.ParseUint(input, 10, 64)
		v = int64(u)
	}
	if err != nil || float64(v) < r[0] || float64(v) > r[1] {
		return nil, fmt.Errorf("valeur %s invalide pour le format %s", strconv.QuoteToASCII(input), c.Format)
	}
	if err := bounds(c, float64(v)); err != nil {
		return nil, err
	}
	return json.RawMessage(strconv.FormatInt(v, 10)), nil
}

// bounds vérifie min, max, pas, valid-values et valid-values-range.
func bounds(c hapc.Characteristic, v float64) error {
	num := func(m json.RawMessage) (float64, bool) {
		var f float64
		return f, len(m) > 0 && json.Unmarshal(m, &f) == nil
	}
	min, hasMin := num(c.MinValue)
	if hasMin && v < min {
		return fmt.Errorf("valeur %g sous le minimum %g", v, min)
	}
	if max, ok := num(c.MaxValue); ok && v > max {
		return fmt.Errorf("valeur %g au-dessus du maximum %g", v, max)
	}
	if step, ok := num(c.MinStep); ok && step > 0 && c.Format != "float" {
		base := 0.0
		if hasMin {
			base = min
		}
		if q := (v - base) / step; q != math.Trunc(q) {
			return fmt.Errorf("valeur %g hors du pas %g", v, step)
		}
	}
	if len(c.ValidValues) > 0 {
		var valid []float64
		if json.Unmarshal(c.ValidValues, &valid) != nil || !slices.Contains(valid, v) {
			return fmt.Errorf("valeur %g hors des valeurs admises %s", v, c.ValidValues)
		}
	}
	if len(c.ValidRange) > 0 {
		var rg []float64
		if json.Unmarshal(c.ValidRange, &rg) != nil || len(rg) != 2 || v < rg[0] || v > rg[1] {
			return fmt.Errorf("valeur %g hors de la plage admise %s", v, c.ValidRange)
		}
	}
	return nil
}
