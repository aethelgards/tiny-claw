export interface SessionData {
  id: string;
  created_at: string;
  updated_at: string;
  prompt: string;
  model: string;
  status: 'running' | 'completed' | 'failed';
  turns: number;
  total_tokens: {
    prompt_tokens: number;
    completion_tokens: number;
    total_tokens: number;
  };
  total_cost: number;
  duration_ms: number;
  tags?: string[];
  summary?: string;
}

export interface TraceEntry {
  session_id: string;
  span_id: string;
  parent_id?: string;
  name: string;
  start_time: string;
  end_time: string;
  duration_ms: number;
  status: 'ok' | 'error';
  error?: string;
  attributes?: Record<string, unknown>;
}

/** A trace span nested with its child spans, as returned by the API. */
export interface SpanNode extends TraceEntry {
  children: SpanNode[];
}

export interface ToolCallRecord {
  session_id: string;
  span_id: string;
  tool_name: string;
  arguments: string;
  result: string;
  is_error: boolean;
  duration_ms: number;
  start_time: string;
}

export interface OverviewStats {
  total_sessions: number;
  total_cost: number;
  total_tokens: number;
  avg_duration_ms: number;
  success_rate: number;
}

export interface DailyStat {
  date: string;
  sessions: number;
  cost: number;
  tokens: number;
  avg_duration_ms: number;
  success_rate: number;
}

export interface ModelStat {
  model: string;
  sessions: number;
  cost: number;
  tokens: number;
}

export interface ToolStat {
  tool_name: string;
  calls: number;
  errors: number;
  avg_duration_ms: number;
}

export interface SessionListParams {
  page?: number;
  limit?: number;
  status?: string;
  search?: string;
}

export interface SessionListResponse {
  sessions: SessionData[];
  total: number;
  page: number;
  limit: number;
}

export interface TraceListResponse {
  session_id: string;
  traces: SpanNode[];
}

export interface ToolCallListResponse {
  tools: ToolCallRecord[];
}

export interface DailyStatsResponse {
  daily: DailyStat[];
}

export interface ModelStatsResponse {
  models: ModelStat[];
}

export interface ToolStatsResponse {
  tools: ToolStat[];
}
