import { useCallback, useEffect, useState } from 'react';
import { api } from '../api/client';
import type { SessionData, SessionListParams } from '../api/types';

export interface UseSessionsState {
  sessions: SessionData[];
  total: number;
  page: number;
  limit: number;
  status?: string;
  search: string;
  loading: boolean;
  error: Error | null;
  setPage: (page: number) => void;
  setLimit: (limit: number) => void;
  setStatus: (status?: string) => void;
  setSearch: (search: string) => void;
  refresh: () => void;
}

export function useSessions(initialParams?: SessionListParams): UseSessionsState {
  const [page, setPage] = useState(initialParams?.page ?? 1);
  const [limit, setLimit] = useState(initialParams?.limit ?? 20);
  const [status, setStatus] = useState<string | undefined>(initialParams?.status);
  const [search, setSearch] = useState(initialParams?.search ?? '');
  const [reloadToken, setReloadToken] = useState(0);

  const [sessions, setSessions] = useState<SessionData[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);

    api.sessions
      .list({ page, limit, status, search: search || undefined })
      .then((res) => {
        if (cancelled) return;
        setSessions(res.sessions);
        setTotal(res.total);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(err instanceof Error ? err : new Error(String(err)));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [page, limit, status, search, reloadToken]);

  const refresh = useCallback(() => setReloadToken((t) => t + 1), []);

  return {
    sessions,
    total,
    page,
    limit,
    status,
    search,
    loading,
    error,
    setPage,
    setLimit,
    setStatus,
    setSearch,
    refresh,
  };
}
