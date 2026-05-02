---
tags: [migration, naming, camelCase, audit, active]
domain: diet-tracker
status: active
created: 2026-05-02
updated: 2026-05-02
---

# Snake Case to CamelCase Migration

## Objective
Replace all non-DB snake_case identifiers with camelCase per user instruction and Diet_Tracker Current Truth (camelCase for JSON/API/Frontend, snake_case for DB schema).

## Preserve (No Replacement)
- All snake_case in .sql files (migrations, schema)
- All snake_case in internal/store/sqlite.go (DB queries)
- Legacy JSON tags for backwards compatibility (e.g., json:"duration_raw")
- String enum values mapping to DB entries (e.g., "lightly_active")

## Replace (snake_case → camelCase)
- Frontend Svelte files: All legacy snake_case API field references (duration_min → durationMin, load_lbs → loadLbs, avg_hr → avgHr, etc.)
- Go comments referencing legacy snake_case fields
- All non-DB snake_case identifiers in codebase

## Execution
Delegated to Implementation Specialist; Quality Auditor to verify no breaking changes.
