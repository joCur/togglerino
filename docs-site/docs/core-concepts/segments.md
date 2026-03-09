---
sidebar_position: 5
title: Segments
---

# Segments

Segments are reusable, named groups of targeting conditions scoped to a project. Instead of repeating the same set of conditions across many flags, you define them once as a segment and reference that segment in your targeting rules.

## Why Use Segments

Suppose you have a "Beta Users" audience defined by two conditions: `plan` equals `"beta"` AND `signup_date` greater_than `"2025-01-01"`. Without segments, you'd copy these conditions into every flag that targets beta users. If the definition changes (say you update the signup date threshold), you'd need to update every flag individually.

With a segment, you define "Beta Users" once and reference it everywhere. Update the segment, and all flags using it pick up the change automatically.

## Segment Fields

Each segment has:

| Field | Description |
|-------|-------------|
| **Key** | Unique within the project. URL-safe slug (e.g., `beta-users`). |
| **Name** | Human-readable label (e.g., "Beta Users"). |
| **Description** | Optional text explaining who the segment represents. |
| **Conditions** | A list of conditions in the same format as targeting rule conditions (attribute, operator, value). All conditions must match (AND logic). |

## Using Segments in Targeting Rules

To use a segment in a targeting rule, add a condition with:

- **Operator:** `segment_match`
- **Value:** the segment's key (e.g., `beta-users`)

When the flag is evaluated, the engine looks up the segment by key and evaluates its conditions against the evaluation context. If all of the segment's conditions match, the `segment_match` condition passes.

### Example

**Segment:** "Beta Users"
- `plan` equals `"beta"`
- `signup_date` greater_than `"2025-01-01"`

**Flag targeting rule:**

| Conditions | Variant | Rollout |
|-----------|---------|---------|
| `segment_match` = `beta-users` | `enabled` | 100% |

Any user matching the Beta Users segment receives the `enabled` variant.

You can combine `segment_match` with other conditions in the same rule. For example: `segment_match` = `beta-users` AND `country` equals `"US"` would target only US-based beta users.

## Key Constraints

### No Nesting

Segments cannot reference other segments. A segment's conditions can only use the standard operators (`equals`, `contains`, `in`, etc.) — not `segment_match`. This is enforced at write time to prevent circular references and keep evaluation predictable.

### Deletion Protection

A segment cannot be deleted if any flag references it in a targeting rule. Attempting to delete a referenced segment returns a **409 Conflict** error. To delete a segment:

1. Check which flags reference it using the segment usage endpoint.
2. Remove or replace the `segment_match` conditions in those flags.
3. Delete the segment.

## Managing Segments

You can create, edit, and delete segments from the dashboard under **Project > Segments**. See the [Dashboard Guide](/dashboard/managing-flags) for a walkthrough.
