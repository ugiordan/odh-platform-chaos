import { useState, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { Spinner, Alert, EmptyState, EmptyStateBody } from '@patternfly/react-core';
import { useApi } from '../api/hooks';
import { apiUrl } from '../api/client';
import { ProgressBar } from '../components/ProgressBar';
import { VerdictBadge } from '../components/VerdictBadge';
import { CoverageMatrix } from '../components/CoverageMatrix';
import type { ListResult, Experiment } from '../types/api';
import { formatMs } from '../utils/format';
import './Operators.css';

function formatDate(dateStr: string | undefined): string {
  if (!dateStr) return '—';
  const date = new Date(dateStr);
  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

interface OperatorCardProps {
  operatorName: string;
}

function OperatorCard({ operatorName }: OperatorCardProps) {
  const [expandedComponent, setExpandedComponent] = useState<string | null>(null);

  // Fetch experiments for this operator
  const { data, loading, error } = useApi<ListResult>(
    apiUrl('/experiments', { operator: operatorName, pageSize: '200' })
  );

  const experiments = data?.items || [];

  // Compute verdict counts
  const verdictCounts = useMemo(() => {
    const counts = { resilient: 0, degraded: 0, failed: 0 };
    for (const exp of experiments) {
      if (exp.verdict === 'Resilient') counts.resilient++;
      else if (exp.verdict === 'Degraded') counts.degraded++;
      else if (exp.verdict === 'Failed') counts.failed++;
    }
    return counts;
  }, [experiments]);

  // Group experiments by component
  const experimentsByComponent = useMemo(() => {
    const grouped: Record<string, Experiment[]> = {};
    for (const exp of experiments) {
      if (!grouped[exp.component]) {
        grouped[exp.component] = [];
      }
      grouped[exp.component]!.push(exp);
    }
    return grouped;
  }, [experiments]);

  const components = Object.keys(experimentsByComponent).sort();

  if (loading) {
    return (
      <div className="operator-card">
        <div className="operator-card-header">
          <div className="operator-name">{operatorName}</div>
        </div>
        <div style={{ padding: 24, textAlign: 'center' }}>
          <Spinner size="md" aria-label="Loading" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="operator-card">
        <div className="operator-card-header">
          <div className="operator-name">{operatorName}</div>
        </div>
        <div style={{ padding: 16 }}>
          <Alert variant="warning" title="Failed to load experiments" isInline>
            {error}
          </Alert>
        </div>
      </div>
    );
  }

  const toggleComponent = (component: string) => {
    setExpandedComponent(expandedComponent === component ? null : component);
  };

  return (
    <div className="operator-card">
      <div className="operator-card-header">
        <div className="operator-name">{operatorName}</div>
        <div className="operator-stats">
          <div className="mini-badge">
            <div className="mini-dot green" />
            <span>{verdictCounts.resilient}</span>
          </div>
          <div className="mini-badge">
            <div className="mini-dot yellow" />
            <span>{verdictCounts.degraded}</span>
          </div>
          <div className="mini-badge">
            <div className="mini-dot red" />
            <span>{verdictCounts.failed}</span>
          </div>
          <ProgressBar
            resilient={verdictCounts.resilient}
            degraded={verdictCounts.degraded}
            failed={verdictCounts.failed}
          />
        </div>
      </div>

      {data && data.totalCount > data.items.length && (
        <div style={{ padding: '8px 16px', fontSize: 13, color: '#6a6e73', background: '#f0f0f0' }}>
          Showing {data.items.length} of {data.totalCount} experiments
        </div>
      )}

      {components.length === 0 ? (
        <div style={{ padding: 24, textAlign: 'center', color: '#6a6e73' }}>
          No experiments found for this operator.
        </div>
      ) : (
        components.map((component) => {
          const componentExperiments = experimentsByComponent[component] || [];
          const isExpanded = expandedComponent === component;

          // Compute component-level verdict counts
          const compCounts = { resilient: 0, degraded: 0, failed: 0 };
          for (const exp of componentExperiments) {
            if (exp.verdict === 'Resilient') compCounts.resilient++;
            else if (exp.verdict === 'Degraded') compCounts.degraded++;
            else if (exp.verdict === 'Failed') compCounts.failed++;
          }

          // Get recent experiments (last 5, sorted by date)
          const recentExperiments = [...componentExperiments]
            .sort((a, b) => {
              const dateA = a.endTime || a.startTime || a.createdAt;
              const dateB = b.endTime || b.startTime || b.createdAt;
              return new Date(dateB).getTime() - new Date(dateA).getTime();
            })
            .slice(0, 5);

          return (
            <div key={component} className="component-section">
              <div className="component-header" onClick={() => toggleComponent(component)}>
                <span className={`component-chevron ${isExpanded ? 'expanded' : ''}`}>›</span>
                <span className="component-name">{component}</span>
                <div className="component-stats">
                  <div className="mini-badge">
                    <div className="mini-dot green" />
                    <span>{compCounts.resilient}</span>
                  </div>
                  <div className="mini-badge">
                    <div className="mini-dot yellow" />
                    <span>{compCounts.degraded}</span>
                  </div>
                  <div className="mini-badge">
                    <div className="mini-dot red" />
                    <span>{compCounts.failed}</span>
                  </div>
                </div>
              </div>

              {isExpanded && (
                <div className="component-body">
                  <div className="component-coverage">
                    <div className="component-coverage-label">Test Coverage</div>
                    <CoverageMatrix experiments={componentExperiments} />
                  </div>

                  <div className="component-experiments">
                    <div className="component-experiments-label">Recent Experiments</div>
                    {recentExperiments.map((exp) => (
                      <div key={exp.id} className="exp-history-row">
                        <Link to={`/experiments/${exp.namespace}/${exp.name}`} className="exp-name">
                          {exp.name}
                        </Link>
                        <span className="exp-type">{exp.type}</span>
                        <VerdictBadge verdict={exp.verdict ?? ''} />
                        <span className="exp-recovery">{formatMs(exp.recoveryMs)}</span>
                        <span className="exp-date">{formatDate(exp.endTime || exp.startTime)}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          );
        })
      )}
    </div>
  );
}

export function Operators() {
  const { data: operators, loading, error } = useApi<string[]>(apiUrl('/operators'));

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', padding: 80 }}>
        <Spinner aria-label="Loading" />
      </div>
    );
  }

  if (error) {
    return (
      <div style={{ padding: 24 }}>
        <Alert variant="danger" title="Failed to load operators">{error}</Alert>
      </div>
    );
  }

  if (!operators || operators.length === 0) {
    return (
      <>
        <div className="operators-header">
          <h1>Operators</h1>
          <p>Per-operator resilience insights and test coverage</p>
        </div>
        <div style={{ padding: 24 }}>
          <EmptyState>
            <EmptyStateBody>No operators found. Run experiments to see operator data here.</EmptyStateBody>
          </EmptyState>
        </div>
      </>
    );
  }

  return (
    <>
      <div className="operators-header">
        <h1>Operators</h1>
        <p>Per-operator resilience insights and test coverage</p>
      </div>

      <div className="operators-content">
        {operators.map((operatorName) => (
          <OperatorCard key={operatorName} operatorName={operatorName} />
        ))}
      </div>
    </>
  );
}
