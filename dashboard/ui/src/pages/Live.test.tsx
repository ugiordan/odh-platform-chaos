import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Live } from './Live';
import { vi } from 'vitest';

vi.mock('../api/hooks', () => ({
  useSSE: vi.fn(() => ({ events: [], connected: false, error: null })),
  useApi: vi.fn(() => ({ data: null, loading: true, error: null, refetch: vi.fn() })),
}));

import { useSSE } from '../api/hooks';

function renderLive() {
  return render(<MemoryRouter><Live /></MemoryRouter>);
}

describe('Live', () => {
  it('shows empty state when no experiments running', () => {
    (useSSE as ReturnType<typeof vi.fn>).mockReturnValue({
      events: [], connected: true, error: null,
    });
    renderLive();
    expect(screen.getByText(/no experiments/i)).toBeInTheDocument();
  });

  it('renders experiment card with name and phase', () => {
    (useSSE as ReturnType<typeof vi.fn>).mockReturnValue({
      events: [{
        id: '1', name: 'omc-podkill', namespace: 'test', operator: 'op',
        component: 'comp', type: 'PodKill', phase: 'Injecting',
        specJson: '{}', statusJson: '{}', createdAt: '', updatedAt: '',
      }],
      connected: true, error: null,
    });
    renderLive();
    expect(screen.getByText('omc-podkill')).toBeInTheDocument();
  });

  it('shows reconnection banner on error', () => {
    (useSSE as ReturnType<typeof vi.fn>).mockReturnValue({
      events: [], connected: false, error: 'Connection lost',
    });
    renderLive();
    expect(screen.getByText(/connection lost/i)).toBeInTheDocument();
  });

  it('shows connected indicator when streaming', () => {
    (useSSE as ReturnType<typeof vi.fn>).mockReturnValue({
      events: [], connected: true, error: null,
    });
    renderLive();
    // The live dot should be present (not have disconnected class)
    expect(document.querySelector('.live-dot')).toBeInTheDocument();
  });
});
