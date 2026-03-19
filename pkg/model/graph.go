package model

import (
	"log/slog"
)

// ComponentRef uniquely identifies a component within the dependency graph.
type ComponentRef struct {
	Operator  string
	Component string
}

// ResolvedComponent contains a component reference along with its full
// component data and namespace information.
type ResolvedComponent struct {
	Ref       ComponentRef
	Component *ComponentModel
	Namespace string
}

// DependencyGraph represents the dependency relationships between components
// across one or more operators. Edges point from dependencies to dependents:
// if component A declares dependency "B", then edges[B] contains A.
type DependencyGraph struct {
	components map[ComponentRef]*ResolvedComponent
	edges      map[ComponentRef][]ComponentRef
}

// BuildDependencyGraph constructs a dependency graph from operator knowledge models.
// It resolves dependencies using two rules:
// 1. Intra-operator: exact component name match within the same operator
// 2. Cross-operator: operator name match creates edges from all components in the
//    matched operator to the declaring component
//
// Unresolvable dependencies are logged as warnings but do not cause errors.
// Duplicate edges (same target→dependent pair) are automatically deduplicated.
func BuildDependencyGraph(models []*OperatorKnowledge) (*DependencyGraph, error) {
	g := &DependencyGraph{
		components: make(map[ComponentRef]*ResolvedComponent),
		edges:      make(map[ComponentRef][]ComponentRef),
	}

	if len(models) == 0 {
		return g, nil
	}

	// Index all components
	for _, m := range models {
		for i := range m.Components {
			ref := ComponentRef{Operator: m.Operator.Name, Component: m.Components[i].Name}
			g.components[ref] = &ResolvedComponent{
				Ref:       ref,
				Component: &m.Components[i],
				Namespace: m.Operator.Namespace,
			}
		}
	}

	// Build edges with deduplication
	edgeSet := make(map[[2]ComponentRef]bool)

	for _, m := range models {
		for i := range m.Components {
			comp := &m.Components[i]
			dependent := ComponentRef{Operator: m.Operator.Name, Component: comp.Name}

			for _, dep := range comp.Dependencies {
				resolved := false

				// Rule 1: intra-operator component name match
				target := ComponentRef{Operator: m.Operator.Name, Component: dep}
				if _, exists := g.components[target]; exists {
					key := [2]ComponentRef{target, dependent}
					if !edgeSet[key] {
						edgeSet[key] = true
						g.edges[target] = append(g.edges[target], dependent)
					}
					resolved = true
					continue
				}

				// Rule 2: cross-operator name match
				for _, other := range models {
					if other.Operator.Name == dep {
						resolved = true
						for j := range other.Components {
							target = ComponentRef{Operator: other.Operator.Name, Component: other.Components[j].Name}
							key := [2]ComponentRef{target, dependent}
							if !edgeSet[key] {
								edgeSet[key] = true
								g.edges[target] = append(g.edges[target], dependent)
							}
						}
					}
				}

				if !resolved {
					slog.Warn("unresolvable dependency",
						"operator", m.Operator.Name,
						"component", comp.Name,
						"dependency", dep)
				}
			}
		}
	}

	return g, nil
}

// DirectDependents returns all components that directly depend on the given component.
// Returns nil if there are no dependents or if the component reference is not found.
func (g *DependencyGraph) DirectDependents(ref ComponentRef) []*ResolvedComponent {
	refs := g.edges[ref]
	if len(refs) == 0 {
		return nil
	}

	var result []*ResolvedComponent
	for _, r := range refs {
		if rc, ok := g.components[r]; ok {
			result = append(result, rc)
		}
	}
	return result
}
