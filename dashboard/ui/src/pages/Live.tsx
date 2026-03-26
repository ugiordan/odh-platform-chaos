import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useSSE } from '../api/hooks';
import { PhaseStepper } from '../components/PhaseStepper';
import { PhaseBadge } from '../components/PhaseBadge';
import { StatusBanner } from '../components/StatusBanner';
import type { Experiment } from '../types/api';
import './Live.css';

function formatElapsed(startTime?: string): string {
  if (!startTime) return '—';
  const elapsed = Date.now() - new Date(startTime).getTime();
  const seconds = Math.floor(elapsed / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);

  if (hours > 0) {
    return `${hours}h ${minutes % 60}m`;
  }
  if (minutes > 0) {
    return `${minutes}m ${seconds % 60}s`;
  }
  return `${seconds}s`;
}

export function Live() {
  const { events, connected, error } = useSSE<Experiment>('/api/v1/experiments/live');
  const [, setTick] = useState(0);

  // P2-8: Auto-update elapsed time every second
  useEffect(() => {
    const timer = setInterval(() => setTick(t => t + 1), 1000);
    return () => clearInterval(timer);
  }, []);

  // Filter out completed/aborted experiments
  const runningExperiments = events.filter(
    (exp) => exp.phase !== 'Complete' && exp.phase !== 'Aborted'
  );

  return (
    <>
      <div className="live-header">
        <div>
          <h1>
            Live Monitoring
            <div className={`live-dot${!connected ? ' disconnected' : ''}`} />
          </h1>
          <div className="subtitle">
            Real-time experiment progress • {runningExperiments.length} running
          </div>
        </div>
      </div>

      {error && !connected && (
        <StatusBanner variant="warning" message={error} />
      )}

      <div className="live-content">
        {runningExperiments.length === 0 ? (
          <div className="live-empty">
            <h3>No experiments currently running</h3>
            <p>New experiments will appear here automatically when they start</p>
          </div>
        ) : (
          runningExperiments.map((exp) => {
            const isInjecting = exp.phase === 'Injecting';

            return (
              <div
                key={exp.id}
                className={`live-card${isInjecting ? ' injecting' : ''}`}
              >
                <div className="live-card-header">
                  <div>
                    <h3>
                      {exp.name}
                      <PhaseBadge phase={exp.phase} />
                    </h3>
                    <div className="live-card-meta">
                      <span>Operator: {exp.operator}</span>
                      <span>Type: {exp.type}</span>
                      <span>Started: {exp.startTime ? new Date(exp.startTime).toLocaleTimeString() : '—'}</span>
                    </div>
                  </div>
                  <div className="live-actions">
                    <Link
                      to={`/experiments/${exp.namespace}/${exp.name}`}
                      style={{
                        fontSize: 13,
                        color: '#06c',
                        textDecoration: 'none',
                      }}
                    >
                      View Detail →
                    </Link>
                  </div>
                </div>

                <PhaseStepper currentPhase={exp.phase} />

                <div className="live-progress-info">
                  <div className="live-progress-item">
                    <span className="live-progress-label">Elapsed:</span>
                    <span className="live-progress-value">{formatElapsed(exp.startTime)}</span>
                  </div>
                  <div className="live-progress-item">
                    <span className="live-progress-label">Component:</span>
                    <span className="live-progress-value">{exp.component}</span>
                  </div>
                  <div className="live-progress-item">
                    <span className="live-progress-label">Injection:</span>
                    <span className="live-progress-value">{exp.type}</span>
                  </div>
                </div>
              </div>
            );
          })
        )}
      </div>
    </>
  );
}
