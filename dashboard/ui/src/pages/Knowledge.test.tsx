import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Knowledge } from './Knowledge';
import { vi } from 'vitest';

vi.mock('../api/hooks', () => ({
  useApi: vi.fn(() => ({ data: null, loading: false, error: null, refetch: vi.fn() })),
}));

import { useApi } from '../api/hooks';

function renderKnowledge() {
  return render(<MemoryRouter><Knowledge /></MemoryRouter>);
}

describe('Knowledge', () => {
  it('shows loading state', () => {
    (useApi as ReturnType<typeof vi.fn>).mockReturnValue({
      data: null, loading: true, error: null, refetch: vi.fn(),
    });
    renderKnowledge();
    expect(screen.getByLabelText(/loading/i)).toBeInTheDocument();
  });

  it('renders operator selector', () => {
    (useApi as ReturnType<typeof vi.fn>).mockImplementation((url: string) => {
      if (url && url.includes('/operators') && !url.includes('/components')) {
        return { data: ['opendatahub-operator'], loading: false, error: null, refetch: vi.fn() };
      }
      return { data: null, loading: false, error: null, refetch: vi.fn() };
    });
    renderKnowledge();
    expect(screen.getAllByText(/operator/i).length).toBeGreaterThan(0);
  });

  it('renders graph area when component data loaded', () => {
    (useApi as ReturnType<typeof vi.fn>).mockImplementation((url: string) => {
      if (url && url.includes('/operators') && !url.includes('/components')) {
        return { data: ['op1'], loading: false, error: null, refetch: vi.fn() };
      }
      if (url && url.includes('/components')) {
        return { data: ['comp1'], loading: false, error: null, refetch: vi.fn() };
      }
      if (url && url.includes('/knowledge')) {
        return {
          data: { name: 'comp1', controller: 'ctrl', managedResources: [
            { apiVersion: 'apps/v1', kind: 'Deployment', name: 'deploy1' },
            { apiVersion: 'v1', kind: 'ConfigMap', name: 'cm1' },
          ]},
          loading: false, error: null, refetch: vi.fn(),
        };
      }
      if (url && url.includes('/experiments')) {
        return { data: { items: [], totalCount: 0 }, loading: false, error: null, refetch: vi.fn() };
      }
      return { data: null, loading: false, error: null, refetch: vi.fn() };
    });
    renderKnowledge();
    expect(screen.getByText(/Dependency Graph/)).toBeInTheDocument();
  });

  it('shows prompt when no component selected', () => {
    (useApi as ReturnType<typeof vi.fn>).mockImplementation((url: string) => {
      if (url && url.includes('/operators') && !url.includes('/components')) {
        return { data: ['op1'], loading: false, error: null, refetch: vi.fn() };
      }
      return { data: null, loading: false, error: null, refetch: vi.fn() };
    });
    renderKnowledge();
    expect(screen.getByText(/Select an operator and component to view the dependency graph/i)).toBeInTheDocument();
  });
});
