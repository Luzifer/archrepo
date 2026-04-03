package config

import "fmt"

func (f File) validate() (err error) {
	for name, pkg := range f.Packages {
		if pkg.Repo == "" {
			return fmt.Errorf("package %q: repo must not be empty", name)
		}

		for _, dep := range pkg.DependsOn {
			if dep == name {
				return fmt.Errorf("package %q: dependsOn must not reference itself", name)
			}

			if _, ok := f.Packages[dep]; !ok {
				return fmt.Errorf("package %q: dependsOn references unknown package %q", name, dep)
			}
		}
	}

	visiting := make(map[string]bool, len(f.Packages))
	visited := make(map[string]bool, len(f.Packages))

	var walk func(string) error
	walk = func(name string) error {
		if visited[name] {
			return nil
		}
		if visiting[name] {
			return fmt.Errorf("dependency cycle detected at package %q", name)
		}

		visiting[name] = true
		for _, dep := range f.Packages[name].DependsOn {
			if err := walk(dep); err != nil {
				return err
			}
		}
		visiting[name] = false
		visited[name] = true

		return nil
	}

	for name := range f.Packages {
		if err := walk(name); err != nil {
			return err
		}
	}

	return nil
}
