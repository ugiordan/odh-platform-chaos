export const INJECTION_TYPES = [
  'PodKill', 'NetworkPartition', 'CRDMutation', 'ConfigDrift',
  'WebhookDisrupt', 'RBACRevoke', 'FinalizerBlock', 'ClientFault',
] as const;
export type InjectionType = typeof INJECTION_TYPES[number];

export const PHASES = [
  'Pending', 'SteadyStatePre', 'Injecting', 'Observing',
  'SteadyStatePost', 'Evaluating', 'Complete', 'Aborted',
] as const;
export type ExperimentPhase = typeof PHASES[number];

export const VERDICTS = ['Resilient', 'Degraded', 'Failed', 'Inconclusive'] as const;
export type Verdict = typeof VERDICTS[number];

export const DANGER_LEVELS = ['low', 'medium', 'high'] as const;
export type DangerLevel = typeof DANGER_LEVELS[number];

const PHASE_DISPLAY: Record<string, string> = {
  Pending: 'Pending',
  SteadyStatePre: 'Pre-check',
  Injecting: 'Injecting',
  Observing: 'Observing',
  SteadyStatePost: 'Post-check',
  Evaluating: 'Evaluating',
  Complete: 'Complete',
  Aborted: 'Aborted',
};

export function phaseDisplayName(phase: string): string {
  return PHASE_DISPLAY[phase] ?? phase;
}

export interface Experiment {
  id: string;
  name: string;
  namespace: string;
  operator: string;
  component: string;
  type: string;
  phase: string;
  verdict?: string;
  dangerLevel?: string;
  recoveryMs?: number;
  startTime?: string;
  endTime?: string;
  suiteName?: string;
  suiteRunId?: string;
  operatorVersion?: string;
  cleanupError?: string;
  specJson: string;
  statusJson: string;
  createdAt: string;
  updatedAt: string;
}

export interface ListResult {
  items: Experiment[];
  totalCount: number;
}

export interface TrendStats {
  total: number;
  resilient: number;
  degraded: number;
  failed: number;
}

export interface DayVerdicts {
  date: string;
  resilient: number;
  degraded: number;
  failed: number;
}

export interface RunningExperimentSummary {
  name: string;
  namespace: string;
  phase: string;
  component: string;
  type: string;
}

export interface OverviewStats {
  total: number;
  resilient: number;
  degraded: number;
  failed: number;
  inconclusive: number;
  running: number;
  trends: TrendStats | null;
  verdictTimeline: DayVerdicts[] | null;
  avgRecoveryByType: Record<string, number>;
  runningExperiments: RunningExperimentSummary[];
}

export interface SuiteRun {
  suiteName: string;
  suiteRunId: string;
  operatorVersion: string;
  total: number;
  resilient: number;
  degraded: number;
  failed: number;
}

export interface ManagedResource {
  apiVersion: string;
  kind: string;
  name: string;
  namespace?: string;
  labels?: Record<string, string>;
  ownerRef?: string;
  expectedSpec?: Record<string, unknown>;
}

export interface WebhookSpec {
  name: string;
  type: string;
  path: string;
}

export interface ComponentModel {
  name: string;
  controller: string;
  managedResources: ManagedResource[];
  dependencies?: string[];
  webhooks?: WebhookSpec[];
  finalizers?: string[];
}
