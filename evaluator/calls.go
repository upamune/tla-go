package evaluator

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (e *evaluator) callKey(d parser.Definition, args []value.Value, defined *scope) (string, error) {
	key, err := callArgumentsKey(d, args, defined)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(key)
	if err := appendState(&b, e.context.Current); err != nil {
		return "", err
	}
	if err := appendState(&b, e.context.Next); err != nil {
		return "", err
	}
	return b.String(), nil
}

func callArgumentsKey(d parser.Definition, args []value.Value, defined *scope) (string, error) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%p:%s:", defined, d.Name))
	if err := appendArgumentKeys(&b, args); err != nil {
		return "", err
	}
	return b.String(), nil
}

func appendArgumentKeys(b *strings.Builder, args []value.Value) error {
	for _, v := range args {
		key, err := v.Key()
		if err != nil {
			return err
		}
		appendKey(b, key)
	}
	return nil
}

func appendKey(b *strings.Builder, key string) {
	b.WriteString(strconv.Itoa(len(key)))
	b.WriteByte(':')
	b.WriteString(key)
}

func appendState(b *strings.Builder, state State) error {
	if state == nil {
		b.WriteByte('N')
		return nil
	}
	b.WriteByte('S')
	names := make([]string, 0, len(state))
	for name := range state {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		appendKey(b, name)
		key, err := state[name].Key()
		if err != nil {
			return err
		}
		appendKey(b, key)
	}
	b.WriteByte(';')
	return nil
}
