import { useState, useMemo, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { Spinner, Alert, EmptyState, EmptyStateBody } from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { useApi } from '../api/hooks';
import { apiUrl } from '../api/client';
import { VerdictBadge } from '../components/VerdictBadge';
import { PhaseBadge } from '../components/PhaseBadge';
import { INJECTION_TYPES, VERDICTS, PHASES } from '../types/api';
import type { ListResult } from '../types/api';
import { formatMs, formatDate } from '../utils/format';
import './ExperimentsList.css';

interface Filters {
  operator: string;
  type: string;
  verdict: string;
  phase: string;
  search: string;
}

const EMPTY_FILTERS: Filters = { operator: '', type: '', verdict: '', phase: '', search: '' };

export function ExperimentsList() {
  const navigate = useNavigate();
  const [filters, setFilters] = useState<Filters>(EMPTY_FILTERS);
  const [searchInput, setSearchInput] = useState('');
  const searchTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [page, setPage] = useState(1);
  const pageSize = 10;

  // P2-3: Debounce search input
  useEffect(() => {
    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }
    searchTimeoutRef.current = setTimeout(() => {
      setFilters((prev) => ({ ...prev, search: searchInput }));
      setPage(1);
    }, 300);

    return () => {
      if (searchTimeoutRef.current) {
        clearTimeout(searchTimeoutRef.current);
      }
    };
  }, [searchInput]);

  const url = useMemo(() => apiUrl('/experiments', {
    ...filters,
    page,
    pageSize,
  }), [filters, page]);

  const { data, loading, error } = useApi<ListResult>(url);

  const activeFilters = Object.entries(filters).filter(([, v]) => v !== '');
  const totalPages = data ? Math.ceil(data.totalCount / pageSize) : 0;

  const setFilter = (key: keyof Filters, value: string) => {
    setFilters((prev) => ({ ...prev, [key]: value }));
    setPage(1);
  };

  const clearFilter = (key: keyof Filters) => {
    setFilters((prev) => ({ ...prev, [key]: '' }));
    setPage(1);
  };

  return (
    <>
      <div className="experiments-header">
        <h1>Experiments</h1>
      </div>

      <div className="experiments-toolbar">
        <div className="toolbar-group">
          <span className="toolbar-label">Type</span>
          <select
            className="toolbar-select"
            value={filters.type}
            onChange={(e) => setFilter('type', e.target.value)}
          >
            <option value="">All Types</option>
            {INJECTION_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
          </select>
        </div>

        <div className="toolbar-group">
          <span className="toolbar-label">Verdict</span>
          <select
            className="toolbar-select"
            value={filters.verdict}
            onChange={(e) => setFilter('verdict', e.target.value)}
          >
            <option value="">All Verdicts</option>
            {VERDICTS.map((v) => <option key={v} value={v}>{v}</option>)}
          </select>
        </div>

        <div className="toolbar-group">
          <span className="toolbar-label">Phase</span>
          <select
            className="toolbar-select"
            value={filters.phase}
            onChange={(e) => setFilter('phase', e.target.value)}
          >
            <option value="">All Phases</option>
            {PHASES.map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
        </div>

        <div className="toolbar-divider" />

        <input
          className="toolbar-search"
          type="text"
          placeholder="Search by name..."
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
        />
      </div>

      {activeFilters.length > 0 && (
        <div className="filter-chips">
          {activeFilters.map(([key, value]) => (
            <span key={key} className="filter-chip">
              {key}: {value}
              <button onClick={() => clearFilter(key as keyof Filters)}>&times;</button>
            </span>
          ))}
          <button className="clear-all" onClick={() => { setFilters(EMPTY_FILTERS); setPage(1); }}>
            Clear all
          </button>
        </div>
      )}

      {loading && (
        <div style={{ display: 'flex', justifyContent: 'center', padding: 80 }}>
          <Spinner aria-label="Loading" />
        </div>
      )}

      {error && (
        <div style={{ padding: 24 }}>
          <Alert variant="danger" title="Failed to load experiments">{error}</Alert>
        </div>
      )}

      {data && data.items.length === 0 && (
        <div style={{ padding: 24 }}>
          <EmptyState>
            <EmptyStateBody>No experiments found. Adjust filters or run your first experiment.</EmptyStateBody>
          </EmptyState>
        </div>
      )}

      {data && data.items.length > 0 && (
        <>
          <div className="experiments-table-wrapper">
            <Table aria-label="Experiments" variant="compact">
              <Thead>
                <Tr>
                  <Th>Name</Th>
                  <Th>Operator</Th>
                  <Th>Component</Th>
                  <Th>Type</Th>
                  <Th>Phase</Th>
                  <Th>Verdict</Th>
                  <Th>Recovery</Th>
                  <Th>Date</Th>
                </Tr>
              </Thead>
              <Tbody>
                {data.items.map((exp) => (
                  <Tr
                    key={exp.id}
                    className="experiment-row"
                    onRowClick={() => navigate(`/experiments/${exp.namespace}/${exp.name}`)}
                    isClickable
                  >
                    <Td dataLabel="Name">{exp.name}</Td>
                    <Td dataLabel="Operator">{exp.operator}</Td>
                    <Td dataLabel="Component">{exp.component}</Td>
                    <Td dataLabel="Type">{exp.type}</Td>
                    <Td dataLabel="Phase"><PhaseBadge phase={exp.phase} /></Td>
                    <Td dataLabel="Verdict"><VerdictBadge verdict={exp.verdict ?? ''} /></Td>
                    <Td dataLabel="Recovery">{formatMs(exp.recoveryMs)}</Td>
                    <Td dataLabel="Date">{formatDate(exp.startTime)}</Td>
                  </Tr>
                ))}
              </Tbody>
            </Table>
          </div>

          <div className="experiments-pagination">
            <span>
              Showing {(page - 1) * pageSize + 1}–{Math.min(page * pageSize, data.totalCount)} of {data.totalCount}
            </span>
            <div className="pagination-controls">
              <button className="pagination-btn" disabled={page <= 1} onClick={() => setPage(page - 1)}>
                Previous
              </button>
              <span>Page {page} of {totalPages}</span>
              <button className="pagination-btn" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>
                Next
              </button>
            </div>
          </div>
        </>
      )}
    </>
  );
}
