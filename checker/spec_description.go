package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

type describedParameter struct {
	name  string
	value value.Value
}

type describedAction struct {
	name       string
	location   parser.Range
	parameters []describedParameter
}

type specDescription struct {
	model *model
	sets  map[string][]value.Value
}

func (r *tlcRuntime) specificationValue() (value.Value, error) {
	if r.model == nil {
		return value.Value{}, tlcRuntimeError("TLCGet(spec) was undefined.")
	}
	return r.model.describeSpecification()
}

func (m *model) describeSpecification() (value.Value, error) {
	d := specDescription{model: m, sets: map[string][]value.Value{}}
	if err := d.collectConfigured(); err != nil {
		return value.Value{}, err
	}
	if err := d.collectVariablesAndConstraints(); err != nil {
		return value.Value{}, err
	}
	fields := map[string]value.Value{}
	for _, name := range []string{"inits", "actions", "temporals", "invariants", "impliedinits", "impliedactions", "impliedtemporals", "variables", "actionconstraints", "constraints"} {
		set, err := value.Set(d.sets[name])
		if err != nil {
			return value.Value{}, err
		}
		fields[name] = set
	}
	return makeRecord(fields)
}

func (d *specDescription) collectConfigured() error {
	cfg := d.model.descriptionConfig
	if cfg.Specification != "" {
		definition, ok := d.model.defs[cfg.Specification]
		if !ok {
			return fmt.Errorf("undefined specification %s", cfg.Specification)
		}
		if err := d.collectSpec(definition.Body, map[string]bool{}); err != nil {
			return err
		}
	}
	if err := d.collectInitNext(cfg.Init, cfg.Next); err != nil {
		return err
	}
	return d.collectProperties(cfg.Invariants, cfg.Properties)
}

func (d *specDescription) collectInitNext(init, next string) error {
	if init != "" {
		definition := d.model.defs[init]
		if err := d.add("inits", describedAction{name: init, location: definition.Body.Location.Range}); err != nil {
			return err
		}
	}
	if next != "" {
		definition := d.model.defs[next]
		return d.collectActions(definition.Body, actionDescriptionScope{name: next}, nil, map[string]bool{})
	}
	return nil
}

func (d *specDescription) collectVariablesAndConstraints() error {
	for _, v := range d.model.module.Variables {
		if err := d.add("variables", describedAction{name: v.Name, location: v.Location.Range}); err != nil {
			return err
		}
	}
	cfg := d.model.descriptionConfig
	if err := d.collectDefinitionSet("constraints", cfg.Constraints); err != nil {
		return err
	}
	return d.collectDefinitionSet("actionconstraints", cfg.ActionConstraints)
}

func (d *specDescription) collectDefinitionSet(category string, names []string) error {
	for _, name := range names {
		definition, ok := d.model.defs[name]
		if !ok || d.isConstantDefinition(definition) {
			continue
		}
		if err := d.add(category, describedAction{name: name, location: definition.Location.Range}); err != nil {
			return err
		}
	}
	return nil
}

func (d *specDescription) add(category string, action describedAction) error {
	entry, err := action.value()
	if err != nil {
		return err
	}
	d.sets[category] = append(d.sets[category], entry)
	return nil
}

func (a describedAction) value() (value.Value, error) {
	location, err := specLocation(a.location)
	if err != nil {
		return value.Value{}, err
	}
	fields := map[string]value.Value{"name": value.String(a.name), "location": location}
	if len(a.parameters) > 0 {
		if err := addParameterFields(fields, a.parameters); err != nil {
			return value.Value{}, err
		}
	}
	return makeRecord(fields)
}

func specLocation(r parser.Range) (value.Value, error) {
	if r.Module == "" {
		r.Module = "--TLA+ BUILTINS--"
	}
	return makeRecord(map[string]value.Value{"beginLine": tlcInt(r.Start.Line), "beginColumn": tlcInt(r.Start.Column), "endLine": tlcInt(r.End.Line), "endColumn": tlcInt(r.End.Column), "module": value.String(r.Module)})
}

func addParameterFields(fields map[string]value.Value, parameters []describedParameter) error {
	context := make(map[string]value.Value, len(parameters))
	names := make([]value.Value, len(parameters))
	for i, p := range parameters {
		context[p.name] = p.value
		names[i] = value.String(p.name)
	}
	var err error
	fields["context"], err = makeRecord(context)
	if err != nil {
		return err
	}
	fields["parameters"], err = value.Tuple(names)
	return err
}

func (d *specDescription) isConstantDefinition(definition parser.Definition) bool {
	if definition.Kind != "Operator" || len(definition.Params) != 0 || definition.Body.Kind == "Number" || definition.Body.Kind == "String" {
		return false
	}
	return !d.model.readsState(definition.Body, map[string]bool{}) && !d.model.hasAction(definition.Body, map[string]bool{}) && !d.model.hasTemporal(definition.Body, map[string]bool{})
}
