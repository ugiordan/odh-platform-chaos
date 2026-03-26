import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { App } from './App';

describe('App shell', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    // Mock fetch for Overview page
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        total: 0,
        resilient: 0,
        degraded: 0,
        failed: 0,
        inconclusive: 0,
        running: 0,
        trends: null,
        verdictTimeline: null,
        avgRecoveryByType: {},
        runningExperiments: [],
      }),
    } as Response);
  });

  it('renders the sidebar with navigation groups', () => {
    render(<App />);
    expect(screen.getByText('Monitor')).toBeInTheDocument();
    expect(screen.getByText('Experiments')).toBeInTheDocument();
    expect(screen.getByText('Insights')).toBeInTheDocument();
  });

  it('renders navigation links', () => {
    render(<App />);
    expect(screen.getByRole('link', { name: /overview/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /live/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /all experiments/i })).toBeInTheDocument();
  });

  it('renders the overview page at root route', async () => {
    render(<App />);
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /overview/i })).toBeInTheDocument();
    });
  });
});
