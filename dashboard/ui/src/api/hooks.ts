import { useState, useEffect, useCallback, useRef } from 'react';
import { apiFetch } from './client';

interface ApiState<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function useApi<T>(url: string | null): ApiState<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(url !== null);
  const [error, setError] = useState<string | null>(null);
  const mountedRef = useRef(true);

  const fetchData = useCallback(async () => {
    if (url === null) return;
    setLoading(true);
    setError(null);
    try {
      const result = await apiFetch<T>(url);
      if (mountedRef.current) {
        setData(result);
        setLoading(false);
      }
    } catch (err) {
      if (mountedRef.current) {
        setError(err instanceof Error ? err.message : 'Unknown error');
        setData(null);
        setLoading(false);
      }
    }
  }, [url]);

  useEffect(() => {
    mountedRef.current = true;
    fetchData();
    return () => { mountedRef.current = false; };
  }, [fetchData]);

  return { data, loading, error, refetch: fetchData };
}

interface SSEState<T> {
  events: T[];
  connected: boolean;
  error: string | null;
}

export function useSSE<T extends { id: string }>(url: string | null): SSEState<T> {
  const [events, setEvents] = useState<T[]>([]);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectDelayRef = useRef(1000); // Start at 1s
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    if (url === null) {
      setEvents([]);
      setConnected(false);
      setError(null);
      return;
    }

    const connect = () => {
      if (!mountedRef.current) return;

      const es = new EventSource(url);
      eventSourceRef.current = es;

      es.onopen = () => {
        if (!mountedRef.current) return;
        setConnected(true);
        setError(null);
        reconnectDelayRef.current = 1000; // Reset backoff on successful connection
      };

      es.onmessage = (event) => {
        if (!mountedRef.current) return;
        try {
          const data = JSON.parse(event.data) as T;
          setEvents((prev) => {
            const index = prev.findIndex((e) => e.id === data.id);
            let updated: T[];
            if (index !== -1) {
              updated = [...prev];
              updated[index] = data;
            } else {
              updated = [...prev, data];
            }
            // P1-2: Cap at 500 events
            if (updated.length > 500) {
              return updated.slice(-500);
            }
            return updated;
          });
        } catch (err) {
          setError(err instanceof Error ? err.message : 'Failed to parse event');
        }
      };

      es.onerror = () => {
        if (!mountedRef.current) return;
        setConnected(false);
        setError('SSE connection error');

        // P1-1: Close EventSource to prevent aggressive auto-retry
        es.close();
        eventSourceRef.current = null;

        // Schedule reconnection with exponential backoff
        const delay = reconnectDelayRef.current;
        reconnectTimeoutRef.current = setTimeout(() => {
          if (mountedRef.current) {
            connect();
          }
        }, delay);

        // Exponential backoff: 1s, 2s, 4s, 8s, 16s, max 30s
        reconnectDelayRef.current = Math.min(delay * 2, 30000);
      };
    };

    connect();

    return () => {
      mountedRef.current = false;
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }
    };
  }, [url]);

  return { events, connected, error };
}
