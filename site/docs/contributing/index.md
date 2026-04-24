# Contributing

Operator Chaos is open to contributions. This section covers how to set up a development environment, add new failure modes, and submit changes.

## Where to Start

| Guide | What It Covers |
|-------|---------------|
| [Development Setup](development-setup.md) | Clone, build, test, run locally |
| [Adding Failure Modes](adding-failure-modes.md) | Implement a new Go injector from scratch |

## Quick Links

- **Repository**: [github.com/ugiordan/operator-chaos](https://github.com/ugiordan/operator-chaos)
- **Issues**: [github.com/ugiordan/operator-chaos/issues](https://github.com/ugiordan/operator-chaos/issues)
- **Language**: Go 1.22+
- **Framework**: controller-runtime, kubebuilder

## Types of Contributions

**YAML experiments (no code)**: Most chaos experiments can be expressed by composing the built-in failure modes with different parameters and targets. See [Creating Custom Failure Modes](../failure-modes/custom-failure-modes.md) for the YAML composition approach.

**Go injectors**: If the fault you need isn't expressible through existing failure modes, you can implement a new injector. See [Adding Failure Modes](adding-failure-modes.md) for the full walkthrough.

**Knowledge models**: Adding topology models for new operators or components. Knowledge models are YAML files in the `knowledge/` directory that describe operator structure, managed resources, and recovery behavior.

**Documentation**: Improvements to these docs are welcome. The site uses MkDocs with the Material theme. Run `cd site && mkdocs serve` to preview locally.
