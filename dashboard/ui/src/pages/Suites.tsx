import { useState, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { Spinner, Alert, EmptyState, EmptyStateBody } from '@patternfly/react-core';
import { useApi } from '../api/hooks';
import { apiUrl } from '../api/client';
import { ProgressBar } from '../components/ProgressBar';
import { VerdictBadge } from '../components/VerdictBadge';
import type { SuiteRun, Experiment } from '../types/api';
import { formatMs } from '../utils/format';
import './Suites.css';

interface ComparisonResult {
  runA: Experiment[];
  runB: Experiment[];
}

export function Suites() {
  const [expandedRun, setExpandedRun] = useState<string | null>(null);
  const [selectedSuite, setSelectedSuite] = useState<string>('');
  const [compareA, setCompareA] = useState<string>('');
  const [compareB, setCompareB] = useState<string>('');

  // Fetch all suite runs
  const { data: suites, loading, error } = useApi<SuiteRun[]>(apiUrl('/suites'));

  // Fetch experiments for expanded run
  const expandUrl = expandedRun ? apiUrl(`/suites/${expandedRun}`) : null;
  const { data: expandedExperiments, loading: loadingExpanded } = useApi<Experiment[]>(expandUrl);

  // Fetch comparison data
  const compareUrl = useMemo(() => {
    if (!selectedSuite || !compareA || !compareB) return null;
    return apiUrl('/suites/compare', { suite: selectedSuite, runA: compareA, runB: compareB });
  }, [selectedSuite, compareA, compareB]);
  const { data: comparison } = useApi<ComparisonResult>(compareUrl);

  // Group suites by name and filter for comparison
  const suitesByName = useMemo(() => {
    if (!suites) return {};
    const grouped: Record<string, SuiteRun[]> = {};
    for (const suite of suites) {
      const existing = grouped[suite.suiteName];
      if (!existing) {
        grouped[suite.suiteName] = [suite];
      } else {
        existing.push(suite);
      }
    }
    return grouped;
  }, [suites]);

  const suiteNames = Object.keys(suitesByName);
  const showComparison = suiteNames.some((name) => (suitesByName[name]?.length ?? 0) >= 2);

  // Get runs for selected suite for comparison dropdowns
  const compareRuns = selectedSuite ? (suitesByName[selectedSuite] || []) : [];

  // Build comparison table
  const comparisonRows = useMemo(() => {
    if (!comparison || !comparison.runA || !comparison.runB) return [];

    const rows: Array<{
      name: string;
      type: string;
      verdictA: string;
      recoveryA: number | undefined;
      verdictB: string;
      recoveryB: number | undefined;
      delta: 'improved' | 'regressed' | 'same';
    }> = [];

    // Create a map of experiments by name from runB
    const expBMap = new Map(comparison.runB.map((e) => [e.name, e]));

    for (const expA of comparison.runA) {
      const expB = expBMap.get(expA.name);
      if (!expB) continue;

      let delta: 'improved' | 'regressed' | 'same' = 'same';

      // Compare verdicts (Resilient > Degraded > Failed)
      const verdictRank = (v?: string) => {
        if (v === 'Resilient') return 3;
        if (v === 'Degraded') return 2;
        if (v === 'Failed') return 1;
        return 0;
      };

      const rankA = verdictRank(expA.verdict);
      const rankB = verdictRank(expB.verdict);

      if (rankB > rankA) {
        delta = 'improved';
      } else if (rankB < rankA) {
        delta = 'regressed';
      } else if (expA.recoveryMs !== undefined && expB.recoveryMs !== undefined) {
        // Same verdict, check recovery time
        if (expB.recoveryMs < expA.recoveryMs) {
          delta = 'improved';
        } else if (expB.recoveryMs > expA.recoveryMs) {
          delta = 'regressed';
        }
      }

      rows.push({
        name: expA.name,
        type: expA.type,
        verdictA: expA.verdict || '',
        recoveryA: expA.recoveryMs,
        verdictB: expB.verdict || '',
        recoveryB: expB.recoveryMs,
        delta,
      });
    }

    return rows;
  }, [comparison]);

  const toggleExpand = (runId: string) => {
    setExpandedRun(expandedRun === runId ? null : runId);
  };

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
        <Alert variant="danger" title="Failed to load suites">{error}</Alert>
      </div>
    );
  }

  if (!suites || suites.length === 0) {
    return (
      <>
        <div className="suites-header">
          <h1>Suites</h1>
          <p>Suite run history and version-to-version resilience comparison</p>
        </div>
        <div style={{ padding: 24 }}>
          <EmptyState>
            <EmptyStateBody>No suite runs found. Run a test suite to see results here.</EmptyStateBody>
          </EmptyState>
        </div>
      </>
    );
  }

  return (
    <>
      <div className="suites-header">
        <h1>Suites</h1>
        <p>Suite run history and version-to-version resilience comparison</p>
      </div>

      <div className="suites-content">
        {showComparison && (
          <div className="comparison-card">
            <div className="comparison-header">Version-to-Version Comparison</div>
            <div className="comparison-selectors">
              <div className="comparison-select-group">
                <label className="comparison-select-label">Suite</label>
                <select
                  className="comparison-select"
                  value={selectedSuite}
                  onChange={(e) => {
                    setSelectedSuite(e.target.value);
                    setCompareA('');
                    setCompareB('');
                  }}
                >
                  <option value="">Select a suite...</option>
                  {suiteNames
                    .filter((name) => (suitesByName[name]?.length ?? 0) >= 2)
                    .map((name) => (
                      <option key={name} value={name}>
                        {name} ({suitesByName[name]?.length ?? 0} versions)
                      </option>
                    ))}
                </select>
              </div>

              {selectedSuite && (
                <>
                  <div className="comparison-select-group">
                    <label className="comparison-select-label">Version A (baseline)</label>
                    <select
                      className="comparison-select"
                      value={compareA}
                      onChange={(e) => setCompareA(e.target.value)}
                    >
                      <option value="">Select version...</option>
                      {compareRuns.map((run) => (
                        <option key={run.suiteRunId} value={run.suiteRunId}>
                          {run.operatorVersion}
                        </option>
                      ))}
                    </select>
                  </div>

                  <div className="comparison-select-group">
                    <label className="comparison-select-label">Version B (new)</label>
                    <select
                      className="comparison-select"
                      value={compareB}
                      onChange={(e) => setCompareB(e.target.value)}
                    >
                      <option value="">Select version...</option>
                      {compareRuns.map((run) => (
                        <option key={run.suiteRunId} value={run.suiteRunId}>
                          {run.operatorVersion}
                        </option>
                      ))}
                    </select>
                  </div>
                </>
              )}
            </div>

            {comparison && comparisonRows.length > 0 && (
              <div className="comparison-table-wrapper">
                <table className="comparison-table">
                  <thead>
                    <tr>
                      <th>Experiment</th>
                      <th>Type</th>
                      <th className="col-v-old">Version A Verdict</th>
                      <th className="col-v-old">Version A Recovery</th>
                      <th className="col-v-new">Version B Verdict</th>
                      <th className="col-v-new">Version B Recovery</th>
                      <th>Delta</th>
                    </tr>
                  </thead>
                  <tbody>
                    {comparisonRows.map((row) => (
                      <tr key={row.name}>
                        <td>{row.name}</td>
                        <td>{row.type}</td>
                        <td className="col-v-old">
                          <VerdictBadge verdict={row.verdictA} />
                        </td>
                        <td className="col-v-old">{formatMs(row.recoveryA)}</td>
                        <td className="col-v-new">
                          <VerdictBadge verdict={row.verdictB} />
                        </td>
                        <td className="col-v-new">{formatMs(row.recoveryB)}</td>
                        <td>
                          <span className={`delta ${row.delta}`}>
                            {row.delta === 'improved' && '↑ Improved'}
                            {row.delta === 'regressed' && '↓ Regressed'}
                            {row.delta === 'same' && '= No change'}
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}

        <div>
          {suites.map((suite) => {
            const isExpanded = expandedRun === suite.suiteRunId;
            const experiments = isExpanded ? expandedExperiments : null;

            return (
              <div key={suite.suiteRunId} className="suite-card">
                <div
                  className="suite-card-header"
                  onClick={() => toggleExpand(suite.suiteRunId)}
                >
                  <div className="suite-card-title">
                    <span className="suite-card-name">{suite.suiteName}</span>
                    <span className="suite-card-version">{suite.operatorVersion}</span>
                    <span className="suite-card-count">{suite.total} experiments</span>
                  </div>
                  <span className={`suite-card-chevron ${isExpanded ? 'expanded' : ''}`}>›</span>
                </div>

                <div className="suite-summary">
                  <div className="suite-stat">
                    <div className="suite-stat-value green">{suite.resilient}</div>
                    <div className="suite-stat-label">Resilient</div>
                  </div>
                  <div className="suite-stat">
                    <div className="suite-stat-value yellow">{suite.degraded}</div>
                    <div className="suite-stat-label">Degraded</div>
                  </div>
                  <div className="suite-stat">
                    <div className="suite-stat-value red">{suite.failed}</div>
                    <div className="suite-stat-label">Failed</div>
                  </div>
                  <ProgressBar
                    resilient={suite.resilient}
                    degraded={suite.degraded}
                    failed={suite.failed}
                  />
                </div>

                {isExpanded && loadingExpanded && (
                  <div style={{ padding: '16px', textAlign: 'center', color: 'var(--text-secondary)' }}>Loading experiments...</div>
                )}

                {isExpanded && experiments && !loadingExpanded && (
                  <div className="suite-table-wrapper">
                    <table className="suite-table">
                      <thead>
                        <tr>
                          <th>Name</th>
                          <th>Type</th>
                          <th>Verdict</th>
                          <th>Recovery</th>
                        </tr>
                      </thead>
                      <tbody>
                        {experiments.map((exp) => (
                          <tr key={exp.id}>
                            <td>
                              <Link to={`/experiments/${exp.namespace}/${exp.name}`}>
                                {exp.name}
                              </Link>
                            </td>
                            <td>{exp.type}</td>
                            <td>
                              <VerdictBadge verdict={exp.verdict ?? ''} />
                            </td>
                            <td>{formatMs(exp.recoveryMs)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </>
  );
}
