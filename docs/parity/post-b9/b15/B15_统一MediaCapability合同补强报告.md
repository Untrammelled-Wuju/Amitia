# B15 统一Media Capability合同补强报告

## 1. 执行结果

状态：**PASS_NO_CODE_CHANGE**

Construction Mode：REUSE + EXTEND（实际执行：仅文档映射）

## 2. B9P8输入

- B9P8状态：PASS
- Final Manifest：已读取
- Architecture Guard：20条BLOCKER规则全部遵守
- Step Reuse Matrix：B15 = REUSE + EXTEND + NEW_PROVIDER

## 3. B13输入

- B13状态：PASS_NO_CODE_CHANGE
- ResourceURI Authority：amitia:// scheme
- PhysicalResolver：正确实现

## 4. B14输入

- B14状态：PASS
- Browser合同：Resource Transfer Contract已建立
- Browser State/Error Projection：已定义
- Media交叉合同：Browser screenshot/download/upload已使用ResourceURI

## 5. Construction Mode

Mode = REUSE + EXTEND + NEW_PROVIDER

实际执行：
- REUSE：现有Image/Vision/ASR/TTS/Realtime/Browser全部复用
- EXTEND：无代码扩展（文档映射）
- NEW_PROVIDER：Video/FFmpeg/Probe/Conversion预留给B87-B89

## 6. 当前Media领域总览

Amitia当前存在6个独立Media Domain：

| Domain | 路径 | 状态 |
|--------|------|------|
| Image Generation | imagegen + imageprovider | REUSE |
| Vision | vision | REUSE |
| ASR | asr | REUSE |
| TTS | tts | REUSE |
| Realtime Voice | realtime | REUSE |
| Browser Media | browser | REUSE |

无统一Media Runtime、Media Provider Registry、Media Store。

## 7. Image Generation

- Authority：ImageGenerationProvider interface (backend/internal/imageprovider/provider.go)
- Provider Registry：imageprovider.Registry
- 支持Provider：seedream, stability, tongyi
- 合同：ImageGenerationRequest → ImageGenerationResult → GenerationResult/CandidateImage
- 配置DB：image_gen_configs

## 8. Image Provider

- 核心接口：
  - ImageGenerationProvider: Submit/Query/Cancel/ValidateConfig/Capabilities
  - ExtendedProvider: + ExtendedCapabilities
  - Registry: Register/Resolve/Describe/Names
- 错误：ProviderError domain type with RetryClass
- 能力声明：ImageGenerationCapabilities

## 9. Vision

- Authority：VisionService (backend/internal/vision/)
- 协议：通过Chat Completions API实现视觉分析
- 配置DB：vision_configs
- 支持：volcengine, gemini, openai-compatible
- 输出：结构化文本结果（非媒体资源）

## 10. ASR

- Authority：AsrService (backend/internal/asr/)
- 配置DB：asr_configs
- 默认Provider：volcengine (SeedASR)
- 输出：通过Realtime VoiceTurn转录文本
- 注：无独立ASR Error domain，依赖Realtime错误传递

## 11. TTS

- Authority：TTS Service + Engine (backend/internal/tts/)
- 支持Provider：volcengine, openai, azure, edge, elevenlabs, minimax, aliyun, cosyvoice
- 配置DB：tts_configs
- 输出：SynthesizeResponse { AudioURL, Duration }
- Voice Clone：支持自定义音色克隆
- 缓存：data/tts_cache/

## 12. Realtime Voice

- Authority：VoiceSessionState + Protocol (backend/internal/realtime/)
- Provider：Volcano Engine Realtime Dialogue
- Transport：WebSocket binary protocol
- Session State：VoiceSessionState (in-memory sync.Map)
- Turn States：interim / final / cancel
- 注：Stream media (PCM frames)，不适用ResourceURI

## 13. Future Video / FFmpeg Gap

B15不实现。留给B87-B89：
- FFmpeg transcoding
- Video frame extraction
- Format conversion
- Media probe (ffprobe)
- Metadata extraction

## 14. Final Media Capability

- Required：0 (无新增)
- Already Supported：6 (image.generate, image.analyze, audio.transcribe, audio.synthesize, voice.realtime, browser.screenshot)
- Partial：0
- Future Provider：0
- Unresolved：0

