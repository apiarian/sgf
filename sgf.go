/*
The Smart Game Format (SGF) encodes board game records. This package encodes
board game records in go.
*/

package sgf

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type GameTree struct {
	Sequence []*Node
	Subtrees []*GameTree
}

func NewGameTree() *GameTree {
	return &GameTree{}
}

func (gt *GameTree) String() (s string) {
	s = "("
	for _, n := range gt.Sequence {
		s = s + n.String()
	}
	for _, st := range gt.Subtrees {
		s = s + st.String()
	}
	s = s + ")"
	return
}

func (gt *GameTree) AddNode(n *Node) {
	gt.Sequence = append(gt.Sequence, n)
}

func (gt *GameTree) AddSubtree(st *GameTree) {
	gt.Subtrees = append(gt.Subtrees, st)
}

func (gt *GameTree) FindPropertyByIdentity(identity string) *Property {
	for _, n := range gt.Sequence {
		p := n.FindPropertyByIdentity(identity)
		if p != nil {
			return p
		}
	}
	for _, st := range gt.Subtrees {
		p := st.FindPropertyByIdentity(identity)
		if p != nil {
			return p
		}
	}
	return nil
}

func (gt *GameTree) SimpletextForPropertyIdentity(identity string) (string, error) {
	p := gt.FindPropertyByIdentity(identity)
	if p == nil {
		return "", fmt.Errorf("could not find a Property for identity %s", identity)
	}

	value, err := p.Simpletext()
	if err != nil {
		return "", fmt.Errorf("could not read simpletext from Property: %s", err)
	}

	return value, nil
}

func (gt *GameTree) BlackPlayerName() (string, error) {
	name, err := gt.SimpletextForPropertyIdentity("PB")
	if err != nil {
		return "", fmt.Errorf("could not find black player name: %s", err)
	}

	return name, nil
}

func (gt *GameTree) WhitePlayerName() (string, error) {
	name, err := gt.SimpletextForPropertyIdentity("PW")
	if err != nil {
		return "", fmt.Errorf("could not find black player name: %s", err)
	}

	return name, nil
}

func (gt *GameTree) WinnerColor() (string, error) {
	result, err := gt.SimpletextForPropertyIdentity("RE")
	if err != nil {
		return "", fmt.Errorf("could not find result: %s", err)
	}
	switch {
	case strings.HasPrefix(result, "B+"):
		return "B", nil
	case strings.HasPrefix(result, "W+"):
		return "W", nil
	default:
		return "?", fmt.Errorf("no clear winner: %s", result)
	}
}

func (gt *GameTree) WinnerName() (string, error) {
	winnerColor, err := gt.WinnerColor()
	if err != nil && winnerColor != "?" {
		return "", fmt.Errorf("error getting winner color: %s", err)
	}
	var name string
	switch winnerColor {
	case "B":
		name, err = gt.BlackPlayerName()
	case "W":
		name, err = gt.WhitePlayerName()
	case "?":
	default:
		return "", fmt.Errorf("unknown winner color: %s", winnerColor)
	}
	if name == "" {
		return "", fmt.Errorf("error getting winner name: %s", err)
	}
	return name, nil
}

type Node struct {
	Properties []*Property
}

func NewNode() *Node {
	return &Node{}
}

func (n *Node) String() (s string) {
	s = ";"
	for _, p := range n.Properties {
		s = s + p.String()
	}
	return
}

func (n *Node) AddProperty(p *Property) error {
	for _, existingProperty := range n.Properties {
		if p.Identity == existingProperty.Identity {
			return fmt.Errorf("sgf: only one of each property is allowed per node")
		}
	}
	n.Properties = append(n.Properties, p)
	return nil
}

func (n *Node) FindPropertyByIdentity(identity string) *Property {
	for _, p := range n.Properties {
		if p.Identity == identity {
			return p
		}
	}
	return nil
}

type Property struct {
	Type          PropertyType
	Identity      string
	Values        []*PropertyValue
	Description   string
	Validated     ValidationStatus
	validatorName string
	listiness     propertyListiness
}

type ValidationStatus int

const (
	NotValidated ValidationStatus = iota
	ValidationDeferred
	Valid
	Invalid
)

