# 第三方许可证

> 来源：Android 端依赖、Go 后端依赖、RootFS 组件
> 范围：所有第三方依赖的许可证清单
> 引用：每项依赖均给出 `file_path` 或包名
> change-id：`build-android-native-client`
> 生成时间：2026-07-26

---

## 1. Android 依赖许可证清单

来源：`android/gradle/libs.versions.toml`

### 1.1 Kotlin 与基础库

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `org.jetbrains.kotlin:kotlin-stdlib` | 2.0.20 | Apache License 2.0 | `libs.versions.toml:3` |
| `org.jetbrains.kotlinx:kotlinx-serialization-json` | 1.7.1 | Apache License 2.0 | `libs.versions.toml:9` |
| `org.jetbrains.kotlinx:kotlinx-datetime` | 0.6.0 | Apache License 2.0 | `libs.versions.toml:10` |
| `org.jetbrains.kotlinx:kotlinx-coroutines-core` | 1.8.1 | Apache License 2.0 | `libs.versions.toml:11` |
| `com.jakewharton.retrofit:retrofit2-kotlinx-serialization-converter` | 1.0.0 | Apache License 2.0 | `libs.versions.toml:12` |

### 1.2 AndroidX

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `androidx.core:core-ktx` | 1.13.1 | Apache License 2.0 | `libs.versions.toml:19` |
| `androidx.activity:activity-compose` | 1.9.2 | Apache License 2.0 | `libs.versions.toml:20` |
| `androidx.lifecycle:lifecycle-runtime-ktx` | 2.8.5 | Apache License 2.0 | `libs.versions.toml:21` |
| `androidx.lifecycle:lifecycle-runtime-compose` | 2.8.5 | Apache License 2.0 | `libs.versions.toml:21` |
| `androidx.lifecycle:lifecycle-viewmodel-compose` | 2.8.5 | Apache License 2.0 | `libs.versions.toml:21` |
| `androidx.lifecycle:lifecycle-viewmodel-ktx` | 2.8.5 | Apache License 2.0 | `libs.versions.toml:21` |

### 1.3 Compose

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `androidx.compose:compose-bom` | 2024.09.00 | Apache License 2.0 | `libs.versions.toml:4` |
| `androidx.compose.ui:ui` | —（BOM 管理） | Apache License 2.0 | `libs.versions.toml:49` |
| `androidx.compose.ui:ui-graphics` | —（BOM 管理） | Apache License 2.0 | `libs.versions.toml:50` |
| `androidx.compose.ui:ui-tooling` | —（BOM 管理） | Apache License 2.0 | `libs.versions.toml:51` |
| `androidx.compose.ui:ui-tooling-preview` | —（BOM 管理） | Apache License 2.0 | `libs.versions.toml:52` |
| `androidx.compose.material3:material3` | —（BOM 管理） | Apache License 2.0 | `libs.versions.toml:53` |
| `androidx.compose.material:material-icons-extended` | —（BOM 管理） | Apache License 2.0 | `libs.versions.toml:54` |
| `androidx.compose.foundation:foundation` | —（BOM 管理） | Apache License 2.0 | `libs.versions.toml:55` |
| `androidx.compose.animation:animation` | —（BOM 管理） | Apache License 2.0 | `libs.versions.toml:56` |
| `androidx.compose.runtime:runtime` | —（BOM 管理） | Apache License 2.0 | `libs.versions.toml:57` |

### 1.4 Navigation

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `androidx.navigation:navigation-compose` | 2.8.0 | Apache License 2.0 | `libs.versions.toml:17` |

### 1.5 Hilt（依赖注入）

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `com.google.dagger:hilt-android` | 2.52 | Apache License 2.0 | `libs.versions.toml:5` |
| `com.google.dagger:hilt-compiler` | 2.52 | Apache License 2.0 | `libs.versions.toml:5` |
| `androidx.hilt:hilt-navigation-compose` | 1.2.0 | Apache License 2.0 | `libs.versions.toml:22` |
| `androidx.hilt:hilt-work` | 1.2.0 | Apache License 2.0 | `libs.versions.toml:23` |
| `androidx.hilt:hilt-compiler` | 1.2.0 | Apache License 2.0 | `libs.versions.toml:23` |

