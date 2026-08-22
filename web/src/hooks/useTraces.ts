import { useCallback, useEffect, useState } from 'react';
import { api } from '../api/client';
import type { ToolCallRecord, TraceEntry } from '../api/types';

export interface UseTracesState {
  traces: TraceEntry[];
  tools: ToolCallRecord[];
  loading: boolean;
  error: Error | null;
  refresh: () => void;
}

export function useTraces(sessionId: string | undefined): UseTracesState {
  const [traces, setTraces] = useState<TraceEntry[]>([]);
  const [tools, setTools] = useState<ToolCallRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const [reloadToken, setReloadToken] = useState(0);

  useEffect(() => {
    if (!sessionId) {
      setTraces([]);
      setTools([]);
      setLoading(false);
      setError(null);
      return;
    }

    let cancelled = false;
    setLoading(true);
    setError(null);

    Promise.all([api.sessions.traces(sessionId), api.sessions.tools(sessionId)])
      .then(([traceRes, toolRes]) => {
        if (cancelled) return;
        setTraces(traceRes.traces);
        setTools(toolRes.tools);
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
  }, [sessionId, reloadToken]);

  const refresh = useCallback(() => setReloadToken((t) => t + 1), []);

  return { traces, tools, loading, error, refresh };
}