func NewProperty(identity string, values []*PropertyValue) (*Property, error) {
	p := &Property{
		Identity:  identity,
		Values:    values,
		Validated: NotValidated,
	}
	if typeInfo, ok := propertyTypeMap[identity]; ok {
		p.Type = typeInfo.t
		p.Description = typeInfo.d
		p.validatorName = typeInfo.vn
		p.listiness = typeInfo.l
		for _, v := range p.Values {
			v.validatorName = p.validatorName
		}
	} else {
		p.Type = PropertyTypeUnknown
		p.Description = "unknown"
		p.validatorName = "unknown"
		p.listiness = propertyUnknownList
	}
	err := p.validatePropertyValues()
	return p, err
}

func (p *Property) String() (s string) {
	s = p.Identity
	for _, v := range p.Values {
		s = s + v.String()
	}
	return
}

func (p *Property) AddValue(pv *PropertyValue) error {
	if pv.validatorName == "unknown" {
		pv.validatorName = p.validatorName
	}
	p.Values = append(p.Values, pv)
	return p.validatePropertyValues()
}

func (p *Property) Simpletext() (string, error) {
	if len(p.Values) > 0 {
		if p.listiness == propertyNotList {
			return p.Values[0].Simpletext()
		} else {
			return "", fmt.Errorf("Simpletext is only supported for non-list properties")
		}
	} else {
		return "", fmt.Errorf("no PropertyValues")
	}
}

func (p *Property) validatePropertyValues() error {
	p.Validated = NotValidated
	if p.listiness == propertyNotList {
		if len(p.Values) > 1 {
			p.Validated = Invalid
			return fmt.Errorf("Property %s can only have one value", p.Identity)
		}
	}
	errors := make([]error, 0)
	for _, pv := range p.Values {
		err := validatePropertyValue(pv)
		if err != nil {
			if strings.HasPrefix(err.Error(), "validation deferred:") {
				p.Validated = ValidationDeferred
				errors = append(errors, nil)
			} else {
				errors = append(errors, err)
			}
		} else {
			errors = append(errors, nil)
		}
	}
	var first_error error
	for _, err := range errors {
		if err != nil && first_error == nil {
			first_error = err
		}
	}
	if first_error != nil {
		p.Validated = Invalid
		return first_error
	} else {
		if p.Validated == NotValidated {
			p.Validated = Valid
		}
		return nil
	}
}

type PropertyType int

const (
	PropertyTypeRoot PropertyType = iota
	PropertyTypeGameInfo
	PropertyTypeSetup
	PropertyTypeMove
	PropertyTypeNull
	PropertyTypeUnknown
)

type propertyListiness int

const (
	propertyNotList propertyListiness = iota
	propertyList
	propertyEList
	propertyUnknownList
)

type propertyTypeInfo struct {
	d  string // description
	t  PropertyType
	vn string // validator name
	l  propertyListiness
}

type PropertyValue struct {
	Value         string
	validatorName string
}

func (v *PropertyValue) String() (s string) {
	s = "[" + v.Value + "]"
	return
}

func NewPropertyValue(v string) (*PropertyValue, error) {
	pv := &PropertyValue{Value: v, validatorName: "unknown"}
	return pv, nil
}

func NewEmptyPropertyValue() *PropertyValue {
	pv := &PropertyValue{validatorName: "unknown"}
	return pv
}

func (pv *PropertyValue) SetValue(v string) error {
	pv.Value = v
	// TODO: also set the type?
	return nil
}

func (pv *PropertyValue) Number() (i int, err error) {
	switch pv.validatorName {
	case "number":
		i, err = strconv.Atoi(pv.Value)
		if err != nil {
			err = fmt.Errorf("sgf: Number could not be extracted from Property Value: %s", err)
		}
	default:
		err = fmt.Errorf("sgf: Property Value does not contain a Number")
	}
	return
}

func (pv *PropertyValue) Simpletext() (st string, err error) {
	switch pv.validatorName {
	case "simpletext":
		st = pv.Value
	default:
		err = fmt.Errorf("sgf: Property value does not contain a Simpletext")
	}
	return
}

