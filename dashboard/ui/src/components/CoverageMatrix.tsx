import { INJECTION_TYPES } from '../types/api';
import type { Experiment } from '../types/api';
import './CoverageMatrix.css';

interface CoverageMatrixProps {
  experiments: Experiment[];
}

export function CoverageMatrix({ experiments }: CoverageMatrixProps) {
  // Group experiments by injection type
  const experimentsByType = experiments.reduce((acc, exp) => {
    if (!acc[exp.type]) {
      acc[exp.type] = [];
    }
    acc[exp.type]!.push(exp);
    return acc;
  }, {} as Record<string, Experiment[]>);

  // Compute cell state for each injection type
  const cells = INJECTION_TYPES.map((type) => {
    const typeExperiments = experimentsByType[type] || [];

    if (typeExperiments.length === 0) {
      return { type, state: 'not-tested', label: '—' };
    }

    const hasFailed = typeExperiments.some((e) => e.verdict === 'Failed');
    const hasDegraded = typeExperiments.some((e) => e.verdict === 'Degraded');
    const count = typeExperiments.length;

    if (hasFailed) {
      return { type, state: 'tested-fail', label: `${count}x ✗` };
    }
    if (hasDegraded) {
      return { type, state: 'tested-warn', label: `${count}x ⚠` };
    }
    return { type, state: 'tested-pass', label: `${count}x ✓` };
  });

  return (
    <div>
      <div className="coverage-grid">
        {INJECTION_TYPES.map((type) => (
          <div key={type} className="coverage-label">
            {type}
          </div>
        ))}
      </div>
      <div className="coverage-grid">
        {cells.map((cell) => (
          <div key={cell.type} className={`coverage-cell ${cell.state}`}>
            {cell.label}
          </div>
        ))}
      </div>
    </div>
  );
}
