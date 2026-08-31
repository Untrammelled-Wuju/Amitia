# AMITIA Release Gate Report

Status: **NOT GENERATED FOR THIS CHECKOUT**

This file is generated only after a successful final Windows release build:

```bash
pnpm --dir desktop dist:win
```

The generator is `desktop/scripts/generate-release-report.mjs`. A successful release writes the same report to `release-reports/<timestamp>/release-gate-report.md` and replaces this file. A report must never claim PASS unless `desktop/release/.desktop-pet-release-gate.json` was created and verified for the same frozen source and packaged artifacts.