func validatePropertyValue(pv *PropertyValue) error {
	errors := make([]error, 0)

	vn_parts := strings.Split(pv.validatorName, " | ")
	for _, vn_part := range vn_parts {
		composed_parts := strings.Split(vn_part, " : ")
		value_parts := strings.Split(pv.Value, ":")
		if len(composed_parts) != len(value_parts) {
			errors = append(
				errors,
				fmt.Errorf("not enough parts in composition"),
			)
			continue
		}

		var found_error bool
		for i := range value_parts {
			err := validatePropertyValuePart(
				composed_parts[i],
				value_parts[i],
			)
			if err != nil {
				errors = append(
					errors,
					err,
				)
				found_error = true
				break
			}
		}
		if !found_error {
			errors = append(
				errors,
				nil,
			)
		}
	}

	var first_error error
	var error_count int
	for _, err := range errors {
		if err != nil {
			if first_error == nil {
				first_error = err
			}
			error_count++
		}
	}

	if error_count == len(errors) {
		return first_error
	} else {
		return nil
	}
}

func validatePropertyValuePart(validator string, value string) error {
	switch validator {
	case "unknown":
		return nil

	case "none":
		if value == "" {
			return nil
		} else {
			return fmt.Errorf("None type Property Values must be empty")
		}

	case "double":
		switch value {
		case "1", "2":
			return nil
		default:
			return fmt.Errorf("Double type Property Values must be either '1' or '2'")
		}

	case "color":
		switch value {
		case "B", "W":
			return nil
		default:
			return fmt.Errorf("Color type Property Values must be either 'B' or 'W'")
		}

	case "number":
		matched, err := regexp.MatchString(
			`^[\+-]?\d+$`,
			value,
		)
		if matched {
			return nil
		} else {
			if err == nil {
				return fmt.Errorf("Number type Property Value expected positive or negative integer")
			} else {
				return fmt.Errorf("Number type Property Value regular expression error: %s", err)
			}
		}

	case "real":
		matched, err := regexp.MatchString(
			`^[\+-]?\d+(.\d+)?$`,
			value,
		)
		if matched {
			return nil
		} else {
			if err == nil {
				return fmt.Errorf("Real type Property Value expected positive or negative real number")
			} else {
				return fmt.Errorf("Real type Property Value regular expression error: %s", err)
			}
		}

	case "text", "simpletext":
		return nil

	case "move", "stone", "point":
		return fmt.Errorf("validation deferred: must be validated in the context of the full SGF structure")

	default:
		return fmt.Errorf("No validation implemented for type %s and value %s", validator, value)
	}
}

