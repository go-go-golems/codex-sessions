---
Title: 'Spec: Plugin System'
Ticket: CODEX-006-PLUGIN-SYSTEM
Status: active
Topics:
    - backend
    - chat
    - go
    - plugins
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-25T12:41:58.112493509-05:00
WhatFor: ""
WhenToUse: ""
---

# Spec: Plugin System

## Executive Summary

Add a plugin system to `codex-sessions` so parts of extraction can be customized. One immediate use-case is extracting project names from `session_meta` (instead of relying solely on `cwd` basename).

## Problem Statement

Project naming and other extraction heuristics may vary by environment. A plugin system would allow users to supply custom logic without forking the tool.

## Proposed Solution

Implement a plugin system (details to be specified in a follow-up design) and use it for project-name extraction from `session_meta`.

## Design Decisions

N/A (intentionally deferred).

## Alternatives Considered

N/A (intentionally deferred).

## Implementation Plan

- [ ] Define plugin system approach
- [ ] Add first plugin hook: project name extraction from session_meta

## Open Questions

N/A.

## References

N/A.
