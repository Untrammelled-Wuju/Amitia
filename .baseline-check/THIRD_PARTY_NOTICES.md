# Third-Party Notices / 第三方组件声明

本文件记录当前仓库中可识别的直接第三方依赖和资源。第三方组件仍然适用其各自原始许可证、服务条款或权利声明。本清单不是完整法律审计，间接依赖请结合锁文件和依赖管理工具继续核验。

## Go Modules

| Name | Version | Project | License | Used in | NOTICE | Risk |
|---|---:|---|---|---|---|---|
| github.com/gin-gonic/gin | v1.12.0 | https://github.com/gin-gonic/gin | MIT | backend/go.mod | No known extra NOTICE | Direct dependency |
| github.com/glebarez/sqlite | v1.11.0 | https://github.com/glebarez/sqlite | MIT | backend/go.mod | No known extra NOTICE | Direct dependency |
| github.com/golang-jwt/jwt/v5 | v5.3.1 | https://github.com/golang-jwt/jwt | MIT | backend/go.mod | No known extra NOTICE | Direct dependency |
| github.com/google/uuid | v1.6.0 | https://github.com/google/uuid | BSD-3-Clause | backend/go.mod | Preserve license | Direct dependency |
| github.com/lestrrat-go/file-rotatelogs | v2.4.0+incompatible | https://github.com/lestrrat-go/file-rotatelogs | MIT | backend/go.mod | No known extra NOTICE | Direct dependency |
| github.com/redis/go-redis/v9 | v9.19.0 | https://github.com/redis/go-redis | BSD-2-Clause | backend/go.mod | Preserve license | Direct dependency |
| github.com/sirupsen/logrus | v1.9.4 | https://github.com/sirupsen/logrus | MIT | backend/go.mod | No known extra NOTICE | Direct dependency |
| github.com/spf13/viper | v1.21.0 | https://github.com/spf13/viper | MIT | backend/go.mod | No known extra NOTICE | Direct dependency |
| golang.org/x/crypto | v0.52.0 | https://cs.opensource.google/go/x/crypto | BSD-3-Clause | backend/go.mod | Preserve license | Direct dependency |
| gorm.io/gorm | v1.31.1 | https://github.com/go-gorm/gorm | MIT | backend/go.mod | No known extra NOTICE | Direct dependency |

## Frontend npm Packages

| Name | Version | Project | License | Used in | NOTICE | Risk |
|---|---:|---|---|---|---|---|
| @element-plus/icons-vue | ^2.3.0 | https://github.com/element-plus/element-plus-icons | MIT | front/package.json | No known extra NOTICE | Direct dependency |
| axios | ^1.7.4 | https://github.com/axios/axios | MIT | front/package.json | No known extra NOTICE | Direct dependency |
| echarts | ^6.1.0 | https://github.com/apache/echarts | Apache-2.0 | front/package.json | Preserve Apache NOTICE if redistributed | Direct dependency |
| element-plus | ^2.7.0 | https://github.com/element-plus/element-plus | MIT | front/package.json | No known extra NOTICE | Direct dependency |
| pinia | ^2.1.0 | https://github.com/vuejs/pinia | MIT | front/package.json | No known extra NOTICE | Direct dependency |
| vue | ^3.4.0 | https://github.com/vuejs/core | MIT | front/package.json | No known extra NOTICE | Direct dependency |
| vue-router | ^4.3.0 | https://github.com/vuejs/router | MIT | front/package.json | No known extra NOTICE | Direct dependency |

## Sidecar npm Packages

