import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useApi, useSSE } from './hooks';

describe('useApi', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches data and returns it', async () => {
    const mockData = { total: 10, resilient: 5 };
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockData),
    } as Response);

    const { result } = renderHook(() => useApi<typeof mockData>('/api/v1/overview/stats'));

    expect(result.current.loading).toBe(true);

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.data).toEqual(mockData);
    expect(result.current.error).toBeNull();
  });

  it('sets error on fetch failure', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ error: 'internal error' }),
    } as unknown as Response);

    const { result } = renderHook(() => useApi('/api/v1/overview/stats'));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.data).toBeNull();
    expect(result.current.error).toBe('internal error');
  });

  it('does not fetch when url is null', () => {
    vi.spyOn(globalThis, 'fetch');

    renderHook(() => useApi(null));

    expect(globalThis.fetch).not.toHaveBeenCalled();
  });
});

describe('useSSE', () => {
  beforeEach(() => {
    // Mock EventSource for SSE tests
    class MockEventSource {
      url: string;
      onopen: (() => void) | null = null;
      onmessage: ((event: MessageEvent) => void) | null = null;
      onerror: (() => void) | null = null;

      constructor(url: string) {
        this.url = url;
      }

      close() {
        // Mock close
      }
    }

    globalThis.EventSource = MockEventSource as unknown as typeof EventSource;
  });

  it('returns empty events initially', () => {
    const { result } = renderHook(() => useSSE('/api/v1/experiments/live'));
    expect(result.current.events).toEqual([]);
    expect(result.current.connected).toBe(false);
  });

  it('accepts null URL and does nothing', () => {
    const { result } = renderHook(() => useSSE(null));
    expect(result.current.events).toEqual([]);
    expect(result.current.connected).toBe(false);
  });
});