var propertyTypeMap = map[string]propertyTypeInfo{
	"": {
		"Empty property",
		PropertyTypeUnknown,
		"none",
		propertyNotList,
	},
	"B": {
		"Black",
		PropertyTypeMove,
		"move",
		propertyNotList,
	},
	"BL": {
		"Black time left",
		PropertyTypeMove,
		"real",
		propertyNotList,
	},
	"BM": {
		"Bad move",
		PropertyTypeMove,
		"double",
		propertyNotList,
	},
	"DO": {
		"Doubtful",
		PropertyTypeMove,
		"none",
		propertyNotList,
	},
	"IT": {
		"Interesting",
		PropertyTypeMove,
		"none",
		propertyNotList,
	},
	"KO": {
		"Ko",
		PropertyTypeMove,
		"none",
		propertyNotList,
	},
	"MN": {
		"set MoveNumber",
		PropertyTypeMove,
		"none",
		propertyNotList,
	},
	"OB": {
		"OtStones Black",
		PropertyTypeMove,
		"number",
		propertyNotList,
	},
	"OW": {
		"OtStones White",
		PropertyTypeMove,
		"number",
		propertyNotList,
	},
	"TE": {
		"Tesuji",
		PropertyTypeMove,
		"double",
		propertyNotList,
	},
	"W": {
		"White",
		PropertyTypeMove,
		"move",
		propertyNotList,
	},
	"WL": {
		"White time left",
		PropertyTypeMove,
		"real",
		propertyNotList,
	},

	"AB": {
		"Add Black",
		PropertyTypeSetup,
		"stone",
		propertyList,
	},
	"AE": {
		"Add Empty",
		PropertyTypeSetup,
		"point",
		propertyList,
	},
	"AW": {
		"Add White",
		PropertyTypeSetup,
		"stone",
		propertyList,
	},
	"PL": {
		"Player to play",
		PropertyTypeSetup,
		"color",
		propertyNotList,
	},

	"AR": {
		"Arrow",
		PropertyTypeNull,
		"point : point",
		propertyList,
	},
	"C": {
		"Comment",
		PropertyTypeNull,
		"text",
		propertyNotList,
	},
	"CR": {
		"Circle",
		PropertyTypeNull,
		"point",
		propertyList,
	},
	"DD": {
		"Dim points",
		PropertyTypeNull,
		"point",
		propertyEList,
	},
	"DM": {
		"Even position",
		PropertyTypeNull,
		"double",
		propertyNotList,
	},
	"FG": {
		"Figure",
		PropertyTypeNull,
		"none | number : simpletext",
		propertyNotList,
	},
	"GB": {
		"Good for Black",
		PropertyTypeNull,
		"double",
		propertyNotList,
	},
	"GW": {
		"Good for White",
		PropertyTypeNull,
		"double",
		propertyNotList,
	},
	"HO": {
		"Hotspot",
		PropertyTypeNull,
		"double",
		propertyNotList,
	},
	"LB": {
		"Label",
		PropertyTypeNull,
		"point : simpletext",
		propertyList,
	},
	"LN": {
		"Line",
		PropertyTypeNull,
		"point : point",
		propertyList,
	},
	"MA": {
		"Mark",
		PropertyTypeNull,
		"point",
		propertyList,
	},
	"N": {
		"Nodename",
		PropertyTypeNull,
		"simpletext",
		propertyNotList,
	},
	"PM": {
		"Print move mode",
		PropertyTypeNull,
		"number",
		propertyNotList,
	},
	"SL": {
		"Selected",
		PropertyTypeNull,
		"point",
		propertyList,
	},
	"SQ": {
		"Square",
		PropertyTypeNull,
		"point",
		propertyList,
	},
	"TR": {
		"Triangle",
		PropertyTypeNull,
		"point",
		propertyList,
	},
	"UC": {
		"Unclear pos",
		PropertyTypeNull,
		"double",
		propertyNotList,
	},
	"VW": {
		"View",
		PropertyTypeNull,
		"point",
		propertyEList,
	},

	"AP": {
		"Application",
		PropertyTypeRoot,
		"simpletext : number",
		propertyNotList,
	},
	"CA": {
		"Charset",
		PropertyTypeRoot,
		"simpletext",
		propertyNotList,
	},
	"FF": {
		"Fileformat",
		PropertyTypeRoot,
		"number",
		propertyNotList,
	},
	"GM": {
		"Game",
		PropertyTypeRoot,
		"number",
		propertyNotList,
	},
	"ST": {
		"Style",
		PropertyTypeRoot,
		"number",
		propertyNotList,
	},
	"SZ": {
		"Size",
		PropertyTypeRoot,
		"number | number : number",
		propertyNotList,
	},

	"AN": {
		"Annotation",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"BR": {
		"Black rank",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"BT": {
		"Black team",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"CP": {
		"Copyright",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"DT": {
		"Date",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"EV": {
		"Event",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"GC": {
		"Game comment",
		PropertyTypeGameInfo,
		"text",
		propertyNotList,
	},
	"GN": {
		"Game name",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"OP": {
		"Opening",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"PB": {
		"Player Black",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"PC": {
		"Place",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"PW": {
		"Player White",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"RE": {
		"Result",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"RO": {
		"Round",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"RU": {
		"Rules",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"SO": {
		"Source",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"TM": {
		"Timelimit",
		PropertyTypeGameInfo,
		"real",
		propertyNotList,
	},
	"US": {
		"User",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"WR": {
		"White rank",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"WT": {
		"White team",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},

	// Go GM[1] properties
	"TB": {
		"Territory Black",
		PropertyTypeNull,
		"point",
		propertyEList,
	},
	"TW": {
		"Territory White",
		PropertyTypeNull,
		"point",
		propertyEList,
	},
	"HA": {
		"Handicap",
		PropertyTypeGameInfo,
		"number",
		propertyNotList,
	},
	"KM": {
		"Komi",
		PropertyTypeGameInfo,
		"real",
		propertyNotList,
	},

	// Lines of Action GM[9] properties
	"AS": {
		"Who adds stones",
		PropertyTypeNull,
		"simpletext",
		propertyNotList,
	},
	"IP": {
		"Initial pos.",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"IY": {
		"Invert Y-axis",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
	"SE": {
		"Markup",
		PropertyTypeNull,
		"point",
		propertyNotList,
	},
	"SU": {
		"Setup type",
		PropertyTypeGameInfo,
		"simpletext",
		propertyNotList,
	},
}
