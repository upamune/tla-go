package checker

import "fmt"

func (r *instanceResolver) registerInheritedNested() error {
	for _, binding := range r.session.inherited[r.root] {
		name := binding.module.Name
		if existing, ok := r.nested[name]; ok {
			if existing.inherited && existing.parent.Name == binding.parent.Name {
				continue
			}
			return fmt.Errorf("duplicate inherited nested module %s", name)
		}
		if err := r.validateUnit(binding.module, binding.parent); err != nil {
			return err
		}
		r.nested[name] = binding
	}
	return nil
}