### 1.6 Room

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `androidx.room:room-runtime` | 2.6.1 | Apache License 2.0 | `libs.versions.toml:6` |
| `androidx.room:room-ktx` | 2.6.1 | Apache License 2.0 | `libs.versions.toml:6` |
| `androidx.room:room-compiler` | 2.6.1 | Apache License 2.0 | `libs.versions.toml:6` |
| `androidx.room:room-testing` | 2.6.1 | Apache License 2.0 | `libs.versions.toml:6` |

### 1.7 DataStore

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `androidx.datastore:datastore-preferences` | 1.1.1 | Apache License 2.0 | `libs.versions.toml:13` |
| `androidx.datastore:datastore` | 1.1.1 | Apache License 2.0 | `libs.versions.toml:13` |

### 1.8 WorkManager

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `androidx.work:work-runtime-ktx` | 2.9.1 | Apache License 2.0 | `libs.versions.toml:16` |

### 1.9 Media3

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `androidx.media3:media3-exoplayer` | 1.4.0 | Apache License 2.0 | `libs.versions.toml:15` |
| `androidx.media3:media3-ui` | 1.4.0 | Apache License 2.0 | `libs.versions.toml:15` |
| `androidx.media3:media3-session` | 1.4.0 | Apache License 2.0 | `libs.versions.toml:15` |
| `androidx.media3:media3-common` | 1.4.0 | Apache License 2.0 | `libs.versions.toml:15` |

### 1.10 Retrofit / OkHttp

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `com.squareup.retrofit2:retrofit` | 2.11.0 | Apache License 2.0 | `libs.versions.toml:7` |
| `com.squareup.okhttp3:okhttp` | 4.12.0 | Apache License 2.0 | `libs.versions.toml:8` |
| `com.squareup.okhttp3:logging-interceptor` | 4.12.0 | Apache License 2.0 | `libs.versions.toml:8` |
| `com.squareup.okhttp3:mockwebserver` | 4.12.0 | Apache License 2.0 | `libs.versions.toml:8` |

### 1.11 Coil（图片加载）

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `io.coil-kt:coil-compose` | 2.7.0 | Apache License 2.0 | `libs.versions.toml:14` |
| `io.coil-kt:coil-svg` | 2.7.0 | Apache License 2.0 | `libs.versions.toml:14` |

### 1.12 Accompanist

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `com.google.accompanist:accompanist-permissions` | 0.34.0 | Apache License 2.0 | `libs.versions.toml:18` |

### 1.13 DocumentFile

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `androidx.documentfile:documentfile` | 1.0.1 | Apache License 2.0 | `libs.versions.toml:28` |

### 1.14 Desugar JDK

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `com.android.tools:desugar_jdk_libs` | 2.0.4 | Apache License 2.0 | `libs.versions.toml:27` |

### 1.15 测试依赖

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `junit:junit` | 4.13.2 | Eclipse Public License 1.0 | `libs.versions.toml:24` |
| `androidx.test.ext:junit` | 1.2.1 | Apache License 2.0 | `libs.versions.toml:25` |
| `androidx.test.espresso:espresso-core` | 3.6.1 | Apache License 2.0 | `libs.versions.toml:26` |
| `io.mockk:mockk` | 1.13.12 | Apache License 2.0 | `libs.versions.toml:29` |
| `io.mockk:mockk-android` | 1.13.12 | Apache License 2.0 | `libs.versions.toml:29` |
| `org.jetbrains.kotlinx:kotlinx-coroutines-test` | 1.8.1 | Apache License 2.0 | `libs.versions.toml:30` |
| `app.cash.turbine:turbine` | 1.1.0 | Apache License 2.0 | `libs.versions.toml:31` |
| `org.robolectric:robolectric` | 4.13 | MIT License | `libs.versions.toml:32` |
| `androidx.test:core` | 1.6.1 | Apache License 2.0 | `libs.versions.toml:33` |
| `androidx.test:runner` | 1.6.2 | Apache License 2.0 | `libs.versions.toml:34` |
| `androidx.test:rules` | 1.6.1 | Apache License 2.0 | `libs.versions.toml:35` |
| `androidx.arch.core:core-testing` | 2.2.0 | Apache License 2.0 | `libs.versions.toml:36` |
| `com.google.dagger:hilt-android-testing` | 2.52 | Apache License 2.0 | `libs.versions.toml:37` |
| `androidx.compose.ui:ui-test-junit4` | 1.7.3 | Apache License 2.0 | `libs.versions.toml:38` |
| `androidx.compose.ui:ui-test-manifest` | 1.7.3 | Apache License 2.0 | `libs.versions.toml:38` |
| `com.google.truth:truth` | 1.4.4 | Apache License 2.0 | `libs.versions.toml:39` |

