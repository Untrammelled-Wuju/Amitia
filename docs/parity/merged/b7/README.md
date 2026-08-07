# B7 Three-Source Capability Matrix Merge

## Overview

This directory contains the complete output of the B7 three-source capability matrix merge task.
The merge covers 837 capabilities from three sources:

- **Operit**: 365 capabilities (Android automation platform)
- **OpenMinis**: 145 capabilities (iOS/Android agent framework)
- **Amitia**: 327 capabilities (Desktop AI agent platform)

## Key Outputs

| File | Description |
|------|-------------|
| atomic_projection.json | Behavior signature projection for each capability |
| capability_mapping_groups.json | MAP grouping of related projections |
| capability_mapping_matrix.json | Full mapping matrix (JSON) |
| capability_mapping_matrix.md | Full mapping matrix (Markdown table) |
| capability_union_catalog.json | Unified capability catalog |
| operit_openminis_union.json | Union of Operit and OpenMinis |
| amitia_preservation_inventory.json | Amitia-only capabilities to preserve |
| target_candidate_inventory.json | Target candidate classification |
| behavior_conflicts.json | Detected behavior conflicts |
| source_projection_coverage.json | Projection coverage (all 100%) |
| B7_summary.json | Summary statistics |
| B7_三方能力矩阵合并报告.md | Full Chinese report |

## ID Convention

- MAP groups: MAP-0001, MAP-0002, ... (sequential)
- Operit projections: PROJ-OPR-XXXX-01
- OpenMinis projections: PROJ-OMN-XXXX-01
- Amitia projections: PROJ-AMT-XXXX-01

## Next Steps

- B8: Target capability validation
- B9: Implementation gap analysis

---

Generated: 2026-08-07 13:53:11

