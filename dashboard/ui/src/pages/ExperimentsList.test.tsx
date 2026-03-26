import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ExperimentsList } from './ExperimentsList';

const mockList = {
  items: [
    {
      id: 'ns/exp1/2026-01-01T00:00:00Z',
      name: 'omc-podkill',
      namespace: 'opendatahub',
      operator: 'odh-model-controller',
      component: 'controller',
      type: 'PodKill',
      phase: 'Complete',
      verdict: 'Resilient',
      recoveryMs: 12000,
      startTime: '2026-03-25T10:00:00Z',
      specJson: '{}',
      statusJson: '{}',
      createdAt: '2026-03-25T10:00:00Z',
      updatedAt: '2026-03-25T10:05:00Z',
    },
    {
      id: 'ns/exp2/2026-01-01T00:00:00Z',
      name: 'omc-configdrift',
      namespace: 'opendatahub',
      operator: 'odh-model-controller',
      component: 'controller',
      type: 'ConfigDrift',
      phase: 'Complete',
      verdict: 'Failed',
      recoveryMs: 45000,
      startTime: '2026-03-25T11:00:00Z',
      specJson: '{}',
      statusJson: '{}',
      createdAt: '2026-03-25T11:00:00Z',
      updatedAt: '2026-03-25T11:05:00Z',
    },
  ],
  totalCount: 2,
};

describe('ExperimentsList', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders experiments in a table', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockList),
    } as Response);

    render(<MemoryRouter><ExperimentsList /></MemoryRouter>);

    await waitFor(() => {
      expect(screen.getByText('omc-podkill')).toBeInTheDocument();
      expect(screen.getByText('omc-configdrift')).toBeInTheDocument();
    });
  });

  it('shows verdict badges', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockList),
    } as Response);

    render(<MemoryRouter><ExperimentsList /></MemoryRouter>);

    await waitFor(() => {
      expect(screen.getByText('Resilient')).toBeInTheDocument();
      expect(screen.getByText('Failed')).toBeInTheDocument();
    });
  });

  it('shows empty state when no experiments', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ items: [], totalCount: 0 }),
    } as Response);

    render(<MemoryRouter><ExperimentsList /></MemoryRouter>);

    await waitFor(() => {
      expect(screen.getByText(/no experiments found/i)).toBeInTheDocument();
    });
  });

  it('includes search param in fetch URL', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockList),
    } as Response);

    render(<MemoryRouter><ExperimentsList /></MemoryRouter>);

    await waitFor(() => {
      expect(fetchSpy).toHaveBeenCalled();
    });

    const searchInput = screen.getByPlaceholderText(/search by name/i);
    fireEvent.change(searchInput, { target: { value: 'omc' } });

    await waitFor(() => {
      const lastCall = fetchSpy.mock.calls[fetchSpy.mock.calls.length - 1]?.[0];
      expect(String(lastCall)).toContain('search=omc');
    });
  });
});
