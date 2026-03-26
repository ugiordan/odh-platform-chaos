import './PhaseStepper.css';
import { phaseDisplayName } from '../types/api';

interface PhaseStepperProps {
  currentPhase: string;
  abortedAtPhase?: string;
}

const PHASES = [
  'Pending',
  'SteadyStatePre',
  'Injecting',
  'Observing',
  'SteadyStatePost',
  'Evaluating',
  'Complete',
];

export function PhaseStepper({ currentPhase, abortedAtPhase }: PhaseStepperProps) {
  const isAborted = currentPhase === 'Aborted';
  const activePhase = isAborted && abortedAtPhase ? abortedAtPhase : currentPhase;
  const activeIdx = PHASES.indexOf(activePhase);

  return (
    <div className="phase-stepper">
      {PHASES.map((phase, idx) => {
        const isDone = idx < activeIdx;
        const isActive = idx === activeIdx;
        const isAbortedStep = isAborted && isActive;

        let dotClass = 'step-dot';
        if (isAbortedStep) {
          dotClass += ' aborted';
        } else if (isDone) {
          dotClass += ' done';
        } else if (isActive) {
          dotClass += ' active';
        } else {
          dotClass += ' pending';
        }

        return (
          <div key={phase} style={{ display: 'flex', alignItems: 'center' }}>
            <div style={{ textAlign: 'center' }}>
              <div className={dotClass}>
                {isDone ? '✓' : idx + 1}
              </div>
              <div className="step-label">{phaseDisplayName(phase)}</div>
            </div>
            {idx < PHASES.length - 1 && (
              <div className={`step-line ${idx < activeIdx ? 'done' : 'pending'}`} />
            )}
          </div>
        );
      })}
    </div>
  );
}
