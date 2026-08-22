import type {
  DailyStatsResponse,
  ModelStatsResponse,
  OverviewStats,
  SessionData,
  SessionListParams,
  SessionListResponse,
  ToolCallListResponse,
  ToolStatsResponse,
  TraceListResponse,
} from './types';

const API_BASE = '/api';

async function fetchJSON<T>(url: string): Promise<T> {
  const response = await fetch(`${API_BASE}${url}`);
  if (!response.ok) throw new Error(`API error: ${response.status}`);
  return response.json();
}

export const api = {
  sessions: {
    list: (params?: SessionListParams): Promise<SessionListResponse> => {
      const query = new URLSearchParams();
      if (params?.page) query.set('page', String(params.page));
      if (params?.limit) query.set('limit', String(params.limit));
      if (params?.status) query.set('status', params.status);
      if (params?.search) query.set('search', params.search);
      return fetchJSON<SessionListResponse>(`/sessions?${query}`);
    },
    get: (id: string): Promise<SessionData> => fetchJSON<SessionData>(`/sessions/${id}`),
    traces: (id: string): Promise<TraceListResponse> => fetchJSON<TraceListResponse>(`/sessions/${id}/traces`),
    tools: (id: string): Promise<ToolCallListResponse> => fetchJSON<ToolCallListResponse>(`/sessions/${id}/tools`),
  },
  stats: {
    overview: (): Promise<OverviewStats> => fetchJSON<OverviewStats>('/stats/overview'),
    daily: (): Promise<DailyStatsResponse> => fetchJSON<DailyStatsResponse>('/stats/daily'),
    models: (): Promise<ModelStatsResponse> => fetchJSON<ModelStatsResponse>('/stats/models'),
    tools: (): Promise<ToolStatsResponse> => fetchJSON<ToolStatsResponse>('/stats/tools'),
  },
};
