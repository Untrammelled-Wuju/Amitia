# Package Integrity 算法说明

## 算法标识

```
amitia-package-sha256-v2
```

## 核心原则

manifest.json 不列入自身的 Integrity Files 列表，避免递归 Hash。

## Payload Files

`integrity.files` 只列出 Payload 文件：

- Action JSON (`actions/{key}/action.json`)
- Frame 文件 (`actions/{key}/frames/{file}`)
- Preview 文件
- Notice 文件
- Metadata 文件
- 其他声明资源

不列 `manifest.json` 自身。

## Canonical Manifest Hash

### 构建步骤

1. 深复制 Manifest 副本
2. 将副本的 `integrity.manifestHash` 和 `integrity.contentRootHash` 置为空字符串
3. 其他字段和 Files 列表完整保留
4. 使用 Canonical JSON 序列化：
   - UTF-8 编码
   - Object Key 按字典序排序
   - 无多余空白
   - 稳定数字表示
   - 数组顺序保持
5. 计算 `manifestHash = SHA256(canonicalManifestBytes)`

## Content Root Hash

### 构建步骤

1. 收集所有 Payload File entries（已排序）
2. 追加伪条目：
   - `path = "@manifest"`
   - `sha256 = manifestHash`
   - `bytes = canonicalManifestBytes.length`
3. 对所有条目（含伪条目）计算 Tree Hash

### Tree Hash 算法

```
对每个 entry（按 path 字典序排序）：
  写入 "file" + NUL
  写入 entry.path + NUL
  写入 entry.bytes（十进制字符串）+ NUL
  写入 entry.sha256（hex 解码为 raw bytes）+ NUL
最终 SHA256(result)
```

## 写入流程

```
1. 写 Action JSON
2. 写 Frame
3. 写 Preview 和其他资源
4. 建立 Payload File Manifest
5. 构建完整 Manifest（manifestHash 和 contentRootHash 为空）
6. 计算 Canonical Manifest Hash
7. 计算 Content Root Hash（含 @manifest 伪条目）
8. 填入 manifestHash 和 contentRootHash
9. 写最终 manifest.json
10. 对最终目录执行 Strict Validate
11. 生成 Archive
12. 对 Archive 再执行 Strict Validate
```

## 验证规则

### 必须严格要求

```
algorithm 非空且受支持
manifestHash 非空
contentRootHash 非空
fileCount == len(files)
totalBytes == sum(files.bytes)
files 至少包含所有 Action JSON 和 Frame
SHA256 格式合法（64 位 hex）
Path 唯一
Path 大小写唯一
```

### manifest.json 特殊范围

验证最终目录时允许 `manifest.json` 不在 `files` 列表中，但只允许这一个特殊文件。

其他任何未声明文件：

```
PACKAGE_FILE_UNDECLARED
```

## Legacy 兼容

旧算法 `amitia-tree-sha256-v1` 的 Schema 2 包标记 `legacy_v2_integrity`，允许迁移期读取，首次升级或重建时写新 Integrity。

禁止继续创建旧算法 Package。