## 15. Tool Exposure

根据final_tool_manifest.json，当前Media能力均未注册为Agent Tool。
- Agent Callable：253 capabilities中不包含Media-specific tools
- Internal Domain：所有Media调用均为Internal Domain Call
- User Action：部分Media功能通过UI触发（TTS/ASR）
- Unexpected Media Tool：0

## 16. Provider Mapping

所有Domain保持独立Provider，无统一MediaProviderRegistry：
- Image：imageprovider.Registry
- Vision：vision.Service
- ASR：asr.Service
- TTS：tts.Service + Engine
- Realtime：realtime.Protocol

## 17. Runtime Binding

- Image/Vision/ASR/TTS：INTERNAL_DOMAIN_CALL
- Realtime：SESSION_BASED (WebSocket)
- Browser：RuntimeTypeBrowser (B14)
- 无新增Runtime Binding

## 18. Permission Mapping

- Authority：PermissionBroker (existing)
- 无新增Permission
- 文件读写：files.read/write
- 麦克风/摄像头：microphone.use/camera.use（平台授权）

## 19. ResourceURI统一

- Official Scheme：amitia:// (B13)
- Media Input：ResourceURI + remote URL + base64 + stream
- Media Output：ResourceURI + local path + remote URL
- Browser Media：已使用ResourceURI (B14)
- Physical Path不作为公共合同：是
- 新增Media Store：否

## 20. Media Input

- Image：ImageInput { Path, Bytes, MimeType }
- Vision：Image URL via Chat API
- ASR：Audio resource/stream
- TTS：Text string
- Realtime：PCM frames
- Browser：ResourceURI

## 21. Media Output

- Image：CandidateImage { Bytes, RemoteURL, MimeType, Width, Height }
- Vision：Structured text result
- ASR：Transcription text
- TTS：SynthesizeResponse { AudioURL, Duration }
- Realtime：PCM frames + ASR events
- Browser：ResourceURI (screenshot/download)

## 22. Media Metadata

无统一Media Metadata合同：
- Image：MIME, width, height in CandidateImage
- Audio：duration in SynthesizeResponse
- Realtime：AudioConfig { Format, SampleRate, Channel }
- Format Registry：无新增

## 23. Stream与Stored Resource边界

- Stream Media：Realtime PCM frames (不写ResourceURI)
- Stored Media Resource：Image/Audio files (可映射ResourceURI)
- 边界明确：是

## 24. Browser Screenshot

- 合同：BrowserScreenshotResult { ResourceURI, Width, Height } (B14)
- ResourceURI：amitia://temp/screenshots/<filename>
- MIME：Implicit from format parameter

## 25. Browser Download

- 合同：BrowserDownloadResult { ResourceURI, Filename, SizeBytes, ContentType } (B14)
- MIME：ContentType为标准MIME类型

## 26. Browser Upload

- 合同：BrowserUploadRequest/Result { ResourceURI } (B14)
- ResourceURI处理：resourceuri.Parse + PhysicalResolver.Resolve

## 27. Media State Projection

- Image：SubmissionState domain-owned
- Voice/Realtime：VoiceSessionState domain-owned
- Browser：BrowserSessionState + TabState (B14)
- Protocol Projection：从B9P8 final_state_projection_manifest读取
- Media Global State Store：无（0）

## 28. Media Error Projection

- Image：ProviderError domain type (IMAGE_* codes)
- Browser：BrowserError domain type (B14)
- Protocol Projection：从B9P8 final_error_projection_manifest读取
- Media Error Registry：无（0）

## 29. FFmpeg/Video Future Contract

B15不实现。未来合同预留给B87-B89：
- 输入：ResourceURI
- 输出：ResourceURI
- 长操作：TaskRuntime-compatible
- 状态/错误：Domain-owned

## 30. Media Probe Future Contract

B15不实现。未来可能：
- Probe(ResourceURI) → MediaInfo { MIME, Duration, Dimensions, Codec }
- 实际FFprobe执行留给B87-B89

## 31. Conversion Future Contract

B15不实现。未来可能：
- Convert(input ResourceURI, targetFormat) → ResourceURI
- 实际转换留给B87-B89

## 32. 实际代码修改

无代码修改。

现有Media领域合同已经满足B15要求，未新增生产Media系统。

