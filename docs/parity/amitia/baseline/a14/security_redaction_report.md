# Security Redaction Report
# 安全消隐报告

## Generated: 2026-08-07
## Baseline Step: A14
## Project: Amitia

## Methodology

Scanned for files matching sensitive patterns:
- .env, .env.*, secrets.*, credentials.*, config.local.*
- *.pem, *.key, *.p12, *.keystore, *.pfx
- service-account.*, *credential*, *secret*, *token*

## Findings

### .env files
**Status: NO_SECRET_CANDIDATE_FOUND**
- No .env files found in working tree

### Certificate/Key files
**Status: NO_SECRET_CANDIDATE_FOUND**
- No *.pem, *.key, *.p12, *.keystore files found

### Configuration files with potential secrets
| Path | Git Tracked | Size | SHA-256 | Risk Status |
|------|-------------|------|---------|-------------|
| appsettings.json | No | 981 | 4825719a615548497c592d2d0ea4d02436bf41fca019c4d38975f68059cfcd27 | TEMPLATE_ONLY |
| backend/appsettings.json | No | 981 | 4825719a615548497c592d2d0ea4d02436bf41fca019c4d38975f68059cfcd27 | TEMPLATE_ONLY |
| config/config.yml | Yes | 2630 | e12b314678b8b600d977ec2bbdcde1609d77d8970e793aeb7ca277b7561aca58 | LOCAL_CONFIG |

### Debug/fix scripts (potential security concern)
- Multiple ix_*.py and debug_*.py scripts found in repo root
- These appear to be development utilities, not secret-containing files
- Status: NO_SECRET_CANDIDATE_FOUND

## Conclusion

**No high-risk secret files detected.**

The project uses configuration appsettings.json files that are untracked (not in git), which is a safe practice. The config/config.yml file is tracked but contains application configuration rather than secrets.

No action required for B3 baseline. Any secret management improvements are outside B3 scope.
