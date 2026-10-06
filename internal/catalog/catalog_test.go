package catalog_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/Djoko-cli/haptool/internal/catalog"
)

// Défaut attrapé : un type HAP sortirait sous plusieurs formes (UUID
// complet, forme courte, casse variable) et les recherches par nom
// échoueraient. Un UUID de fabricant ne doit pas être déformé.
func TestShort(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"UUID Apple, majuscules", "0000011A-0000-1000-8000-0026BB765291", "11A"},
		{"UUID Apple, minuscules", "0000011a-0000-1000-8000-0026bb765291", "11A"},
		{"UUID Apple, casse mêlée", "0000011a-0000-1000-8000-0026BB765291", "11A"},
		{"UUID Apple, deux chiffres", "0000003E-0000-1000-8000-0026BB765291", "3E"},
		{"UUID Apple, sans lettre", "00000052-0000-1000-8000-0026BB765291", "52"},
		{"UUID Apple, premier groupe plein", "12345678-0000-1000-8000-0026BB765291", "12345678"},
		{"forme courte, minuscules", "3e", "3E"},
		{"forme courte, zéros de tête", "011a", "11A"},
		{"forme courte déjà normalisée", "11A", "11A"},
		{"forme courte sur 8 chiffres", "0000011a", "11A"},
		{"forme courte, que des zéros", "000", "0"},
		{"UUID de fabricant, majuscules", "1F5B9E2A-4C3D-4E8F-9A1B-2C3D4E5F6A7B", "1F5B9E2A-4C3D-4E8F-9A1B-2C3D4E5F6A7B"},
		{"UUID de fabricant, minuscules", "1f5b9e2a-4c3d-4e8f-9a1b-2c3d4e5f6a7b", "1F5B9E2A-4C3D-4E8F-9A1B-2C3D4E5F6A7B"},
		{"texte qui n'est pas un type", "hello", "HELLO"},
		{"vide", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := catalog.Short(tt.in); got != tt.want {
				t.Errorf("\ngot  %q\nwant %q", got, tt.want)
			}
		})
	}
}

// Défaut attrapé : un type de fabricant serait présenté comme un type Apple
// « inconnu » au lieu d'être signalé comme propre au fabricant.
func TestIsApple(t *testing.T) {
	tests := []struct {
		name, in string
		want     bool
	}{
		{"UUID Apple complet", "0000011A-0000-1000-8000-0026BB765291", true},
		{"UUID Apple complet, minuscules", "0000011a-0000-1000-8000-0026bb765291", true},
		{"forme courte", "11A", true},
		{"forme courte, minuscules et zéro de tête", "03e", true},
		{"UUID de fabricant", "1F5B9E2A-4C3D-4E8F-9A1B-2C3D4E5F6A7B", false},
		{"UUID de fabricant, minuscules", "1f5b9e2a-4c3d-4e8f-9a1b-2c3d4e5f6a7b", false},
		{"base Apple altérée", "0000011A-0000-1000-8000-0026BB765292", false},
		{"texte qui n'est pas un type", "hello", false},
		{"vide", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := catalog.IsApple(tt.in); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// Défaut attrapé : un service de caméra sortirait sans nom, ou sous le nom
// d'un autre type. Attentes relevées à la main dans HAP-NodeJS v2.2.3,
// src/lib/definitions/ServiceDefinitions.ts : commentaire JSDoc
// « Service "…" » de chaque classe, numéro de ligne en commentaire.
func TestService(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"3E, forme courte", "3E", "Accessory Information"},                                                // l. 47
		{"3E, UUID complet", "0000003E-0000-1000-8000-0026BB765291", "Accessory Information"},              // l. 47
		{"3E, minuscules", "3e", "Accessory Information"},                                                  // l. 47
		{"110, forme courte", "110", "Camera RTP Stream Management"},                                       // l. 298
		{"110, UUID complet", "00000110-0000-1000-8000-0026BB765291", "Camera RTP Stream Management"},      // l. 298
		{"112", "112", "Microphone"},                                                                       // l. 879
		{"113 : Speaker, pas Television Speaker", "113", "Speaker"},                                        // l. 1193 ; alias l. 1362
		{"113, UUID complet", "00000113-0000-1000-8000-0026BB765291", "Speaker"},                           // l. 1193
		{"204", "204", "Camera Recording Management"},                                                      // l. 275
		{"21A, forme courte", "21A", "Camera Operating Mode"},                                              // l. 250
		{"21A, UUID complet, minuscules", "0000021a-0000-1000-8000-0026bb765291", "Camera Operating Mode"}, // l. 250
		{"5A, dépréciée dans la source", "5A", "Cloud Relay"},                                              // l. 372
		{"56, dépréciée dans la source", "56", "Tunnel"},                                                   // l. 1484

		{"type absent de la source", "FFFF", ""},
		{"caractéristique, pas service", "52", ""},
		{"UUID de fabricant", "1F5B9E2A-4C3D-4E8F-9A1B-2C3D4E5F6A7B", ""},
		{"vide", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := catalog.Service(tt.in); got != tt.want {
				t.Errorf("\ngot  %q\nwant %q", got, tt.want)
			}
		})
	}
}

