// Package catalog nomme les types HomeKit (HAP) : services, caractéristiques
// et codes de statut, pour afficher des inventaires lisibles.
package catalog

import "strings"

// Suffixe de l'UUID de base d'Apple, après le premier groupe de 8 chiffres.
const appleSuffix = "-0000-1000-8000-0026BB765291"

// Short ramène un type HAP à sa forme courte. Un UUID complet sur la base
// Apple (« 0000011A-0000-1000-8000-0026BB765291 », en toute casse) devient
// « 11A » ; une forme courte (« 3e », « 011a ») passe en majuscules, sans
// zéros de tête (« 3E », « 11A »). Tout autre type, comme l'UUID d'un
// fabricant, est renvoyé en majuscules, sans autre changement.
func Short(t string) string {
	u := strings.ToUpper(t)
	if n := len(u) - len(appleSuffix); n == 8 && strings.HasSuffix(u, appleSuffix) && isHex(u[:n]) {
		return trimZeros(u[:n])
	}
	if len(u) <= 8 && isHex(u) {
		return trimZeros(u)
	}
	return u
}

// IsApple indique si t est un type défini par Apple : une forme courte, ou
// un UUID complet sur la base Apple.
func IsApple(t string) bool {
	s := Short(t)
	return len(s) <= 8 && isHex(s)
}

// isHex indique si s est une suite non vide de chiffres hexadécimaux
// majuscules.
func isHex(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; (c < '0' || c > '9') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// trimZeros retire les zéros de tête, mais garde « 0 » pour une valeur nulle.
func trimZeros(s string) string {
	if s = strings.TrimLeft(s, "0"); s == "" {
		return "0"
	}
	return s
}

// Service renvoie le nom du service Apple t, donné sous toute forme admise
// par Short, ou "" s'il est inconnu : type de fabricant, ou absent de la
// source des tables.
func Service(t string) string { return services[Short(t)] }

// Characteristic renvoie le nom de la caractéristique Apple t, avec les mêmes
// conventions que Service.
func Characteristic(t string) string { return characteristics[Short(t)] }

// Status renvoie le libellé français d'un code de statut HAP, suivi entre
// parenthèses de l'identifiant HAP-NodeJS, ou "" si le code est inconnu.
func Status(code int) string { return statuses[code] }
