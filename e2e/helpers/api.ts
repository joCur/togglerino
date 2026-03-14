import type { APIRequestContext } from '@playwright/test';

/** Response types matching the backend JSON shapes. */
export interface Project {
  id: string;
  key: string;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
}

export interface Flag {
  id: string;
  key: string;
  name: string;
  description: string;
  value_type: 'boolean' | 'string' | 'number' | 'json';
  flag_type: 'release' | 'experiment' | 'operational' | 'kill-switch' | 'permission';
  default_value: unknown;
  tags: string[];
  lifecycle_status: 'active' | 'potentially_stale' | 'stale' | 'archived';
}

export interface Environment {
  id: string;
  key: string;
  name: string;
  sort_order: number;
  created_at: string;
}

export interface FlagEnvironmentConfig {
  enabled: boolean;
  [key: string]: unknown;
}

/** Typed API helper wrapping Playwright's request context. */
export class ApiHelper {
  constructor(private request: APIRequestContext) {}

  async setup(email: string, password: string) {
    const res = await this.request.post('/api/v1/auth/setup', {
      data: { email, password },
    });
    return { status: res.status(), data: res.ok() ? await res.json() : null };
  }

  async login(email: string, password: string) {
    const res = await this.request.post('/api/v1/auth/login', {
      data: { email, password },
    });
    return { status: res.status(), data: res.ok() ? await res.json() : null };
  }

  async authStatus() {
    const res = await this.request.get('/api/v1/auth/status');
    return (await res.json()) as { setup_required: boolean };
  }

  async me() {
    const res = await this.request.get('/api/v1/auth/me');
    return { status: res.status(), data: res.ok() ? await res.json() : null };
  }

  async createProject(data: { key: string; name: string; description?: string }): Promise<Project> {
    const res = await this.request.post('/api/v1/projects', { data });
    if (!res.ok()) throw new Error(`createProject failed: ${res.status()} ${await res.text()}`);
    return (await res.json()) as Project;
  }

  async deleteProject(key: string) {
    const res = await this.request.delete(`/api/v1/projects/${key}`);
    if (!res.ok()) throw new Error(`deleteProject failed: ${res.status()} ${await res.text()}`);
  }

  async listProjects(): Promise<{ data: Project[]; total: number; limit: number; offset: number }> {
    const res = await this.request.get('/api/v1/projects');
    return (await res.json()) as { data: Project[]; total: number; limit: number; offset: number };
  }

  async listEnvironments(projectKey: string): Promise<Environment[]> {
    const res = await this.request.get(`/api/v1/projects/${projectKey}/environments`);
    return (await res.json()) as Environment[];
  }

  async createFlag(projectKey: string, data: {
    key: string;
    name: string;
    value_type?: string;
    flag_type?: string;
    default_value?: unknown;
    description?: string;
    tags?: string[];
  }): Promise<Flag> {
    const res = await this.request.post(`/api/v1/projects/${projectKey}/flags`, { data });
    if (!res.ok()) throw new Error(`createFlag failed: ${res.status()} ${await res.text()}`);
    return (await res.json()) as Flag;
  }

  async getFlag(projectKey: string, flagKey: string): Promise<{ flag: Flag; environment_configs: FlagEnvironmentConfig[] }> {
    const res = await this.request.get(`/api/v1/projects/${projectKey}/flags/${flagKey}`);
    if (!res.ok()) throw new Error(`getFlag failed: ${res.status()} ${await res.text()}`);
    return (await res.json()) as { flag: Flag; environment_configs: FlagEnvironmentConfig[] };
  }

  /**
   * Update a flag's environment config. Fetches the current config first and merges
   * to avoid zeroing out fields the backend expects (default_variant, variants, targeting_rules).
   */
  async updateFlagEnvConfig(
    projectKey: string,
    flagKey: string,
    envKey: string,
    config: { enabled: boolean },
  ) {
    // Fetch current config to preserve existing values
    const { environment_configs } = await this.getFlag(projectKey, flagKey);
    const current = environment_configs.find((c: any) => c.environment_key === envKey || c.env_key === envKey);

    const res = await this.request.put(
      `/api/v1/projects/${projectKey}/flags/${flagKey}/environments/${envKey}`,
      {
        data: {
          enabled: config.enabled,
          default_variant: current?.default_variant ?? '',
          variants: current?.variants ?? [],
          targeting_rules: current?.targeting_rules ?? [],
        },
      },
    );
    if (!res.ok()) throw new Error(`updateFlagEnvConfig failed: ${res.status()} ${await res.text()}`);
    return (await res.json()) as FlagEnvironmentConfig;
  }

