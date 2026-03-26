import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Suites } from './Suites';
import { vi } from 'vitest';

vi.mock('../api/hooks', () => ({
  useApi: vi.fn(() => ({ data: null, loading: false, error: null, refetch: vi.fn() })),
}));

import { useApi } from '../api/hooks';

function renderSuites() {
  return render(<MemoryRouter><Suites /></MemoryRouter>);
}

describe('Suites', () => {
  it('shows loading state', () => {
    (useApi as ReturnType<typeof vi.fn>).mockReturnValue({
      data: null, loading: true, error: null, refetch: vi.fn(),
    });
    renderSuites();
    expect(screen.getByLabelText(/loading/i)).toBeInTheDocument();
  });

  it('shows empty state when no suites', () => {
    (useApi as ReturnType<typeof vi.fn>).mockReturnValue({
      data: [], loading: false, error: null, refetch: vi.fn(),
    });
    renderSuites();
    expect(screen.getByText(/no suite runs/i)).toBeInTheDocument();
  });

  it('renders suite run cards', () => {
    (useApi as ReturnType<typeof vi.fn>).mockReturnValue({
      data: [{
        suiteName: 'omc-full-suite', suiteRunId: 'run-1',
        operatorVersion: 'v2.10.0', total: 7, resilient: 5, degraded: 1, failed: 1,
      }],
      loading: false, error: null, refetch: vi.fn(),
    });
    renderSuites();
    expect(screen.getByText('omc-full-suite')).toBeInTheDocument();
    expect(screen.getByText('v2.10.0')).toBeInTheDocument();
  });

  it('shows error state', () => {
    (useApi as ReturnType<typeof vi.fn>).mockReturnValue({
      data: null, loading: false, error: 'Server error', refetch: vi.fn(),
    });
    renderSuites();
    expect(screen.getByText(/server error/i)).toBeInTheDocument();
  });
});
