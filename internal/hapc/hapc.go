// Package hapc ouvre une session de contrôle HAP courte avec une caméra :
// résolution d'adresse, pair-verify, lectures chiffrées de /accessories et
// /characteristics, et écriture sur /characteristics. Aucune autre
// ressource n'est touchée (liste blanche : allowed).
package hapc

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// ID désigne une caractéristique : aid.iid.
type ID struct{ AID, IID uint64 }

func (id ID) String() string { return fmt.Sprintf("%d.%d", id.AID, id.IID) }

var idPattern = regexp.MustCompile(`^[1-9][0-9]*\.[1-9][0-9]*$`)

// ParseIDs lit une liste « aid.iid[,aid.iid…] » d'entiers non nuls.
func ParseIDs(s string) ([]ID, error) {
	var ids []ID
	for _, part := range strings.Split(s, ",") {
		if !idPattern.MatchString(part) {
			return nil, fmt.Errorf("identifiant %q invalide : attendu aid.iid, entiers à partir de 1", part)
		}
		a, i, _ := strings.Cut(part, ".")
		aid, errA := strconv.ParseUint(a, 10, 64)
		iid, errI := strconv.ParseUint(i, 10, 64)
		if errA != nil || errI != nil {
			return nil, fmt.Errorf("identifiant %q hors limites", part)
		}
		ids = append(ids, ID{AID: aid, IID: iid})
	}
	return ids, nil
}

// Accessory, Service et Characteristic suivent les objets JSON de HAP.
// Les champs de valeur restent bruts (json.RawMessage), pour ne rien
// arrondir ni convertir ; tout champ non prévu est gardé dans Extra.
type Accessory struct {
	AID      uint64    `json:"aid"`
	Services []Service `json:"services"`
}

type Service struct {
	IID             uint64                     `json:"iid"`
	Type            string                     `json:"type"`
	Primary         bool                       `json:"primary,omitempty"`
	Hidden          bool                       `json:"hidden,omitempty"`
	Linked          []uint64                   `json:"linked,omitempty"`
	Characteristics []Characteristic           `json:"characteristics"`
	Extra           map[string]json.RawMessage `json:"-"`
}

type Characteristic struct {
	AID         uint64                     `json:"aid,omitempty"`
	IID         uint64                     `json:"iid"`
	Type        string                     `json:"type,omitempty"`
	Format      string                     `json:"format,omitempty"`
	Perms       []string                   `json:"perms,omitempty"`
	Value       json.RawMessage            `json:"value,omitempty"`
	Status      *int                       `json:"status,omitempty"`
	Ev          *bool                      `json:"ev,omitempty"`
	Description string                     `json:"description,omitempty"`
	Unit        string                     `json:"unit,omitempty"`
	MinValue    json.RawMessage            `json:"minValue,omitempty"`
	MaxValue    json.RawMessage            `json:"maxValue,omitempty"`
	MinStep     json.RawMessage            `json:"minStep,omitempty"`
	MaxLen      json.RawMessage            `json:"maxLen,omitempty"`
	MaxDataLen  json.RawMessage            `json:"maxDataLen,omitempty"`
	ValidValues json.RawMessage            `json:"valid-values,omitempty"`
	ValidRange  json.RawMessage            `json:"valid-values-range,omitempty"`
	TTL         json.RawMessage            `json:"TTL,omitempty"`
	PID         json.RawMessage            `json:"pid,omitempty"`
	Extra       map[string]json.RawMessage `json:"-"`
}

func (s *Service) UnmarshalJSON(b []byte) error {
	type plain Service
	if err := json.Unmarshal(b, (*plain)(s)); err != nil {
		return err
	}
	var err error
	s.Extra, err = extra(b, plain{})
	return err
}

func (c *Characteristic) UnmarshalJSON(b []byte) error {
	type plain Characteristic
	if err := json.Unmarshal(b, (*plain)(c)); err != nil {
		return err
	}
	var err error
	c.Extra, err = extra(b, plain{})
	return err
}

// extra renvoie les champs de l'objet JSON b que la structure v ne
// déclare pas, ou nil.
func extra(b []byte, v any) (map[string]json.RawMessage, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(b, &all); err != nil {
		return nil, err
	}
	t := reflect.TypeOf(v)
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		delete(all, name)
	}
	if len(all) == 0 {
		return nil, nil
	}
	return all, nil
}

var errNoIDs = errors.New("aucune caractéristique demandée")
