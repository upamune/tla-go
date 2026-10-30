package checker

import (
	"fmt"
	"strconv"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func validate(module *parser.Module, cfg config.Config) error {
	if err := validateModules(module); err != nil {
		return err
	}
	if cfg.Init == "" || cfg.Next == "" {
		return fmt.Errorf("INIT and NEXT are required")
	}
	if unsupportedConfig(cfg) {
		return fmt.Errorf("configuration uses unsupported features")
	}
	return validateAssignments(cfg)
}

func validateModules(module *parser.Module) error {
	if len(module.Instances) > 0 || len(module.Modules) > 0 {
		return fmt.Errorf("module instances and nested modules are not yet supported")
	}
	for _, name := range module.Extends {
		if !supportedModule(name) {
			return fmt.Errorf("module %s is not yet supported", name)
		}
	}
	return nil
}

func supportedModule(name string) bool {
	switch name {
	case "Naturals", "Integers", "FiniteSets", "Sequences", "TLC", "TLCExt", "Bags", "Json", "Randomization":
		return true
	default:
		return false
	}
}

func validateAssignments(cfg config.Config) error {
	for _, a := range cfg.Constants {
		if a.Module != "" || a.Args != nil {
			return fmt.Errorf("scoped and operator constants are not yet supported")
		}
	}
	return nil
}

func unsupportedConfig(c config.Config) bool {
	return unsupportedPredicates(c) || unsupportedModelOptions(c) || unsupportedRuntimeOptions(c)
}

func unsupportedPredicates(c config.Config) bool {
	return c.Specification != ""
}

func unsupportedModelOptions(c config.Config) bool {
	return len(c.Substitutions) > 0
}

func unsupportedRuntimeOptions(c config.Config) bool {
	return c.Periodic != "" || c.RLReward != ""
}

func configValue(v config.Value) (value.Value, error) {
	switch v.Kind {
	case "Number":
		n, err := strconv.ParseInt(v.Text, 10, 64)
		if err != nil {
			return value.Value{}, err
		}
		return value.Integer(n)
	case "String":
		return value.String(v.Text), nil
	case "Boolean":
		return value.Bool(v.Text == "TRUE"), nil
	case "ModelValue":
		return value.Model(v.Text), nil
	case "Set":
		return configSet(v.Elements)
	}
	return value.Value{}, fmt.Errorf("unsupported configuration value %s", v.Kind)
}

func configSet(elements []config.Value) (value.Value, error) {
	items := make([]value.Value, 0, len(elements))
	for _, element := range elements {
		v, err := configValue(element)
		if err != nil {
			return value.Value{}, err
		}
		items = append(items, v)
	}
	return value.Set(items)
}
