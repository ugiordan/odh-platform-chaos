import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ExperimentDetail } from './ExperimentDetail';

const mockExp = {
  id: 'opendatahub/omc-podkill/2026-03-25T10:00:00Z',
  name: 'omc-podkill',
  namespace: 'opendatahub',
  operator: 'odh-model-controller',
  component: 'controller',
  type: 'PodKill',
  phase: 'Complete',
  verdict: 'Resilient',
  dangerLevel: 'medium',
  recoveryMs: 12000,
  startTime: '2026-03-25T10:00:00Z',
  endTime: '2026-03-25T10:05:00Z',
  cleanupError: '',
  specJson: JSON.stringify({
    target: { operator: 'odh-model-controller', component: 'controller' },
    injection: { type: 'PodKill', dangerLevel: 'medium' },
    hypothesis: 'Controller recovers within 60s',
  }),
  statusJson: JSON.stringify({
    phase: 'Complete',
    verdict: 'Resilient',
    message: 'Experiment completed successfully',
    evaluationResult: { verdict: 'Resilient', confidence: 95, recoveryTime: '12s', reconcileCycles: 3 },
    steadyStatePre: { checks: [{ name: 'pod-running', passed: true, value: '1/1' }] },
    steadyStatePost: { checks: [{ name: 'pod-running', passed: true, value: '1/1' }] },
    injectionLog: [{ timestamp: '2026-03-25T10:01:00Z', action: 'inject', target: 'pod/omc-abc', details: 'killed' }],
    conditions: [{ type: 'Ready', status: 'True', reason: 'Complete' }],
  }),
  createdAt: '2026-03-25T10:00:00Z',
  updatedAt: '2026-03-25T10:05:00Z',
};

function renderDetail() {
  return render(
    <MemoryRouter initialEntries={['/experiments/opendatahub/omc-podkill']}>
      <Routes>
        <Route path="/experiments/:namespace/:name" element={<ExperimentDetail />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('ExperimentDetail', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockExp),
    } as Response);
  });

  it('renders experiment name and badges', async () => {
    renderDetail();
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'omc-podkill' })).toBeInTheDocument();
      expect(screen.getAllByText('Resilient').length).toBeGreaterThan(0);
      expect(screen.getAllByText('Complete').length).toBeGreaterThan(0);
      expect(screen.getAllByText('medium').length).toBeGreaterThan(0);
    });
  });

  it('renders all 7 tab labels', async () => {
    renderDetail();
    await waitFor(() => {
      expect(screen.getByText('Summary')).toBeInTheDocument();
      expect(screen.getByText('Evaluation')).toBeInTheDocument();
      expect(screen.getByText('Steady State')).toBeInTheDocument();
      expect(screen.getByText('Injection Log')).toBeInTheDocument();
      expect(screen.getByText('Conditions')).toBeInTheDocument();
      expect(screen.getByText('YAML')).toBeInTheDocument();
      expect(screen.getByText('Debug')).toBeInTheDocument();
    });
  });

  it('shows Summary tab content by default', async () => {
    renderDetail();
    await waitFor(() => {
      expect(screen.getByText('odh-model-controller')).toBeInTheDocument();
    });
  });

  it('shows cleanup error banner when present', async () => {
    const expWithError = { ...mockExp, cleanupError: 'failed to remove finalizer' };
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(expWithError),
    } as Response);

    renderDetail();
    await waitFor(() => {
      expect(screen.getByText('Cleanup error: failed to remove finalizer')).toBeInTheDocument();
    });
  });

  it('shows error when experiment not found', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ error: 'experiment not found' }),
    } as unknown as Response);

    renderDetail();
    await waitFor(() => {
      expect(screen.getByText('Failed to load experiment')).toBeInTheDocument();
      expect(screen.getByText('experiment not found')).toBeInTheDocument();
    });
  });
});