// Défaut attrapé : une caractéristique, en particulier celles du voyant, du
// micro et des flux, sortirait sans nom ou sous un autre. Attentes relevées
// à la main dans HAP-NodeJS v2.2.3,
// src/lib/definitions/CharacteristicDefinitions.ts : commentaire JSDoc
// « Characteristic "…" » de chaque classe (identique à l'appel super), numéro
// de ligne en commentaire.
func TestCharacteristic(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"52, forme courte", "52", "Firmware Revision"},                                                  // l. 1372
		{"52, UUID complet", "00000052-0000-1000-8000-0026BB765291", "Firmware Revision"},                // l. 1372
		{"11A, forme courte", "11A", "Mute"},                                                             // l. 2196
		{"11A, UUID complet", "0000011A-0000-1000-8000-0026BB765291", "Mute"},                            // l. 2196
		{"11A, minuscules", "11a", "Mute"},                                                               // l. 2196
		{"119", "119", "Volume"},                                                                         // l. 4887
		{"11B", "11B", "Night Vision"},                                                                   // l. 2318
		{"21B", "21B", "HomeKit Camera Active"},                                                          // l. 1531
		{"21D, forme courte", "21D", "Camera Operating Mode Indicator"},                                  // l. 378
		{"21D, UUID complet", "0000021D-0000-1000-8000-0026BB765291", "Camera Operating Mode Indicator"}, // l. 378
		{"223", "223", "Event Snapshots Active"},                                                         // l. 1287
		{"225", "225", "Periodic Snapshots Active"},                                                      // l. 2554
		{"226", "226", "Recording Audio Active"},                                                         // l. 2852
		{"227, forme courte", "227", "Manually Disabled"},                                                // l. 2014
		{"227, UUID complet", "00000227-0000-1000-8000-0026BB765291", "Manually Disabled"},               // l. 2014
		{"120", "120", "Streaming Status"},                                                               // l. 3843
		{"117", "117", "Selected RTP Stream Configuration"},                                              // l. 3268
		{"118", "118", "Setup Endpoints"},                                                                // l. 3419
		{"5E, dépréciée dans la source", "5E", "Relay Control Point"},                                    // l. 2915

		{"type absent de la source", "FFFF", ""},
		{"service, pas caractéristique", "3E", ""},
		{"service, pas caractéristique (21A)", "0000021A-0000-1000-8000-0026BB765291", ""},
		{"UUID de fabricant", "1F5B9E2A-4C3D-4E8F-9A1B-2C3D4E5F6A7B", ""},
		{"vide", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := catalog.Characteristic(tt.in); got != tt.want {
				t.Errorf("\ngot  %q\nwant %q", got, tt.want)
			}
		})
	}
}

// Défaut attrapé : un code de statut rejeté par une caméra (-70410 dans un
// 207 Multi-Status) passerait inaperçu, ou serait associé à l'identifiant
// d'un autre code. Codes et identifiants relevés à la main dans HAP-NodeJS
// v2.2.3, src/lib/HAPServer.ts, enum HAPStatus (numéro de ligne en
// commentaire) ; les libellés français traduisent les commentaires de l'enum.
func TestStatus(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{0, "succès (SUCCESS)"}, // l. 57
		{-70401, "privilèges insuffisants (INSUFFICIENT_PRIVILEGES)"},                                 // l. 61
		{-70402, "échec de communication avec la caractéristique (SERVICE_COMMUNICATION_FAILURE)"},    // l. 65
		{-70403, "ressource occupée, réessayer (RESOURCE_BUSY)"},                                      // l. 69
		{-70404, "écriture impossible, caractéristique en lecture seule (READ_ONLY_CHARACTERISTIC)"},  // l. 73
		{-70405, "lecture impossible, caractéristique en écriture seule (WRITE_ONLY_CHARACTERISTIC)"}, // l. 77
		{-70406, "notifications non prises en charge (NOTIFICATION_NOT_SUPPORTED)"},                   // l. 81
		{-70407, "ressources insuffisantes pour traiter la requête (OUT_OF_RESOURCE)"},                // l. 85
		{-70408, "délai de l'opération dépassé (OPERATION_TIMED_OUT)"},                                // l. 89
		{-70409, "ressource inexistante (RESOURCE_DOES_NOT_EXIST)"},                                   // l. 93
		{-70410, "valeur invalide dans la requête (INVALID_VALUE_IN_REQUEST)"},                        // l. 97
		{-70411, "autorisation insuffisante (INSUFFICIENT_AUTHORIZATION)"},                            // l. 101
		{-70412, "opération non autorisée dans l'état actuel (NOT_ALLOWED_IN_CURRENT_STATE)"},         // l. 105

		{-70413, ""}, // juste après le dernier code connu
		{-70400, ""}, // juste avant le premier
		{70410, ""},  // signe inversé
		{-1, ""},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.code), func(t *testing.T) {
			if got := catalog.Status(tt.code); got != tt.want {
				t.Errorf("\ngot  %q\nwant %q", got, tt.want)
			}
		})
	}
}

// Défaut attrapé : une Source sans le nom de la bibliothèque, sa licence ou
// ses fichiers ferait perdre l'attribution Apache-2.0 des tables.
func TestSourceNamesLibraryLicenseAndFiles(t *testing.T) {
	for _, want := range []string{
		"HAP-NodeJS v",
		"(Apache-2.0)",
		"src/lib/definitions/ServiceDefinitions.ts",
		"src/lib/definitions/CharacteristicDefinitions.ts",
		"src/lib/HAPServer.ts",
	} {
		if !strings.Contains(catalog.Source, want) {
			t.Errorf("Source %q ne contient pas %q", catalog.Source, want)
		}
	}
}
