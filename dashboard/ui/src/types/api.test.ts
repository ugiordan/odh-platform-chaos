import { describe, it, expect } from 'vitest';
import type { Experiment, ManagedResource, ComponentModel } from './api';
import { VERDICTS, PHASES, INJECTION_TYPES, DANGER_LEVELS, phaseDisplayName } from './api';

describe('API types', () => {
  it('defines all injection types', () => {
    expect(INJECTION_TYPES).toHaveLength(8);
    expect(INJECTION_TYPES).toContain('PodKill');
    expect(INJECTION_TYPES).toContain('ClientFault');
  });

  it('defines all phases', () => {
    expect(PHASES).toHaveLength(8);
    expect(PHASES).toContain('Pending');
    expect(PHASES).toContain('Aborted');
  });

  it('defines all verdicts', () => {
    expect(VERDICTS).toHaveLength(4);
    expect(VERDICTS).toContain('Resilient');
    expect(VERDICTS).toContain('Inconclusive');
  });

  it('defines all danger levels', () => {
    expect(DANGER_LEVELS).toHaveLength(3);
  });

  it('maps phase to display name', () => {
    expect(phaseDisplayName('SteadyStatePre')).toBe('Pre-check');
    expect(phaseDisplayName('SteadyStatePost')).toBe('Post-check');
    expect(phaseDisplayName('Injecting')).toBe('Injecting');
    expect(phaseDisplayName('Unknown')).toBe('Unknown');
  });

  it('type-checks an Experiment shape', () => {
    const exp: Experiment = {
      id: 'ns/name/2026-01-01T00:00:00Z',
      name: 'test',
      namespace: 'ns',
      operator: 'op',
      component: 'comp',
      type: 'PodKill',
      phase: 'Complete',
      specJson: '{}',
      statusJson: '{}',
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    };
    expect(exp.name).toBe('test');
  });
});

describe('Knowledge types', () => {
  it('defines ManagedResource shape', () => {
    const r: ManagedResource = {
      apiVersion: 'apps/v1',
      kind: 'Deployment',
      name: 'odh-model-controller',
    };
    expect(r.kind).toBe('Deployment');
  });

  it('defines ComponentModel shape', () => {
    const c: ComponentModel = {
      name: 'odh-model-controller',
      controller: 'odh-model-controller',
      managedResources: [],
    };
    expect(c.name).toBe('odh-model-controller');
  });
});
