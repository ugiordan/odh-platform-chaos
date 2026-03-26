import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Overview } from './Overview';

const mockStats = {
  total: 30,
  resilient: 23,
  degraded: 4,
  failed: 1,
  inconclusive: 0,
  running: 2,
  trends: { total: 5, resilient: 3, degraded: 1, failed: -1 },
  verdictTimeline: null,
  avgRecoveryByType: { PodKill: 12000, ConfigDrift: 28000 },
  runningExperiments: [
    { name: 'exp-1', namespace: 'ns', phase: 'Observing', component: 'comp', type: 'PodKill' },
  ],
};

describe('Overview', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders stat cards with data', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockStats),
    } as Response);

    render(<MemoryRouter><Overview /></MemoryRouter>);

    await waitFor(() => {
      expect(screen.getByText('30')).toBeInTheDocument();
    });
    expect(screen.getByText('23')).toBeInTheDocument();
    expect(screen.getByText('4')).toBeInTheDocument();
  });

  it('renders running experiments', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockStats),
    } as Response);

    render(<MemoryRouter><Overview /></MemoryRouter>);

    await waitFor(() => {
      expect(screen.getByText('exp-1')).toBeInTheDocument();
    });
  });

  it('renders avg recovery times', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockStats),
    } as Response);

    render(<MemoryRouter><Overview /></MemoryRouter>);

    await waitFor(() => {
      expect(screen.getByText('PodKill')).toBeInTheDocument();
      expect(screen.getByText('12.0s')).toBeInTheDocument();
    });
  });

  it('shows spinner while loading', () => {
    vi.spyOn(globalThis, 'fetch').mockReturnValue(new Promise(() => {}));
    render(<MemoryRouter><Overview /></MemoryRouter>);
    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('shows error on fetch failure', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ error: 'db error' }),
    } as unknown as Response);

    render(<MemoryRouter><Overview /></MemoryRouter>);

    await waitFor(() => {
      expect(screen.getByText(/db error/)).toBeInTheDocument();
    });
  });
});
