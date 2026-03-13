# Changelog

## [Unreleased]

### Added
- `create_segment` — create a new targeting segment
- `update_segment` — update a targeting segment (GET-then-merge for partial updates)
- `delete_segment` — delete a targeting segment
- `get_segment_usage` — check which flags reference a segment
- `delete_flag` — permanently delete an archived flag
- `archive_flag` — archive or restore a feature flag
- `create_sdk_key` — create a new SDK key for an environment
- `list_sdk_keys` — list SDK keys for an environment
- `delete_sdk_key` — revoke an SDK key
- `get_audit_log` — retrieve project audit log entries
- `evaluate_flags` — evaluate flags via the playground with detailed traces

### Changed
- `update_flag_config` — added `enabled` and `variants` parameters for complete environment config management

### Removed
- `update_flag_config` — removed phantom `rollout_percentage` parameter (was silently ignored by backend)

## [1.0.0](https://github.com/joCur/togglerino/compare/mcp-v0.1.0...mcp-v1.0.0) (2026-03-12)


### ⚠ BREAKING CHANGES

* MCP server for flag management ([#124](https://github.com/joCur/togglerino/issues/124)) (#128)

### Features

* MCP server for flag management ([#124](https://github.com/joCur/togglerino/issues/124)) ([#128](https://github.com/joCur/togglerino/issues/128)) ([ea6d0e8](https://github.com/joCur/togglerino/commit/ea6d0e8ec43ab1e2eda6c55ad669cb7ce6b255ff))