### 1.16 许可证类型汇总

| 许可证 | 依赖数量 |
|---|---|
| Apache License 2.0 | 多数（约 50+） |
| Eclipse Public License 1.0 | 1（JUnit 4） |
| MIT License | 1（Robolectric） |

---

## 2. Go 后端依赖许可证清单

来源：`backend/go.mod`

### 2.1 直接依赖

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `github.com/gin-gonic/gin` | v1.12.0 | MIT License | `backend/go.mod:6` |
| `github.com/glebarez/sqlite` | v1.11.0 | MIT License | `backend/go.mod:7` |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | MIT License | `backend/go.mod:8` |
| `github.com/google/uuid` | v1.6.0 | BSD-3-Clause License | `backend/go.mod:9` |
| `github.com/gorilla/websocket` | v1.5.3 | BSD-2-Clause License | `backend/go.mod:10` |
| `github.com/lestrrat-go/file-rotatelogs` | v2.4.0+incompatible | MIT License | `backend/go.mod:11` |
| `github.com/qdrant/go-client` | v1.18.2 | Apache License 2.0 | `backend/go.mod:12` |
| `github.com/redis/go-redis/v9` | v9.19.0 | BSD-2-Clause License | `backend/go.mod:13` |
| `github.com/santhosh-tekuri/jsonschema/v6` | v6.0.2 | Apache License 2.0 | `backend/go.mod:14` |
| `github.com/sirupsen/logrus` | v1.9.4 | MIT License | `backend/go.mod:15` |
| `github.com/spf13/viper` | v1.21.0 | MIT License | `backend/go.mod:16` |
| `github.com/surrealdb/surrealdb.go` | v1.4.0 | Apache License 2.0 | `backend/go.mod:17` |
| `go.yaml.in/yaml/v3` | v3.0.4 | Apache License 2.0 | `backend/go.mod:18` |
| `golang.org/x/crypto` | v0.52.0 | BSD-3-Clause License | `backend/go.mod:19` |
| `golang.org/x/image` | v0.44.0 | BSD-3-Clause License | `backend/go.mod:20` |
| `golang.org/x/sys` | v0.45.0 | BSD-3-Clause License | `backend/go.mod:21` |
| `golang.org/x/text` | v0.40.0 | BSD-3-Clause License | `backend/go.mod:22` |
| `gopkg.in/yaml.v3` | v3.0.1 | Apache License 2.0 | `backend/go.mod:23` |
| `gorm.io/driver/sqlite` | v1.6.0 | MIT License | `backend/go.mod:24` |
| `gorm.io/gorm` | v1.31.1 | MIT License | `backend/go.mod:25` |

### 2.2 间接依赖（主要）

