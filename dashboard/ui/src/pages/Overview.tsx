import { Spinner, Alert, EmptyState, EmptyStateBody } from '@patternfly/react-core';
import { useApi } from '../api/hooks';
import { apiUrl } from '../api/client';
import { TrendIndicator } from '../components/TrendIndicator';
import { PhaseBadge } from '../components/PhaseBadge';
import type { OverviewStats } from '../types/api';
import { formatMs } from '../utils/format';
import './Overview.css';

export function Overview() {
  const { data, loading, error } = useApi<OverviewStats>(apiUrl('/overview/stats'));

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
        <Alert variant="danger" title="Failed to load overview">{error}</Alert>
      </div>
    );
  }

  if (!data) {
    return (
      <EmptyState>
        <EmptyStateBody>No data available.</EmptyStateBody>
      </EmptyState>
    );
  }

  const cards = [
    { label: 'Total', value: data.total, color: '', trend: data.trends?.total, goodDir: 'up' as const },
    { label: 'Resilient', value: data.resilient, color: 'stat-value-green', trend: data.trends?.resilient, goodDir: 'up' as const },
    { label: 'Degraded', value: data.degraded, color: 'stat-value-yellow', trend: data.trends?.degraded, goodDir: 'down' as const },
    { label: 'Failed', value: data.failed, color: 'stat-value-red', trend: data.trends?.failed, goodDir: 'down' as const },
    { label: 'Running', value: data.running, color: 'stat-value-blue', trend: undefined, goodDir: 'up' as const },
  ];

  return (
    <>
      <div className="overview-header">
        <h1>Overview</h1>
      </div>
      <div className="overview-content">
        <div className="stat-cards">
          {cards.map((c) => (
            <div key={c.label} className="stat-card">
              <div className={`stat-value ${c.color}`}>{c.value}</div>
              <div className="stat-label">{c.label}</div>
              {c.trend !== undefined && (
                <div className="stat-trend">
                  <TrendIndicator value={c.trend} goodDirection={c.goodDir} />
                </div>
              )}
            </div>
          ))}
        </div>

        <div className="overview-grid">
          <div className="section-card">
            <div className="card-header">Avg Recovery Time by Type</div>
            <div className="card-body">
              {Object.keys(data.avgRecoveryByType).length === 0 ? (
                <div style={{ color: '#6a6e73', fontSize: 13 }}>No recovery data yet</div>
              ) : (
                Object.entries(data.avgRecoveryByType).map(([type, ms]) => (
                  <div key={type} className="recovery-row">
                    <span>{type}</span>
                    <span style={{ fontWeight: 600 }}>{formatMs(ms as number)}</span>
                  </div>
                ))
              )}
            </div>
          </div>

          <div className="section-card">
            <div className="card-header">Running Experiments ({data.runningExperiments.length})</div>
            <div className="card-body">
              {data.runningExperiments.length === 0 ? (
                <div style={{ color: '#6a6e73', fontSize: 13 }}>No experiments running</div>
              ) : (
                data.runningExperiments.map((exp) => (
                  <div key={`${exp.namespace}/${exp.name}`} className="running-item">
                    <div>
                      <div style={{ fontWeight: 500 }}>{exp.name}</div>
                      <div style={{ fontSize: 12, color: '#6a6e73' }}>{exp.component} / {exp.type}</div>
                    </div>
                    <PhaseBadge phase={exp.phase} />
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
