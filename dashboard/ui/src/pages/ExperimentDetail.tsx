import { useState, useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Spinner, Alert, Tabs, Tab, TabTitleText, Button, EmptyState, EmptyStateBody } from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { useApi } from '../api/hooks';
import { apiUrl } from '../api/client';
import { VerdictBadge } from '../components/VerdictBadge';
import { PhaseBadge } from '../components/PhaseBadge';
import { StatusBanner } from '../components/StatusBanner';
import type { Experiment } from '../types/api';
import { formatMs, formatDate } from '../utils/format';
import './ExperimentDetail.css';

export function ExperimentDetail() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const [activeTab, setActiveTab] = useState(0);

  const { data: exp, loading, error } = useApi<Experiment>(
    namespace && name ? apiUrl(`/experiments/${namespace}/${name}`) : null
  );

  const spec = useMemo(() => {
    if (!exp) return null;
    try { return JSON.parse(exp.specJson); } catch { return null; }
  }, [exp]);

  const status = useMemo(() => {
    if (!exp) return null;
    try { return JSON.parse(exp.statusJson); } catch { return null; }
  }, [exp]);

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
        <Alert variant="danger" title="Failed to load experiment">{error}</Alert>
      </div>
    );
  }

  if (!exp) {
    return (
      <EmptyState>
        <EmptyStateBody>Experiment not found.</EmptyStateBody>
      </EmptyState>
    );
  }

  const evaluation = status?.evaluationResult;

  return (
    <>
      <div className="detail-breadcrumb">
        <Link to="/experiments">Experiments</Link> / {namespace} / {name}
      </div>

      {exp.cleanupError && (
        <StatusBanner variant="error" message={`Cleanup error: ${exp.cleanupError}`} />
      )}
      {status?.message && !exp.cleanupError && (
        <StatusBanner variant="info" message={status.message} />
      )}

      <div className="detail-header">
        <div>
          <h1>{exp.name}</h1>
          <div className="meta">
            <VerdictBadge verdict={exp.verdict ?? ''} />
            <PhaseBadge phase={exp.phase} />
            {exp.dangerLevel && (
              <span className={`detail-danger-badge danger-${exp.dangerLevel}`}>
                {exp.dangerLevel}
              </span>
            )}
            <span style={{ color: '#6a6e73', fontSize: 13 }}>{exp.namespace}</span>
          </div>
        </div>
        <div className="actions">
          <Button
            variant="secondary"
            onClick={() => {
              const blob = new Blob([JSON.stringify({ spec, status }, null, 2)], { type: 'application/json' });
              const url = URL.createObjectURL(blob);
              const a = document.createElement('a');
              a.href = url;
              a.download = `${exp.name}.json`;
              a.click();
              URL.revokeObjectURL(url);
            }}
          >
            Export JSON
          </Button>
        </div>
      </div>

      <div className="detail-tabs">
        <Tabs activeKey={activeTab} onSelect={(_ev, key) => setActiveTab(key as number)}>
          <Tab eventKey={0} title={<TabTitleText>Summary</TabTitleText>}>
            <div className="tab-content">
              <table className="kv-table">
                <tbody>
                  <tr><td>Operator</td><td>{exp.operator}</td></tr>
                  <tr><td>Component</td><td>{exp.component}</td></tr>
                  <tr><td>Injection Type</td><td>{exp.type}</td></tr>
                  <tr><td>Danger Level</td><td>{exp.dangerLevel ?? '—'}</td></tr>
                  <tr><td>Recovery Time</td><td>{formatMs(exp.recoveryMs)}</td></tr>
                  <tr><td>Start Time</td><td>{formatDate(exp.startTime)}</td></tr>
                  <tr><td>End Time</td><td>{formatDate(exp.endTime)}</td></tr>
                  {spec?.hypothesis && <tr><td>Hypothesis</td><td>{spec.hypothesis}</td></tr>}
                  {spec?.blastRadius?.maxPodsAffected !== undefined && (
                    <tr><td>Max Pods Affected</td><td>{spec.blastRadius.maxPodsAffected}</td></tr>
                  )}
                  {spec?.blastRadius?.dryRun !== undefined && (
                    <tr><td>Dry Run</td><td>{spec.blastRadius.dryRun ? 'Yes' : 'No'}</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </Tab>

          <Tab eventKey={1} title={<TabTitleText>Evaluation</TabTitleText>}>
            <div className="tab-content">
              {evaluation ? (
                <table className="kv-table">
                  <tbody>
                    <tr><td>Verdict</td><td><VerdictBadge verdict={evaluation.verdict} /></td></tr>
                    {evaluation.confidence !== undefined && <tr><td>Confidence</td><td>{evaluation.confidence}%</td></tr>}
                    {evaluation.recoveryTime && <tr><td>Recovery Time</td><td>{evaluation.recoveryTime}</td></tr>}
                    {evaluation.reconcileCycles !== undefined && <tr><td>Reconcile Cycles</td><td>{evaluation.reconcileCycles}</td></tr>}
                    {evaluation.deviations && evaluation.deviations.length > 0 && (
                      <tr><td>Deviations</td><td><ul>{evaluation.deviations.map((d: string, i: number) => <li key={i}>{d}</li>)}</ul></td></tr>
                    )}
                  </tbody>
                </table>
              ) : (
                <div style={{ color: '#6a6e73' }}>No evaluation data available.</div>
              )}
            </div>
          </Tab>

          <Tab eventKey={2} title={<TabTitleText>Steady State</TabTitleText>}>
            <div className="tab-content">
              {['Pre-check', 'Post-check'].map((label, idx) => {
                const checks = idx === 0 ? status?.steadyStatePre?.checks : status?.steadyStatePost?.checks;
                return (
                  <div key={label} style={{ marginBottom: 24 }}>
                    <h3>{label}</h3>
                    {checks && checks.length > 0 ? (
                      checks.map((c: { name: string; passed: boolean; value?: string; error?: string }, i: number) => (
                        <div key={i} className="check-row">
                          <span className={c.passed ? 'check-pass' : 'check-fail'}>
                            {c.passed ? '✓' : '✗'}
                          </span>
                          <span style={{ fontWeight: 500 }}>{c.name}</span>
                          {c.value && <span style={{ color: '#6a6e73' }}>({c.value})</span>}
                          {c.error && <span className="check-fail">{c.error}</span>}
                        </div>
                      ))
                    ) : (
                      <div style={{ color: '#6a6e73' }}>No checks recorded.</div>
                    )}
                  </div>
                );
              })}
            </div>
          </Tab>

          <Tab eventKey={3} title={<TabTitleText>Injection Log</TabTitleText>}>
            <div className="tab-content">
              {status?.injectionLog && status.injectionLog.length > 0 ? (
                status.injectionLog.map((entry: { timestamp: string; action: string; target?: string; details?: string }, i: number) => (
                  <div key={i} className="log-entry">
                    <span className="log-time">{new Date(entry.timestamp).toLocaleTimeString()}</span>
                    <strong>{entry.action}</strong>
                    {entry.target && <span> → {entry.target}</span>}
                    {entry.details && <span style={{ color: '#6a6e73' }}> ({entry.details})</span>}
                  </div>
                ))
              ) : (
                <div style={{ color: '#6a6e73' }}>No injection events recorded.</div>
              )}
            </div>
          </Tab>

          <Tab eventKey={4} title={<TabTitleText>Conditions</TabTitleText>}>
            <div className="tab-content">
              {status?.conditions && status.conditions.length > 0 ? (
                <Table aria-label="Conditions" variant="compact">
                  <Thead>
                    <Tr>
                      <Th>Type</Th>
                      <Th>Status</Th>
                      <Th>Reason</Th>
                      <Th>Message</Th>
                      <Th>Last Transition</Th>
                    </Tr>
                  </Thead>
                  <Tbody>
                    {status.conditions.map((c: { type: string; status: string; reason?: string; message?: string; lastTransitionTime?: string }, i: number) => (
                      <Tr key={i}>
                        <Td>{c.type}</Td>
                        <Td>{c.status}</Td>
                        <Td>{c.reason ?? '—'}</Td>
                        <Td>{c.message ?? '—'}</Td>
                        <Td>{formatDate(c.lastTransitionTime)}</Td>
                      </Tr>
                    ))}
                  </Tbody>
                </Table>
              ) : (
                <div style={{ color: '#6a6e73' }}>No conditions.</div>
              )}
            </div>
          </Tab>

          <Tab eventKey={5} title={<TabTitleText>YAML</TabTitleText>}>
            <div className="tab-content">
              <div className="yaml-actions">
                <Button
                  variant="secondary"
                  onClick={() => navigator.clipboard.writeText(JSON.stringify({ spec, status }, null, 2))}
                >
                  Copy
                </Button>
              </div>
              <pre className="yaml-block">
                {JSON.stringify({ spec, status }, null, 2)}
              </pre>
            </div>
          </Tab>

          <Tab eventKey={6} title={<TabTitleText>Debug</TabTitleText>}>
            <div className="tab-content">
              <table className="kv-table">
                <tbody>
                  <tr><td>Observed Generation</td><td>{status?.observedGeneration ?? '—'}</td></tr>
                  <tr><td>Cleanup Error</td><td>{exp.cleanupError || '—'}</td></tr>
                  <tr><td>Created At</td><td>{formatDate(exp.createdAt)}</td></tr>
                  <tr><td>Updated At</td><td>{formatDate(exp.updatedAt)}</td></tr>
                </tbody>
              </table>
              <h3 style={{ marginTop: 24 }}>Raw Status JSON</h3>
              <details>
                <summary>Expand</summary>
                <pre className="yaml-block">{JSON.stringify(status, null, 2)}</pre>
              </details>
            </div>
          </Tab>
        </Tabs>
      </div>
    </>
  );
}
