import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js'
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js'
import { z } from 'zod'
import { TogglerinoClient, TogglerinoError } from './client.js'
import { listProjects } from './tools/projects.js'
import { listFlags, getFlag, createFlag, updateFlag, toggleFlag, updateFlagConfig } from './tools/flags.js'
import { listEnvironments } from './tools/environments.js'
import { listSegments, getSegment, createSegment, updateSegment, deleteSegment, getSegmentUsage } from './tools/segments.js'

const pkg = JSON.parse(readFileSync(join(__dirname, '..', 'package.json'), 'utf-8')) as { version: string }
const version = pkg.version

const baseUrl = process.env.TOGGLERINO_URL
const apiKey = process.env.TOGGLERINO_API_KEY
const defaultProject = process.env.TOGGLERINO_PROJECT

if (!baseUrl) {
  process.stderr.write('Error: TOGGLERINO_URL environment variable is required\n')
  process.exit(1)
}

if (!apiKey) {
  process.stderr.write('Error: TOGGLERINO_API_KEY environment variable is required\n')
  process.exit(1)
}

const client = new TogglerinoClient(baseUrl, apiKey)

function resolveProject(projectKey?: string): string | null {
  return projectKey || defaultProject || null
}

function requireProject(projectKey?: string): string {
  const resolved = resolveProject(projectKey)
  if (!resolved) {
    throw new Error('No project key provided and TOGGLERINO_PROJECT is not set. Call list_projects to see available projects, then provide the projectKey parameter.')
  }
  return resolved
}

function ok(result: unknown) {
  return { content: [{ type: 'text' as const, text: JSON.stringify(result, null, 2) }] }
}

function err(e: unknown) {
  const msg = e instanceof TogglerinoError ? e.message : e instanceof Error ? e.message : 'Unexpected error'
  return { content: [{ type: 'text' as const, text: msg }], isError: true as const }
}

const server = new McpServer({ name: 'togglerino', version })

server.tool('list_projects', 'List all projects in the Togglerino instance', async () => {
  try {
    const result = await listProjects(client)
    return ok(result)
  } catch (e) {
    return err(e)
  }
})

server.tool(
  'list_flags',
  'List all feature flags in a project, optionally filtered by search term or tag',
  {
    projectKey: z.string().optional().describe('Project key (uses TOGGLERINO_PROJECT env var if not provided)'),
    search: z.string().optional().describe('Search term to filter flags by name or key'),
    tag: z.string().optional().describe('Filter flags by tag'),
  },
  async ({ projectKey, search, tag }) => {
    try {
      const project = requireProject(projectKey)
      const result = await listFlags(client, project, { search, tag })
      return ok(result)
    } catch (e) {
      return err(e)
    }
  },
)

server.tool(
  'get_flag',
  'Get details of a specific feature flag including all environment configurations',
  {
    projectKey: z.string().optional().describe('Project key (uses TOGGLERINO_PROJECT env var if not provided)'),
    flagKey: z.string().describe('The flag key to retrieve'),
  },
  async ({ projectKey, flagKey }) => {
    try {
      const project = requireProject(projectKey)
      const result = await getFlag(client, project, flagKey)
      return ok(result)
    } catch (e) {
      return err(e)
    }
  },
)

server.tool(
  'create_flag',
  'Create a new feature flag in a project',
  {
    projectKey: z.string().optional().describe('Project key (uses TOGGLERINO_PROJECT env var if not provided)'),
    name: z.string().describe('Display name for the flag'),
    key: z.string().describe('Unique key for the flag (used in SDK calls)'),
    flag_type: z.enum(['release', 'experiment', 'operational', 'kill-switch', 'permission']).describe('The type/purpose of the flag'),
    value_type: z.enum(['boolean', 'string', 'number', 'json']).describe('The type of value this flag returns'),
    default_value: z.string().describe('The default value for the flag'),
    description: z.string().optional().describe('Optional description of the flag'),
    tags: z.array(z.string()).optional().describe('Optional list of tags for organizing flags'),
    environment_overrides: z.record(z.string(), z.unknown()).optional().describe('Optional per-environment configuration overrides'),
  },
  async ({ projectKey, name, key, flag_type, value_type, default_value, description, tags, environment_overrides }) => {
    try {
      const project = requireProject(projectKey)
      const params: Record<string, unknown> = { name, key, flag_type, value_type, default_value }
      if (description !== undefined) params.description = description
      if (tags !== undefined) params.tags = tags
      if (environment_overrides !== undefined) params.environment_overrides = environment_overrides
      const result = await createFlag(client, project, params)
      return ok(result)
    } catch (e) {
      return err(e)
    }
  },
)