| 依赖 | 版本 | 许可证 | 引用 |
|---|---|---|---|
| `github.com/bytedance/gopkg` | v0.1.3 | Apache License 2.0 | `backend/go.mod:29` |
| `github.com/bytedance/sonic` | v1.15.0 | Apache License 2.0 | `backend/go.mod:30` |
| `github.com/cespare/xxhash/v2` | v2.3.0 | MIT License | `backend/go.mod:31` |
| `github.com/cloudwego/base64x` | v0.1.6 | Apache License 2.0 | `backend/go.mod:32` |
| `github.com/dustin/go-humanize` | v1.0.1 | MIT License | `backend/go.mod:33` |
| `github.com/fsnotify/fsnotify` | v1.9.0 | BSD-3-Clause License | `backend/go.mod:34` |
| `github.com/fxamacker/cbor/v2` | v2.7.0 | MIT License | `backend/go.mod:35` |
| `github.com/gabriel-vasile/mimetype` | v1.4.12 | MIT License | `backend/go.mod:36` |
| `github.com/gin-contrib/sse` | v1.1.0 | MIT License | `backend/go.mod:37` |
| `github.com/glebarez/go-sqlite` | v1.21.2 | MIT License | `backend/go.mod:38` |
| `github.com/go-playground/locales` | v0.14.1 | MIT License | `backend/go.mod:39` |
| `github.com/go-playground/universal-translator` | v0.18.1 | MIT License | `backend/go.mod:40` |
| `github.com/go-playground/validator/v10` | v10.30.1 | MIT License | `backend/go.mod:41` |
| `github.com/go-viper/mapstructure/v2` | v2.4.0 | MIT License | `backend/go.mod:42` |
| `github.com/goccy/go-json` | v0.10.5 | MIT License | `backend/go.mod:43` |
| `github.com/goccy/go-yaml` | v1.19.2 | MIT License | `backend/go.mod:44` |
| `github.com/gofrs/uuid` | v4.4.0+incompatible | MIT License | `backend/go.mod:45` |
| `github.com/jinzhu/inflection` | v1.0.0 | MIT License | `backend/go.mod:46` |
| `github.com/jinzhu/now` | v1.1.5 | MIT License | `backend/go.mod:47` |
| `github.com/jonboulle/clockwork` | v0.5.0 | Apache License 2.0 | `backend/go.mod:48` |
| `github.com/json-iterator/go` | v1.1.12 | MIT License | `backend/go.mod:49` |
| `github.com/klauspost/cpuid/v2` | v2.3.0 | MIT License | `backend/go.mod:50` |
| `github.com/leodido/go-urn` | v1.4.0 | MIT License | `backend/go.mod:51` |
| `github.com/lestrrat-go/strftime` | v1.2.0 | MIT License | `backend/go.mod:52` |
| `github.com/mattn/go-isatty` | v0.0.20 | MIT License | `backend/go.mod:53` |
| `github.com/mattn/go-sqlite3` | v1.14.22 | MIT License | `backend/go.mod:54` |
| `github.com/modern-go/concurrent` | v0.0.0-20180306012644-bacd9c7ef1dd | Apache License 2.0 | `backend/go.mod:55` |
| `github.com/modern-go/reflect2` | v1.0.2 | Apache License 2.0 | `backend/go.mod:56` |
| `github.com/pelletier/go-toml/v2` | v2.2.4 | MIT License | `backend/go.mod:57` |
| `github.com/pkg/errors` | v0.9.1 | BSD-2-Clause License | `backend/go.mod:58` |
| `github.com/quic-go/qpack` | v0.6.0 | MIT License | `backend/go.mod:59` |
| `github.com/quic-go/quic-go` | v0.59.0 | Apache License 2.0 | `backend/go.mod:60` |
| `github.com/remyoudompheng/bigfft` | v0.0.0-20230129092748-24d4a6f8daec | BSD-3-Clause License | `backend/go.mod:61` |
| `github.com/sagikazarmark/locafero` | v0.11.0 | MIT License | `backend/go.mod:62` |
| `github.com/sourcegraph/conc` | v0.3.1-0.20240121214520-5f936abd7ae8 | MIT License | `backend/go.mod:63` |
| `github.com/spf13/afero` | v1.15.0 | Apache License 2.0 | `backend/go.mod:64` |
| `github.com/spf13/cast` | v1.10.0 | MIT License | `backend/go.mod:65` |
| `github.com/spf13/pflag` | v1.0.10 | BSD-3-Clause License | `backend/go.mod:66` |
| `github.com/subosito/gotenv` | v1.6.0 | MIT License | `backend/go.mod:67` |
| `github.com/twitchyliquid64/golang-asm` | v0.15.1 | BSD-3-Clause License | `backend/go.mod:68` |
| `github.com/ugorji/go/codec` | v1.3.1 | MIT License | `backend/go.mod:69` |
| `github.com/x448/float16` | v0.8.4 | BSD-3-Clause License | `backend/go.mod:70` |
| `go.mongodb.org/mongo-driver/v2` | v2.5.0 | Apache License 2.0 | `backend/go.mod:71` |
| `go.uber.org/atomic` | v1.11.0 | MIT License | `backend/go.mod:72` |
| `golang.org/x/arch` | v0.22.0 | BSD-3-Clause License | `backend/go.mod:73` |
| `golang.org/x/net` | v0.54.0 | BSD-3-Clause License | `backend/go.mod:74` |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260427160629 | Apache License 2.0 | `backend/go.mod:75` |
| `google.golang.org/grpc` | v1.80.0 | Apache License 2.0 | `backend/go.mod:76` |
| `google.golang.org/protobuf` | v1.36.11 | BSD-3-Clause License | `backend/go.mod:77` |
| `modernc.org/libc` | v1.22.5 | BSD-3-Clause License | `backend/go.mod:78` |
| `modernc.org/mathutil` | v1.5.0 | BSD-3-Clause License | `backend/go.mod:79` |
| `modernc.org/memory` | v1.5.0 | BSD-3-Clause License | `backend/go.mod:80` |
| `modernc.org/sqlite` | v1.23.1 | BSD-3-Clause License | `backend/go.mod:81` |

