import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Operators } from './Operators';
import { vi } from 'vitest';

vi.mock('../api/hooks', () => ({
  useApi: vi.fn(() => ({ data: null, loading: false, error: null, refetch: vi.fn() })),
}));

import { useApi } from '../api/hooks';

function renderOperators() {
  return render(<MemoryRouter><Operators /></MemoryRouter>);
}

describe('Operators', () => {
  it('shows loading state', () => {
    (useApi as ReturnType<typeof vi.fn>).mockReturnValue({
      data: null, loading: true, error: null, refetch: vi.fn(),
    });
    renderOperators();
    expect(screen.getByLabelText(/loading/i)).toBeInTheDocument();
  });

  it('shows empty state when no operators', () => {
    (useApi as ReturnType<typeof vi.fn>).mockReturnValue({
      data: [], loading: false, error: null, refetch: vi.fn(),
    });
    renderOperators();
    expect(screen.getByText(/no operators/i)).toBeInTheDocument();
  });

  it('renders operator names', () => {
    (useApi as ReturnType<typeof vi.fn>).mockImplementation((url: string) => {
      if (url && url.includes('/operators')) {
        return { data: ['opendatahub-operator'], loading: false, error: null, refetch: vi.fn() };
      }
      return { data: { items: [], totalCount: 0 }, loading: false, error: null, refetch: vi.fn() };
    });
    renderOperators();
    expect(screen.getByText('opendatahub-operator')).toBeInTheDocument();
  });
});