server.tool(
  'update_flag',
  'Update the metadata of a feature flag (name, description, tags)',
  {
    projectKey: z.string().optional().describe('Project key (uses TOGGLERINO_PROJECT env var if not provided)'),
    flagKey: z.string().describe('The flag key to update'),
    name: z.string().optional().describe('New display name for the flag'),
    description: z.string().optional().describe('New description for the flag'),
    tags: z.array(z.string()).optional().describe('New list of tags for the flag'),
  },
  async ({ projectKey, flagKey, name, description, tags }) => {
    try {
      const project = requireProject(projectKey)
      const params: Record<string, unknown> = {}
      if (name !== undefined) params.name = name
      if (description !== undefined) params.description = description
      if (tags !== undefined) params.tags = tags
      const result = await updateFlag(client, project, flagKey, params)
      return ok(result)
    } catch (e) {
      return err(e)
    }
  },
)

server.tool(
  'toggle_flag',
  'Enable or disable a feature flag in a specific environment',
  {
    projectKey: z.string().optional().describe('Project key (uses TOGGLERINO_PROJECT env var if not provided)'),
    flagKey: z.string().describe('The flag key to toggle'),
    environmentKey: z.string().describe('The environment key (e.g. production, staging, development)'),
    enabled: z.boolean().describe('Whether to enable (true) or disable (false) the flag'),
  },
  async ({ projectKey, flagKey, environmentKey, enabled }) => {
    try {
      const project = requireProject(projectKey)
      const result = await toggleFlag(client, project, flagKey, environmentKey, enabled)
      return ok(result)
    } catch (e) {
      return err(e)
    }
  },
)

server.tool(
  'update_flag_config',
  'Update the environment-specific configuration of a feature flag (targeting rules, rollout percentage, default variant)',
  {
    projectKey: z.string().optional().describe('Project key (uses TOGGLERINO_PROJECT env var if not provided)'),
    flagKey: z.string().describe('The flag key to update'),
    environmentKey: z.string().describe('The environment key (e.g. production, staging, development)'),
    default_variant: z.string().optional().describe('The default variant key to serve when no rules match'),
    targeting_rules: z.array(z.record(z.string(), z.unknown())).optional().describe('Targeting rules array for the flag'),
    rollout_percentage: z.number().min(0).max(100).optional().describe('Percentage of users to roll out to (0-100)'),
  },
  async ({ projectKey, flagKey, environmentKey, default_variant, targeting_rules, rollout_percentage }) => {
    try {
      const project = requireProject(projectKey)
      const updates: Record<string, unknown> = {}
      if (default_variant !== undefined) updates.default_variant = default_variant
      if (targeting_rules !== undefined) updates.targeting_rules = targeting_rules
      if (rollout_percentage !== undefined) updates.rollout_percentage = rollout_percentage
      const result = await updateFlagConfig(client, project, flagKey, environmentKey, updates)
      return ok(result)
    } catch (e) {
      return err(e)
    }
  },
)

server.tool(
  'list_environments',
  'List all environments in a project',
  {
    projectKey: z.string().optional().describe('Project key (uses TOGGLERINO_PROJECT env var if not provided)'),
  },
  async ({ projectKey }) => {
    try {
      const project = requireProject(projectKey)
      const result = await listEnvironments(client, project)
      return ok(result)
    } catch (e) {
      return err(e)
    }
  },
)

server.tool(
  'list_segments',
  'List all reusable targeting segments in a project',
  {
    projectKey: z.string().optional().describe('Project key (uses TOGGLERINO_PROJECT env var if not provided)'),
  },
  async ({ projectKey }) => {
    try {
      const project = requireProject(projectKey)
      const result = await listSegments(client, project)
      return ok(result)
    } catch (e) {
      return err(e)
    }
  },
)

