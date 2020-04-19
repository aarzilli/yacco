package iniparse

import (
	"bufio"
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type SpecialUnmarshalFn func(path string, lineno int, lines []string) (interface{}, error)

type Unmarshaller struct {
	specialUnmarshalFns map[string]SpecialUnmarshalFn
	Path                string // path to the unmarshalled file, used in error reporting
}

func NewUnmarshaller() *Unmarshaller {
	return &Unmarshaller{map[string]SpecialUnmarshalFn{}, "<data>"}
}

func Unmarshal(data []byte, v interface{}) error {
	return NewUnmarshaller().Unmarshal(data, v)
}

func (u *Unmarshaller) AddSpecialUnmarshaller(selector string, fn SpecialUnmarshalFn) {
	u.specialUnmarshalFns[strings.ToLower(selector)] = fn
}

func (u *Unmarshaller) RmSpecialUnmarshaller(selector string) {
	delete(u.specialUnmarshalFns, strings.ToLower(selector))
}

type parserState struct {
	u              *Unmarshaller
	v              interface{}
	lineno         int
	selector       string
	hassubselector bool
	subselector    string
	specialfn      SpecialUnmarshalFn
	specialacc     []string
}

type parserStateFn func(line string, eof bool, ps *parserState) (parserStateFn, error)

func (u *Unmarshaller) Unmarshal(data []byte, v interface{}) error {
	rd := bytes.NewReader(data)
	scanner := bufio.NewScanner(rd)

	var ps parserState
	var psf parserStateFn = outsideParserState

	ps.u = u
	ps.lineno = 1
	ps.v = v

	for scanner.Scan() {
		line := scanner.Text()
		var err error
		psf, err = psf(line, false, &ps)
		if err != nil {
			return err
		}
		ps.lineno++
	}

	if err := scanner.Err(); err != nil {
		// should never happen
		return err
	}

	if _, err := psf("", true, &ps); err != nil {
		return err
	}

	return nil
}

func outsideParserState(line string, eof bool, ps *parserState) (parserStateFn, error) {
	if eof {
		return nil, nil
	}

	tz := tokenize(ps.u.Path, line, ps.lineno)

	t1 := <-tz.ch
	switch t1.t {
	case _BOPEN:
		return headerParser(tz, ps)
	case _EOL:
		return outsideParserState, nil
	default:
		return nil, tokUnexpected(t1, "files must start with a section header")
	}
}

func headerParser(tz *tokenizer, ps *parserState) (parserStateFn, error) {
	selector := ""
	subselector := ""
	hassubselector := false

	t1 := <-tz.ch
	switch t1.t {
	case _SYM:
		selector = strings.ToLower(t1.v)

	default:
		return nil, tokUnexpected(t1, "")
	}

	t2 := <-tz.ch
	switch t2.t {
	case _STR:
		subselector = t2.v
		hassubselector = true

		t3 := <-tz.ch
		switch t3.t {
		case _BCLOSE:
			// nothing more to do
		default:
			return nil, tokUnexpected(t3, "Expecting closing bracket")
		}

	case _BCLOSE:
		// nothing more to do

	default:
		return nil, tokUnexpected(t2, "")
	}

	t4 := <-tz.ch
	if t4.t != _EOL {
		return nil, tokUnexpected(t4, "Expecting end of line after section header")
	}

	ps.selector = selector
	ps.hassubselector = hassubselector
	ps.subselector = subselector

	if fn, ok := ps.u.specialUnmarshalFns[selector]; ok {
		ps.specialfn = fn
		ps.specialacc = []string{}
		return specialParserState, nil
	} else {
		return normalParserState, nil
	}
}

func specialParserState(line string, eof bool, ps *parserState) (parserStateFn, error) {
	if eof {
		err := specialParserFlush(ps)
		return nil, err
	}

	isHeader := false
headerCheckLoop:
	for i := range line {
		switch line[i] {
		case ' ', '\t', '\n':
			// continue
		case '[':
			isHeader = true
		default:
			break headerCheckLoop
		}
	}

	if isHeader {
		err := specialParserFlush(ps)
		if err != nil {
			return nil, err
		}
		return outsideParserState(line, eof, ps)
	} else {
		ps.specialacc = append(ps.specialacc, line)
		return specialParserState, nil
	}
}

func specialParserFlush(ps *parserState) error {
	out, err := ps.specialfn(ps.u.Path, ps.lineno-len(ps.specialacc), ps.specialacc)
	if err != nil {
		return err
	}
	_, err = ps.set(&out)
	return err
}

func normalParserState(line string, eof bool, ps *parserState) (parserStateFn, error) {
	if eof {
		return nil, nil
	}

	name := ""
	value := ""

	tz := tokenize(ps.u.Path, line, ps.lineno)

	t1 := <-tz.ch
	switch t1.t {
	case _BOPEN:
		return headerParser(tz, ps)

	case _EOL:
		// nothing to do
		return normalParserState, nil

	case _SYM, _STR:
		name = t1.v
		// continue to equal or eol

	default:
		return nil, tokUnexpected(t1, "Expecting '[', symbol or end of line")
	}

	t2 := <-tz.ch
	switch t2.t {
	case _EQ:
		// continue to value
	case _EOL:
		err := ps.setVar(name, "")
		return normalParserState, err
	default:
		return nil, tokUnexpected(t2, "Expecting '=' or end of line")
	}

	sv := []string{}
valueReadingLoop:
	for {
		t3 := <-tz.ch
		switch t3.t {
		case _SYM, _STR:
			sv = append(sv, t3.v)
		case _EOL:
			break valueReadingLoop
		default:
			return nil, tokUnexpected(t3, "Expecting string or symbol")
		}
	}

	value = strings.Join(sv, " ")
	err := ps.setVar(name, value)
	return normalParserState, err
}

func (ps *parserState) set(defaultVal *interface{}) (reflect.Value, error) {
	nilv := reflect.ValueOf(nil)
	val := reflect.ValueOf(ps.v)
	if val.Type().Kind() != reflect.Ptr {
		return nilv, fmt.Errorf("Unmarshal argument was not a pointer to a struct")
	}
	val = val.Elem()
	if val.Type().Kind() != reflect.Struct {
		return nilv, fmt.Errorf("Unmarshal argument was not a pointer to a struct")
	}
	field := val.FieldByNameFunc(func(name string) bool {
		return strings.ToLower(name) == ps.selector
	})
	if !field.IsValid() {
		return nilv, fmt.Errorf("%s:%d: Could not find a field named '%s'", ps.u.Path, ps.lineno, ps.selector)
	}

	fname := ps.selector

	if ps.hassubselector {
		fname += " \"" + ps.subselector + "\""

		if field.Type().Kind() != reflect.Map {
			return nilv, fmt.Errorf("%s:%d: Field '%s' is not a map", ps.u.Path, ps.lineno, ps.selector)
		}

		if field.Type().Key().Kind() != reflect.String {
			return nilv, fmt.Errorf("%s:%d: Field '%s' is a map but doesn't have strings as key type", ps.u.Path, ps.lineno, ps.selector)
		}

		if field.IsNil() {
			field.Set(reflect.MakeMap(field.Type()))
		}

		ssv := reflect.ValueOf(ps.subselector)

		if defaultVal == nil {
			nfield := field.MapIndex(ssv)
			if !nfield.IsValid() {
				field.SetMapIndex(ssv, reflect.New(field.Type().Elem().Elem()))
				nfield = field.MapIndex(ssv)
				if !nfield.IsValid() {
					return nilv, fmt.Errorf("%s:%d: Could not create new key '%s' of field '%s'", ps.u.Path, ps.lineno, ps.subselector, ps.selector)
				}
			}
			field = nfield
			if field.Type().Kind() != reflect.Ptr {
				return nilv, fmt.Errorf("%s:%d: Field '%s' must be a map of pointers to structs indexed by string", ps.u.Path, ps.lineno, ps.selector)
			}
			field = field.Elem()
		} else {
			dv := reflect.ValueOf(*defaultVal)

			if !dv.Type().AssignableTo(field.Type().Elem()) {
				return nilv, fmt.Errorf("%s:%d: Field '%s' (of type %s) can not be assigned value returned by special function of type '%s'", ps.u.Path, ps.lineno, fname, field.Type().Elem(), dv.Type().String())
			}

			field.SetMapIndex(ssv, dv)
			return field, nil
		}
	}

	if defaultVal == nil {
		if field.Type().Kind() != reflect.Struct {
			return nilv, fmt.Errorf("%s:%d: Field '%s' is not a struct", ps.u.Path, ps.lineno, fname)
		}
		return field, nil
	} else {
		dv := reflect.ValueOf(*defaultVal)

		if !dv.Type().AssignableTo(field.Type()) {
			return nilv, fmt.Errorf("%s:%d: Field '%s' (of type %s) can not be assigned value returned by special function of type '%s'", ps.u.Path, ps.lineno, fname, field.Type(), dv.Type().String())
		}

		if !field.CanSet() {
			return nilv, fmt.Errorf("%s:%d: Field '%s' can not be set", ps.u.Path, ps.lineno, fname)
		}

		field.Set(dv)
		return field, nil
	}
}

func (ps *parserState) setVar(name string, value string) error {
	field, err := ps.set(nil)
	if err != nil {
		return err
	}

	fname := ps.selector
	if ps.hassubselector {
		fname += " \"" + ps.subselector + "\""
	}

	field = field.FieldByNameFunc(func(n string) bool {
		return name == n
	})

	if !field.IsValid() {
		return fmt.Errorf("%s:%d: Field '%s %s' doesn't exist", ps.u.Path, ps.lineno, fname, name)
	}

	if !field.CanSet() {
		return fmt.Errorf("%s:%d: Field '%s %s' can not be set", ps.u.Path, ps.lineno, fname, name)
	}

	if field.Type().Kind() == reflect.Slice {
		if field.IsNil() {
			field.Set(reflect.MakeSlice(field.Type(), 0, 5))
		}
		v, err := parseAsKind(ps, fname, name, value, field.Type().Elem().Kind())
		if err != nil {
			return err
		}
		field.Set(reflect.Append(field, v))
	} else {
		v, err := parseAsKind(ps, fname, name, value, field.Type().Kind())
		if err != nil {
			return err
		}
		field.Set(v)
	}

	return nil
}

func parseAsKind(ps *parserState, fname, name, value string, kind reflect.Kind) (reflect.Value, error) {
	nilv := reflect.ValueOf(nil)

	switch kind {
	case reflect.Bool:
		switch value {
		case "true", "":
			return reflect.ValueOf(true), nil
		case "false":
			return reflect.ValueOf(false), nil
		default:
			return nilv, fmt.Errorf("%s:%d: Field '%s %s' (bool) can not be set with '%s'", ps.u.Path, ps.lineno, fname, name, value)
		}

	case reflect.Int:
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nilv, fmt.Errorf("%s:%d: Field '%s %s' (int) can not be set with '%s': %v\n", ps.u.Path, ps.lineno, fname, name, value, err)
		}
		vint := int(v)
		return reflect.ValueOf(vint), nil

	case reflect.String:
		return reflect.ValueOf(value), nil

	case reflect.Float64:
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nilv, fmt.Errorf("%s:%d: Field '%s %s' (float) can not be set with '%s': %v\n", ps.u.Path, ps.lineno, fname, name, value, err)
		}
		return reflect.ValueOf(v), nil

	default:
		return nilv, fmt.Errorf("%s:%d: Can not assign to field '%s %s' bad type\n", ps.u.Path, ps.lineno, fname, name)
	}
}