| Name | Version | Project | License | Used in | NOTICE | Risk |
|---|---:|---|---|---|---|---|
| @fastify/cors | ^9.0.0 | https://github.com/fastify/fastify-cors | MIT | backend/sidecar, backend/qq-sidecar | No known extra NOTICE | Direct dependency |
| @tencent-weixin/openclaw-weixin | ^2.4.3 | https://www.npmjs.com/package/@tencent-weixin/openclaw-weixin | License status: TO BE VERIFIED | backend/sidecar/package.json | TO BE VERIFIED | WeChat SDK license/terms require manual review |
| axios | ^1.16.1 | https://github.com/axios/axios | MIT | backend/sidecar/package.json | No known extra NOTICE | Direct dependency |
| fast-xml-parser | ^4.5.0 | https://github.com/NaturalIntelligence/fast-xml-parser | MIT | backend/sidecar/package.json | No known extra NOTICE | Direct dependency |
| fastify | ^4.29.0 | https://github.com/fastify/fastify | MIT | backend/sidecar, backend/qq-sidecar | No known extra NOTICE | Direct dependency |
| openclaw | ^2026.5.19 | https://www.npmjs.com/package/openclaw | License status: TO BE VERIFIED | backend/sidecar/package.json | TO BE VERIFIED | Bridge SDK license/terms require manual review |
| qrcode | ^1.5.4 | https://github.com/soldair/node-qrcode | MIT | backend/sidecar/package.json | No known extra NOTICE | Direct dependency |
| form-data | ^4.0.5 | https://github.com/form-data/form-data | MIT | backend/qq-sidecar/package.json | No known extra NOTICE | Direct dependency |
| ws | ^8.17.0 | https://github.com/websockets/ws | MIT | backend/qq-sidecar/package.json | No known extra NOTICE | Direct dependency |

## Python Packages

| Name | Version | Project | License | Used in | NOTICE | Risk |
|---|---:|---|---|---|---|---|
| pycryptodome | not pinned | https://www.pycryptodome.org/ | BSD/Public Domain mix | wechat-chat-extractor/requirements.txt | Preserve license | Version should be pinned and verified |
| pymem | not pinned | https://github.com/srounet/Pymem | MIT | wechat-chat-extractor/requirements.txt | No known extra NOTICE | Version should be pinned and verified |

## External Services, SDKs, and Bridges

| Item | Version | License / Terms | Used in | Risk |
|---|---:|---|---|---|
| OpenAI-compatible API providers | User configured | Service terms vary | Model config / chat | Terms and data processing obligations require user review |
| DeepSeek / Ollama compatible models | User configured | Model-specific licenses vary | README / runtime configuration | Model license status depends on user-selected model |
| Doubao Embedding Vision | User configured | Service terms TO BE VERIFIED | README / embedding | Provider terms require manual review |
| WeChat bridge / OpenClaw integration | package versions above | License status: TO BE VERIFIED | backend/sidecar | WeChat/OpenClaw terms and account rules require manual review |
| QQ bridge | package versions above | License status: TO BE VERIFIED | backend/qq-sidecar | QQ bridge implementation and SDK terms require manual review |
| Qdrant runtime binaries | bundled under the runtime directory qdrant folder and backend/qdrant | License status: TO BE VERIFIED | runtime vector database | Bundled binary notices should be reviewed before redistribution |
| SurrealDB runtime binary | bundled under the runtime directory surrealdb folder if present | License status: TO BE VERIFIED | runtime graph database | Bundled binary notices should be reviewed before redistribution |
| unidbg-fetch-qsign-all.jar | bundled JAR | License status: TO BE VERIFIED | backend/libs | Bundled third-party binary requires manual license review |

## Fonts, Images, Icons, Audio, Models, and Media

| Item | Location | License | Risk |
|---|---|---|---|
| Application icons | front/public/icons | License status: TO BE VERIFIED | Confirm original authorship or asset license |
| PWA manifest icon references | front/public/manifest.webmanifest | License status: TO BE VERIFIED | Confirm icon rights |
| Runtime voice messages | backend/data/voice_msg | User/runtime generated | Do not redistribute without confirming rights |
| Runtime images and videos | backend/data/images, backend/data/videos | User/runtime generated | Do not redistribute without confirming rights |
| Test video | _test_video_60mb.mp4 | License status: TO BE VERIFIED | Large media file requires rights review |

## Copied, Generated, Vendor, and Build Outputs

The following locations were treated as third-party, generated, runtime, or build artifacts and were not relicensed as project-owned AGPL source code:

- front/node_modules
- backend/sidecar/node_modules
- backend/qq-sidecar/node_modules
- front/dist
- release
- backend/data
- backend/qdrant
- qdrant
- backend/libs/*.jar
- backend/sidecar/bundle.mjs
- backend/qq-sidecar/bundle.mjs
- generated helper files under backend/internal/system/_gen*.py

## Strong-Copyleft or Source-Available Licenses

No direct dependency was confirmed as GPL, AGPL, SSPL, Elastic License, or BSL during this automated pass. Several bundled binaries and bridge SDKs remain TO BE VERIFIED and require manual legal review before public redistribution.
