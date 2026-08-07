# PRoot Android ARM64 Build

This directory contains scripts and configuration to build PRoot for Android ARM64.

## Fixed Parameters

- **Upstream:** proot-me/proot v5.4.0
- **Patch source:** termux/proot (Android compatibility)
- **Target ABI:** arm64-v8a
- **Architecture:** aarch64
- **Output filename:** libamitia_proot.so

## Prerequisites

- Android NDK (with LLVM/Clang toolchain)
- Python 3.8+
- Git
- Internet access (first run only)

## Usage

### 1. Update Lock File (first run)
```bash
python update_lock.py
```

### 2. Build
```bash
python build.py --clean
```

### 3. Verify
```bash
python verify.py
```

### 4. Install to Android Module
```bash
python build.py --install-android-module
```

### 5. Test Scripts
```bash
python -m unittest test_build.py
```

## Output

Deliverables go to `runtime/out/proot/android-arm64/`:
- `libamitia_proot.so` - The built PRoot ELF
- `proot-artifact.json` - Metadata
- `SHA256SUMS` - Verification checksums
- `source-manifest.json` - Traceability

## License

PRoot is licensed under GPL-2.0-or-later.
The license text is preserved in `licenses/COPYING-PROOT`.