各Domain现状：
- imagegen/imageprovider：完整的Provider/Registry/Result/Error合同
- vision：完整的Config/Service/Provider合同
- asr：完整的Config/Service合同
- tts：完整的Config/Service/Engine/VoiceClone合同
- realtime：完整的Protocol/SessionState/Proxy合同
- browser：完整的Types/ResourceTransfer合同 (B14)

## 33. Backward Compatibility

完全兼容：
- imageGenerationCompatible：true
- visionCompatible：true
- asrCompatible：true
- ttsCompatible：true
- realtimeCompatible：true
- resourceUriCompatible：true
- browserContractCompatible：true

无Breaking Change。

## 34. Security Validation

- rawSecretInMediaContract：0
- rawPhysicalPathAsPortableContract：0
- providerPermissionBypass：0
- agentDirectProviderBypass：0
- productionFakeMediaProvider：0
- newMediaPermissionCenter：0
- newMediaResourceStore：0

## 35. Duplicate System Validation

- MediaRuntime2：0
- MediaProviderRegistry2：0
- ImageProvider2：0
- VisionRuntime2：0
- ASR2：0
- TTS2：0
- Realtime2：0
- MediaStore2：0
- ArtifactSystem2：0
- PermissionSystem2：0
- ExecutionPipeline2：0

## 36. B18输入

- Media Canonical Authorities：各Domain独立
- Capability Mapping：6 capabilities ALREADY_SUPPORTED
- Provider Mapping：全部REUSE_EXISTING_PROVIDER
- ResourceURI Mapping：amitia:// (B13)
- Runtime Binding：Existing Runtime
- Permission Mapping：PermissionBroker
- State/Error Projection：Domain-owned
- Duplicate System Validation：All 0
- Browser Integration：COMPATIBLE

## 37. B69～B73输入

Android Media参考：
- ResourceURI：amitia:// scheme
- Stream Media：PCM边界
- Permission：microphone.use/camera.use
- 无新增合同

## 38. B87～B89输入

Video/FFmpeg Provider参考：
- FFmpeg：按需评估
- Video Processing：按需评估
- Frame Extraction：按需评估
- Format Conversion：按需评估
- Media Probe：按需评估
- I/O：ResourceURI
- 长操作：TaskRuntime-compatible

## 39. iOS Media输入

iOS Media参考：
- ResourceURI：amitia:// scheme
- Stream Media：Realtime边界
- Permission：iOS Platform Authorization
- 无新增合同

## 40. B16兼容

- B16 Voice合同参考：
  - TTS Output：SynthesizeResponse.AudioURL
  - Realtime Stream：PCM frames
  - Voice Clone：VoiceCloneRequest/Response
- 无合同修改要求

## 41. Deferred Gap

- NEW_VIDEO_PROVIDER：B87-B89
- FFMPEG_IMPLEMENTATION：B87-B89
- MEDIA_PROBE_IMPLEMENTATION：B87-B89
- FORMAT_CONVERSION：B87-B89
- FRAME_EXTRACTION：B87-B89

## 42. 测试

B15无代码变更。现有测试状态：
- realtime：proxy_test.go, voice_session_test.go, channel_switch_test.go
- resourceuri：resource_uri_test.go, physical_resolver_test.go
- browser：contract_test.go (B14)
- 无其他Media测试

## 43. Source Boundary

- Modified files：[]
- Unexpected files：[]
- go.mod：unchanged
- go.sum：unchanged
- DB：unchanged

## 44. 阻断项

无。

## 45. 最终结论

1. B15仅复用/扩展现有Image/Vision/ASR/TTS/Realtime体系，未修改任何代码
2. 没有建立第二套Media Runtime或Provider Registry
3. Media输入输出已支持ResourceURI（B13/B14）
4. Browser截图/下载/上传已复用ResourceURI合同（B14）
5. Image、ASR、TTS、Realtime现有Provider配置和业务链保持不变
6. Media Capability全部拥有明确Provider/Runtime/Permission映射
7. State/Error继续由各Domain事实源负责
8. 没有把Media能力错误暴露为Agent Tool
9. FFmpeg/视频/格式转换/抽帧正确留给B87-B89
10. 没有新增FFmpeg等第三方依赖
11. B15允许进入B18统一合同验收
