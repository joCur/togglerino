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
    const current = environment_configs.find((c: any) => c.environment_key === envKey) || {};

    const res = await this.request.put(
      `/api/v1/projects/${projectKey}/flags/${flagKey}/environments/${envKey}`,
      {
        data: {
          enabled: config.enabled,
          default_variant: (current as any).default_variant || 'off',
          variants: (current as any).variants || [],
          targeting_rules: (current as any).targeting_rules || [],
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
}