### 2.3 许可证类型汇总

| 许可证 | 依赖数量（直接 + 间接） |
|---|---|
| MIT License | 多数 |
| Apache License 2.0 | 多数 |
| BSD-3-Clause License | 多数 |
| BSD-2-Clause License | 2（gorilla/websocket、redis/go-redis、pkg/errors） |

---

## 3. RootFS 组件许可证

### 3.1 Qdrant

| 项 | 内容 |
|---|---|
| 名称 | Qdrant |
| 版本 | latest（GitHub Release） |
| 来源 | `https://github.com/qdrant/qdrant/releases/latest/download/qdrant-aarch64-unknown-linux-musl.tar.gz` |
| 许可证 | Apache License 2.0 |
| 商业使用 | 允许 |
| 引用 | `android/app/src/main/assets/rootfs-manifest.json:15-24` |

### 3.2 SurrealDB

| 项 | 内容 |
|---|---|
| 名称 | SurrealDB |
| 版本 | v3.2.0 |
| 来源 | `https://github.com/surrealdb/surrealdb/releases/download/v3.2.0/surreal-v3.2.0.linux-arm64.tgz` |
| 许可证 | Business Source License 1.1（BSL 1.1） |
| 商业使用 | **非商业免费，商业使用需购买商业许可证** |
| 变更日期 | BSL 1.1 通常在 4 年后自动转为 Apache License 2.0 |
| 引用 | `android/app/src/main/assets/rootfs-manifest.json:25-34` |

**BSL 1.1 关键限制**：

- 不得用于商业生产环境（除非购买 SurrealDB 商业许可证）
- 个人使用、教育、内部测试、非商业开源项目允许使用
- 修改后的版本同样受 BSL 1.1 约束

**替代方案**（如需商业使用）：

- 购买 SurrealDB 商业许可证
- 使用其他图数据库（如 Neo4j Community Edition，GPL v3）
- 使用 Redis Graph（Apache License 2.0）

### 3.3 PRoot

| 项 | 内容 |
|---|---|
| 名称 | PRoot |
| 版本 | 待评估（proot-rs 或预编译） |
| 来源 | 待定 |
| 许可证 | GPL-2.0 License |
| 商业使用 | 允许（但衍生作品需开源） |
| 引用 | `android/native/src/main/cpp/proot_jni.cpp`（占位） |

**GPL-2.0 关键限制**：

- 修改 PRoot 源码并分发需开源修改部分
- 动态链接 PRoot 不强制开源主程序（法律灰色地带）
- 静态链接 PRoot 强制开源主程序

**当前状态**：

- `android/native/src/main/cpp/proot_jni.cpp` 仅为 JNI 壳，未集成完整 PRoot
- 第二阶段评估 proot-rs（Rust 实现，许可证待确认）或预编译 PRoot 二进制

### 3.4 Ubuntu / Debian RootFS

如未来使用 Ubuntu 或 Debian RootFS 镜像（当前未使用，采用静态二进制 + Android 文件系统）：

| 项 | 内容 |
|---|---|
| 来源 | Ubuntu / Debian 官方镜像 |
| 许可证 | 各包许可证（多数 GPL / LGPL / Apache / MIT） |
| 商业使用 | 通常允许（需逐包检查） |

**当前策略**：

- 不使用完整 Linux 发行版 RootFS
- 仅使用静态二进制（Go 后端 + Qdrant musl + SurrealDB gnu）
- 避免引入大量 GPL 包

---

## 4. Amitia 自身许可证

### 4.1 主许可证

| 项 | 内容 |
|---|---|
| 项目名称 | Amitia（U-Ai） |
| 许可证 | GNU Affero General Public License v3.0（AGPL-3.0） |
| 文件 | `LICENSE` |
| 引用 | `backend/pkg/platform/runtime_platform.go:1-2`（SPDX 标识） |