  async archiveFlag(projectKey: string, flagKey: string, archived: boolean) {
    const res = await this.request.put(
      `/api/v1/projects/${projectKey}/flags/${flagKey}/archive`,
      { data: { archived } },
    );
    if (!res.ok()) throw new Error(`archiveFlag failed: ${res.status()} ${await res.text()}`);
    return (await res.json()) as Flag;
  }

  // --- SDK Key Management ---

  async createSDKKey(projectKey: string, envKey: string, name: string): Promise<SDKKey> {
    const res = await this.request.post(
      `/api/v1/projects/${projectKey}/environments/${envKey}/sdk-keys`,
      { data: { name } },
    );
    if (!res.ok()) throw new Error(`createSDKKey failed: ${res.status()} ${await res.text()}`);
    return (await res.json()) as SDKKey;
  }

  async listSDKKeys(projectKey: string, envKey: string): Promise<SDKKey[]> {
    const res = await this.request.get(
      `/api/v1/projects/${projectKey}/environments/${envKey}/sdk-keys`,
    );
    if (!res.ok()) throw new Error(`listSDKKeys failed: ${res.status()} ${await res.text()}`);
    return (await res.json()) as SDKKey[];
  }

  async deleteSDKKey(projectKey: string, envKey: string, id: string) {
    const res = await this.request.delete(
      `/api/v1/projects/${projectKey}/environments/${envKey}/sdk-keys/${id}`,
    );
    if (!res.ok()) throw new Error(`deleteSDKKey failed: ${res.status()} ${await res.text()}`);
  }
}

// --- SDK Evaluation (uses SDK key auth, not session auth) ---

export interface EvaluationResult {
  value: unknown;
  variant: string;
  reason: string;
}

export interface SDKKey {
  id: string;
  key: string;
  environment_id: string;
  name: string;
  revoked: boolean;
  created_at: string;
  project_key: string;
  environment_key: string;
}

/**
 * SDK client helper — uses Bearer token auth (SDK key) instead of session cookies.
 * Mirrors what a real SDK client would do.
 */
export class SDKClient {
  constructor(
    private baseURL: string,
    private sdkKey: string,
  ) {}

  private headers() {
    return { Authorization: `Bearer ${this.sdkKey}` };
  }

  async evaluateAll(context?: { user_id?: string; attributes?: Record<string, unknown> }): Promise<Record<string, EvaluationResult>> {
    const res = await fetch(`${this.baseURL}/api/v1/evaluate`, {
      method: 'POST',
      headers: { ...this.headers(), 'Content-Type': 'application/json' },
      body: JSON.stringify(context ? { context } : {}),
    });
    if (!res.ok) throw new Error(`evaluateAll failed: ${res.status} ${await res.text()}`);
    const body = await res.json();
    return body.flags as Record<string, EvaluationResult>;
  }

  async evaluateFlag(flagKey: string, context?: { user_id?: string; attributes?: Record<string, unknown> }): Promise<EvaluationResult> {
    const res = await fetch(`${this.baseURL}/api/v1/evaluate/${flagKey}`, {
      method: 'POST',
      headers: { ...this.headers(), 'Content-Type': 'application/json' },
      body: JSON.stringify(context ? { context } : {}),
    });
    if (!res.ok) throw new Error(`evaluateFlag failed: ${res.status} ${await res.text()}`);
    return (await res.json()) as EvaluationResult;
  }

  async evaluateFlagRaw(flagKey: string): Promise<{ status: number; body: any }> {
    const res = await fetch(`${this.baseURL}/api/v1/evaluate/${flagKey}`, {
      method: 'POST',
      headers: { ...this.headers(), 'Content-Type': 'application/json' },
      body: '{}',
    });
    return { status: res.status, body: await res.json() };
  }
}
