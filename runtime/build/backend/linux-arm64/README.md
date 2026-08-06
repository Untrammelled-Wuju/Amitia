# Amitia Backend Linux ARM64 构建

此目录包含用于构建 Amitia Go 后端 Linux ARM64 产物的脚本和工具。

## 目标平台

- **GOOS**: `linux`
- **GOARCH**: `arm64`
- **GOARM64**: `v8.0`
- **CGO_ENABLED**: `0`

## 输出结构

```text
runtime/out/backend/linux-arm64/
├── backend/
│   └── amitia-server
├── manifest/
│   ├── backend-artifact.json
│   ├── build-inputs.json
│   ├── dependency-manifest.json
│   └── go-version-metadata.txt
├── SHA256SUMS
└── amitia-backend-<version>-linux-arm64.tar.xz
```

## 使用方法

### 正式离线构建

```bash
python build.py --version 1.0.0 --commit <40-char-commit> --offline --clean
```

### 开发构建

```bash
python build.py --development --clean
```

### 仅下载模块

```bash
python build.py --download
```

### 静态验证

```bash
python verify.py --mode static --artifact runtime/out/backend/linux-arm64
```

### ELF 检查

```bash
python inspect_elf.py runtime/out/backend/linux-arm64/backend/amitia-server
```

## 测试

```bash
# Python 单元测试
python -m unittest test_build.py test_verify.py test_archive.py

# Go 测试
cd backend && go test ./internal/buildinfo ./cmd/server
```

## 禁止事项

- 禁止 `CGO_ENABLED=1`
- 禁止 `GOOS=android`
- 禁止 `gomobile`
- 禁止 `buildmode=c-shared` / `buildmode=c-archive`
- 禁止注入 `BuildTime` / `BuildDate` / `CompiledAt`
- 禁止 `UPX` 压缩
- 禁止修改 `go.mod` / `go.sum`
- 禁止 `go.work` workspace 模式
