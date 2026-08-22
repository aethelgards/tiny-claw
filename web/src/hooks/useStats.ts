import { useCallback, useEffect, useState } from 'react';
import { api } from '../api/client';
import type { DailyStat, ModelStat, OverviewStats, ToolStat } from '../api/types';

export interface UseStatsState {
  overview: OverviewStats | null;
  daily: DailyStat[];
  models: ModelStat[];
  tools: ToolStat[];
  loading: boolean;
  error: Error | null;
  refresh: () => void;
}

export function useStats(): UseStatsState {
  const [overview, setOverview] = useState<OverviewStats | null>(null);
  const [daily, setDaily] = useState<DailyStat[]>([]);
  const [models, setModels] = useState<ModelStat[]>([]);
  const [tools, setTools] = useState<ToolStat[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [reloadToken, setReloadToken] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);

    Promise.all([api.stats.overview(), api.stats.daily(), api.stats.models(), api.stats.tools()])
      .then(([overviewRes, dailyRes, modelRes, toolRes]) => {
        if (cancelled) return;
        setOverview(overviewRes);
        setDaily(dailyRes.daily);
        setModels(modelRes.models);
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
  }, [reloadToken]);

  const refresh = useCallback(() => setReloadToken((t) => t + 1), []);

  return { overview, daily, models, tools, loading, error, refresh };
}