server.tool(
  'get_segment',
  'Get details of a specific targeting segment including its conditions',
  {
    projectKey: z.string().optional().describe('Project key (uses TOGGLERINO_PROJECT env var if not provided)'),
    segmentKey: z.string().describe('The segment key to retrieve'),
  },
  async ({ projectKey, segmentKey }) => {
    try {
      const project = requireProject(projectKey)
      const result = await getSegment(client, project, segmentKey)
      return ok(result)
    } catch (e) {
      return err(e)
    }
  },
)

server.tool(
  'create_segment',
  'Create a new reusable targeting segment in a project. Conditions use: attribute, operator (equals, not_equals, contains, not_contains, starts_with, ends_with, greater_than, less_than, gte, lte, in, not_in, exists, not_exists, matches, segment_match), and value.',
  {
    projectKey: z.string().optional().describe('Project key (uses TOGGLERINO_PROJECT env var if not provided)'),
    key: z.string().describe('Unique key for the segment (lowercase alphanumeric + hyphens, 3-64 chars)'),
    name: z.string().describe('Display name for the segment'),
    description: z.string().optional().describe('Optional description of the segment'),
    conditions: z.string().describe('JSON array of conditions, e.g. [{"attribute":"plan","operator":"equals","value":"enterprise"}]'),
  },
  async ({ projectKey, key, name, description, conditions }) => {
    try {
      const project = requireProject(projectKey)
      const parsed = JSON.parse(conditions)
      const params: Record<string, unknown> = { key, name, conditions: parsed }
      if (description !== undefined) params.description = description
      const result = await createSegment(client, project, params as Parameters<typeof createSegment>[2])
      return ok(result)
    } catch (e) {
      return err(e)
    }
  },
)

server.tool(
  'update_segment',
  'Update a targeting segment. Uses GET-then-merge so you only need to provide fields you want to change.',
  {
    projectKey: z.string().optional().describe('Project key (uses TOGGLERINO_PROJECT env var if not provided)'),
    segmentKey: z.string().describe('The segment key to update'),
    name: z.string().optional().describe('New display name for the segment'),
    description: z.string().optional().describe('New description for the segment'),
    conditions: z.string().optional().describe('JSON array of conditions to replace existing conditions'),
  },
  async ({ projectKey, segmentKey, name, description, conditions }) => {
    try {
      const project = requireProject(projectKey)
      const updates: Record<string, unknown> = {}
      if (name !== undefined) updates.name = name
      if (description !== undefined) updates.description = description
      if (conditions !== undefined) updates.conditions = JSON.parse(conditions)
      const result = await updateSegment(client, project, segmentKey, updates)
      return ok(result)
    } catch (e) {
      return err(e)
    }
  },
)

server.tool(
  'delete_segment',
  'Delete a targeting segment. Fails with 409 if the segment is referenced by active flags — use get_segment_usage to check first.',
  {
    projectKey: z.string().optional().describe('Project key (uses TOGGLERINO_PROJECT env var if not provided)'),
    segmentKey: z.string().describe('The segment key to delete'),
  },
  async ({ projectKey, segmentKey }) => {
    try {
      const project = requireProject(projectKey)
      await deleteSegment(client, project, segmentKey)
      return ok({ message: `Segment '${segmentKey}' deleted successfully` })
    } catch (e) {
      return err(e)
    }
  },
)

server.tool(
  'get_segment_usage',
  'Get which flags reference a specific segment — useful before deleting a segment',
  {
    projectKey: z.string().optional().describe('Project key (uses TOGGLERINO_PROJECT env var if not provided)'),
    segmentKey: z.string().describe('The segment key to check usage for'),
  },
  async ({ projectKey, segmentKey }) => {
    try {
      const project = requireProject(projectKey)
      const result = await getSegmentUsage(client, project, segmentKey)
      return ok(result)
    } catch (e) {
      return err(e)
    }
  },
)

;(async () => {
  const transport = new StdioServerTransport()
  await server.connect(transport)
})()