### 4.2 AGPL-3.0 关键条款

- **源代码公开**：任何使用 Amitia 提供网络服务的实例必须公开源代码
- **修改分发**：修改后的版本必须以 AGPL-3.0 许可证分发
- **网络使用**：用户通过网络使用服务时，也视为分发，需公开源代码
- **兼容性**：与 GPL-3.0 兼容，与 Apache 2.0 兼容（在 GPLv3 兼容条款下），与 LGPL 兼容

### 4.3 第三方许可证兼容性

| 第三方许可证 | 与 AGPL-3.0 兼容性 |
|---|---|
| Apache License 2.0 | 兼容（可链接） |
| MIT License | 兼容（可链接） |
| BSD-2-Clause License | 兼容（可链接） |
| BSD-3-Clause License | 兼容（可链接） |
| GPL-2.0 | 不兼容（不可静态链接，可动态链接在法律灰色地带） |
| LGPL | 兼容（可动态链接） |
| Eclipse Public License 1.0 | 兼容（仅测试依赖，不进入运行时） |
| Business Source License 1.1 | 不兼容（商业限制，不可作为衍生作品分发，仅作为独立组件运行） |

### 4.4 注意事项

- **SurrealDB BSL 1.1**：作为独立进程运行，不修改源码，AGPL-3.0 项目可调用其 API
- **PRoot GPL-2.0**：第二阶段集成时需谨慎评估，建议使用动态链接或独立进程调用
- **Go 静态链接**：Go 编译产物静态链接所有依赖，但 BSD/MIT/Apache 许可证允许静态链接且无需开源主程序

---

## 5. 许可证文件清单

Android 端应在发布 APK 时包含以下许可证文件（待后续阶段整理）：

```
android/app/src/main/assets/licenses/
├── AMITIA_LICENSE.txt              # AGPL-3.0 全文
├── KOTLIN_LICENSE.txt              # Apache 2.0
├── ANDROIDX_LICENSE.txt            # Apache 2.0
├── COMPOSE_LICENSE.txt             # Apache 2.0
├── HILT_LICENSE.txt                # Apache 2.0
├── RETROFIT_LICENSE.txt            # Apache 2.0
├── OKHTTP_LICENSE.txt              # Apache 2.0
├── ROOM_LICENSE.txt                # Apache 2.0
├── COIL_LICENSE.txt                # Apache 2.0
├── MEDIA3_LICENSE.txt              # Apache 2.0
├── JUNIT_LICENSE.txt               # Eclipse Public License 1.0
├── MOCKK_LICENSE.txt               # Apache 2.0
├── TRUTH_LICENSE.txt               # Apache 2.0
├── TURBINE_LICENSE.txt             # Apache 2.0
├── ROBOLECTRIC_LICENSE.txt         # MIT License
├── GIN_LICENSE.txt                 # MIT License
├── GORM_LICENSE.txt                # MIT License
├── GLEBAREZ_SQLITE_LICENSE.txt     # MIT License
├── MODERNC_SQLITE_LICENSE.txt      # BSD-3-Clause License
├── VIPER_LICENSE.txt               # MIT License
├── GORILLA_WEBSOCKET_LICENSE.txt   # BSD-2-Clause License
├── GOLANG_JWT_LICENSE.txt          # MIT License
├── SURREALDB_GO_LICENSE.txt        # Apache 2.0
├── QDRANT_GO_CLIENT_LICENSE.txt    # Apache 2.0
├── QDRANT_LICENSE.txt              # Apache 2.0
├── SURREALDB_LICENSE.txt           # Business Source License 1.1
└── PROOT_LICENSE.txt               # GPL-2.0 License
```

实际许可证文件将在第二阶段整理并打包到 APK 中。

---

## 6. 法律免责声明

本文档仅为 Amitia 项目内部的许可证清单整理，不构成法律意见。在商业发布前，建议咨询专业法律顾问确认：

1. 所有依赖的许可证兼容性
2. SurrealDB BSL 1.1 在目标场景下的合规性
3. PRoot GPL-2.0 链接方式的合规性
4. AGPL-3.0 在网络服务场景下的源代码公开义务
5. 各依赖版本的最新许可证条款（许可证可能随版本变更）
