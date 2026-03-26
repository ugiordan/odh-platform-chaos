import { phaseDisplayName } from '../types/api';

interface Props {
  phase: string;
}

const PHASE_VARIANT: Record<string, string> = {
  Complete: 'complete',
  Aborted: 'aborted',
  Pending: 'pending',
};

export function PhaseBadge({ phase }: Props) {
  const variant = PHASE_VARIANT[phase] ?? 'running';
  return <span className={`badge badge-${variant}`}>{phaseDisplayName(phase)}</span>;
}
