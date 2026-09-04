import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_connection/backend_connection_availability.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/backend_transport/websocket/backend_websocket_client.dart';
import '../../../../core/backend_transport/websocket/backend_websocket_message.dart';
import '../../../../core/backend_transport/websocket/backend_websocket_session.dart';
import '../../../../core/realtime/realtime_audio_bridge.dart';
import '../../../../core/realtime/realtime_visual_bridge.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';

enum RealtimeCallMode { voice, video, screen }

class RealtimeVoiceCallSheet extends ConsumerStatefulWidget {
  const RealtimeVoiceCallSheet({
    super.key,
    required this.conversationId,
    required this.characterName,
    this.initialMode = RealtimeCallMode.voice,
  });

  final String conversationId;
  final String characterName;
  final RealtimeCallMode initialMode;

  @override
  ConsumerState<RealtimeVoiceCallSheet> createState() => _RealtimeVoiceCallSheetState();
}

class _RealtimeVoiceCallSheetState extends ConsumerState<RealtimeVoiceCallSheet> {
  final RealtimeAudioBridge _audio = RealtimeAudioBridge();
  final RealtimeVisualBridge _visual = RealtimeVisualBridge();

  BackendWebSocketClient? _client;
  BackendWebSocketSession? _session;
  BackendWebSocketSession? _visualSession;
  StreamSubscription? _wsSubscription;
  StreamSubscription? _visualWsSubscription;
  StreamSubscription<Uint8List>? _audioSubscription;
  StreamSubscription<RealtimeVisualFrame>? _visualFrameSubscription;
  Timer? _durationTimer;

  String _state = 'connecting';
  String? _error;
  String? _visionStatus;
  bool _aiSpeaking = false;
  bool _muted = false;
  bool _cameraActive = false;
  bool _screenActive = false;
  bool _cameraSupported = true;
  bool _screenSupported = true;
  bool _initialMediaApplied = false;
  int _seconds = 0;
  String? _dialogId;
  String? _callId;
  Uint8List? _latestCameraFrame;
  Uint8List? _latestScreenFrame;
  DateTime _lastSpeechVisualBoostAt = DateTime.fromMillisecondsSinceEpoch(0, isUtc: true);

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _connect());
  }

  @override
  void dispose() {
    unawaited(_shutdown(sendStop: true));
    super.dispose();
  }

  Future<void> _connect() async {
    if ((_state == 'connected' || _state == 'connecting') && _session != null) return;
    if (mounted) {
      setState(() {
        _state = 'connecting';
        _error = null;
        _visionStatus = null;
        _seconds = 0;
        _initialMediaApplied = false;
      });
    }

    try {
      final availability = await ref.read(backendConnectionProvider.future);
      if (availability is! BackendConnectionAvailable) {
        throw StateError('后端当前不可用');
      }

      try {
        final visualStatus = await _visual.status();
        _cameraSupported = visualStatus.cameraSupported;
        _screenSupported = visualStatus.screenSupported;
      } catch (_) {
        _cameraSupported = false;
        _screenSupported = false;
      }

      final client = BackendWebSocketClient(availability.config);
      final session = await client.connect(
        '/api/realtime/v2/session',
        queryParameters: <String, dynamic>{
          'conversationId': widget.conversationId,
          if ((_dialogId ?? '').isNotEmpty) 'dialogId': _dialogId,
        },
      );
      _client = client;
      _session = session;
      _wsSubscription = session.messages.listen(
        _handleSocketMessage,
        onError: (Object error) => _fail(error.toString()),
        onDone: () {
          if (mounted && _state != 'idle' && _state != 'error') {
            setState(() => _state = 'idle');
          }
        },
      );

      _audioSubscription = _audio.inputPcm.listen(
        (pcm) {
          final active = _session;
          if (_state != 'connected' || _aiSpeaking || _muted || active == null) return;
          final now = DateTime.now().toUtc();
          if ((_cameraActive || _screenActive) &&
              now.difference(_lastSpeechVisualBoostAt) >= const Duration(milliseconds: 900) &&
              _pcmHasSpeechEnergy(pcm)) {
            _lastSpeechVisualBoostAt = now;
            if (_cameraActive) unawaited(_visual.requestImmediateFrame('camera'));
            if (_screenActive) unawaited(_visual.requestImmediateFrame('screen'));
          }
          unawaited(
            active.send(
              WebSocketTextMessage(
                jsonEncode(<String, dynamic>{'event': 'audio', 'data': base64Encode(pcm)}),
              ),
            ),
          );
        },
        onError: (Object error) => _fail('麦克风采集失败：$error'),
      );

      _visualFrameSubscription = _visual.frames.listen(
        _handleVisualFrame,
        onError: (Object error) {
          if (mounted) setState(() => _visionStatus = '视觉采集异常');
        },
      );
      await _audio.startCapture();
    } catch (error) {
      await _shutdown(sendStop: false);
      _fail(error.toString().replaceFirst('Bad state: ', ''));
    }
  }

  Future<void> _connectVisualSession(Map<dynamic, dynamic> call) async {
    final callId = call['callId']?.toString().trim() ?? '';
    final ticket = call['visualTicket']?.toString().trim() ?? '';
    final endpoint = call['visualEndpoint']?.toString().trim() ?? '';
    if (callId.isEmpty || ticket.isEmpty || endpoint.isEmpty) {
      throw StateError('视觉会话参数不完整');
    }
    final client = _client;
    if (client == null) throw StateError('实时客户端不可用');
    final visualSession = await client.connect(
      endpoint,
      queryParameters: <String, dynamic>{'callId': callId, 'ticket': ticket},
    );
    _callId = callId;
    _visualSession = visualSession;
    _visualWsSubscription = visualSession.messages.listen(
      (message) {
        if (message is! WebSocketTextMessage) return;
        try {
          final decoded = jsonDecode(message.data);
          if (decoded is Map && decoded['event'] == 'visual.rejected' && mounted) {
            setState(() => _visionStatus = '视觉帧被拒绝');
          }
        } catch (_) {}
      },
      onError: (_) {
        if (mounted) setState(() => _visionStatus = '视觉通道已断开');
      },
    );
  }

  void _handleVisualFrame(RealtimeVisualFrame frame) {
    if (mounted) {
      setState(() {
        if (frame.source == 'camera') _latestCameraFrame = frame.bytes;
        if (frame.source == 'screen') _latestScreenFrame = frame.bytes;
      });
    }
    final visualSession = _visualSession;
    if (_state != 'connected' || visualSession == null) return;
    unawaited(
      visualSession.send(
        WebSocketTextMessage(
          jsonEncode(<String, dynamic>{
            'event': 'visual.frame',
            'data': <String, dynamic>{
              'source': frame.source,
              'sequence': frame.sequence,
              'captureTimestamp': frame.capturedAt.toUtc().toIso8601String(),
              'mime': frame.mime,
              'width': frame.width,
              'height': frame.height,
              'data': base64Encode(frame.bytes),
            },
          }),
        ),
      ),
    );
  }

  void _handleSocketMessage(BackendWebSocketMessage message) {
    if (message is WebSocketErrorMessage) {
      _fail(message.error.toString());
      return;
    }
    if (message is! WebSocketTextMessage) return;
    try {
      final decoded = jsonDecode(message.data);
      if (decoded is! Map) return;
      final event = decoded['event']?.toString() ?? '';
      switch (event) {
        case 'connected':
          final dialogId = decoded['dialogId']?.toString();
          if (dialogId != null && dialogId.isNotEmpty) _dialogId = dialogId;
          final call = decoded['call'];
          if (call is! Map) {
            _fail('实时通话缺少视觉会话信息');
            return;
          }
          unawaited(
            _connectVisualSession(call).then((_) async {
              if (!mounted) return;
              setState(() => _state = 'connected');
              _durationTimer ??= Timer.periodic(const Duration(seconds: 1), (_) {
                if (mounted && _state == 'connected') setState(() => _seconds++);
              });
              await _applyInitialMediaMode();
              _publishSources();
            }).catchError((Object error) => _fail(error.toString())),
          );
        case 'audio':
          final payload = decoded['data']?.toString() ?? '';
          if (payload.isEmpty) return;
          _aiSpeaking = true;
          unawaited(_audio.playPcm(Uint8List.fromList(base64Decode(payload))));
          if (mounted) setState(() {});
        case 'tts_ended':
          _aiSpeaking = false;
          if (mounted) setState(() {});
        case 'asr_final':
          final data = decoded['data'];
          if (data is! Map) return;
          final transcript = data['transcript']?.toString().trim() ?? '';
          final eventId = data['eventId']?.toString().trim() ?? '';
          if (transcript.isEmpty || eventId.isEmpty) return;
          unawaited(
            _audio.emitWorkflowAsrFinal(
              transcript: transcript,
              eventId: eventId,
              sessionId: data['sessionId']?.toString(),
              conversationId: data['conversationId']?.toString(),
              visualContext: data['visualContext']?.toString(),
              visualSource: data['visualSource']?.toString(),
            ),
          );
        case 'vision.updated':
          final data = decoded['data'];
          if (data is Map && mounted) {
            setState(() => _visionStatus = '视觉已更新');
          }
        case 'vision.status':
          final data = decoded['data'];
          if (data is Map && data['available'] == false && mounted) {
            setState(() => _visionStatus = '视觉模型暂不可用');
          }
        case 'SessionFinished':
        case 'disconnected':
          unawaited(_shutdown(sendStop: false));
          if (mounted) setState(() => _state = 'idle');
        case 'error':
          _fail(decoded['data']?.toString() ?? '实时通话连接失败');
      }
    } catch (_) {
      // Ignore unknown compatibility frames.
    }
  }

  void _fail(String message) {
    if (!mounted) return;
    setState(() {
      _state = 'error';
      _error = message;
    });
  }

  Future<void> _shutdown({required bool sendStop}) async {
    _durationTimer?.cancel();
    _durationTimer = null;
    final session = _session;
    final visualSession = _visualSession;
    _session = null;
    _visualSession = null;
    if (sendStop) {
      try {
        await session?.send(WebSocketTextMessage(jsonEncode(const {'event': 'stop'})));
      } catch (_) {}
      try {
        await visualSession?.send(WebSocketTextMessage(jsonEncode(const {'event': 'stop'})));
      } catch (_) {}
    }
    await _audioSubscription?.cancel();
    _audioSubscription = null;
    await _visualFrameSubscription?.cancel();
    _visualFrameSubscription = null;
    await _wsSubscription?.cancel();
    _wsSubscription = null;
    await _visualWsSubscription?.cancel();
    _visualWsSubscription = null;
    try { await _audio.reset(); } catch (_) {}
    try { await _visual.reset(); } catch (_) {}
    try { await visualSession?.close(); } catch (_) {}
    try { await session?.close(); } catch (_) {}
    final client = _client;
    _client = null;
    try { await client?.close(); } catch (_) {}
    _aiSpeaking = false;
    _muted = false;
    _cameraActive = false;
    _screenActive = false;
    _initialMediaApplied = false;
    _callId = null;
    _latestCameraFrame = null;
    _latestScreenFrame = null;
  }

  Future<void> _applyInitialMediaMode() async {
    if (_initialMediaApplied || _state != 'connected') return;
    _initialMediaApplied = true;
    try {
      switch (widget.initialMode) {
        case RealtimeCallMode.voice:
          return;
        case RealtimeCallMode.video:
          if (!_cameraSupported) {
            if (mounted) setState(() => _visionStatus = '当前设备不支持摄像头采集');
            return;
          }
          await _visual.startCamera();
          if (mounted) setState(() => _cameraActive = true);
        case RealtimeCallMode.screen:
          if (!_screenSupported) {
            if (mounted) setState(() => _visionStatus = '当前设备不支持屏幕采集');
            return;
          }
          await _visual.startScreen();
          if (mounted) setState(() => _screenActive = true);
      }
    } catch (error) {
      if (mounted) {
        setState(() => _visionStatus = '初始媒体采集失败：${error.toString().replaceFirst('Bad state: ', '')}');
      }
    }
  }

  Future<void> _toggleMute() async {
    if (_state != 'connected') return;
    try {
      if (_muted) {
        await _audio.startCapture();
      } else {
        await _audio.stopCapture();
      }
      if (mounted) setState(() => _muted = !_muted);
      _publishSources();
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '麦克风切换失败：$error');
    }
  }

  Future<void> _toggleCamera() async {
    if (_state != 'connected' || !_cameraSupported) return;
    try {
      if (_cameraActive) {
        await _visual.stopCamera();
      } else {
        await _visual.startCamera();
      }
      if (mounted) {
        setState(() {
          _cameraActive = !_cameraActive;
          if (!_cameraActive) _latestCameraFrame = null;
        });
      }
      _publishSources();
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '摄像头切换失败：$error');
    }
  }

  Future<void> _toggleScreen() async {
    if (_state != 'connected' || !_screenSupported) return;
    try {
      if (_screenActive) {
        await _visual.stopScreen();
      } else {
        await _visual.startScreen();
      }
      if (mounted) {
        setState(() {
          _screenActive = !_screenActive;
          if (!_screenActive) _latestScreenFrame = null;
        });
      }
      _publishSources();
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '屏幕共享切换失败：$error');
    }
  }

  void _publishSources() {
    final active = _session;
    if (active == null) return;
    unawaited(
      active.send(
        WebSocketTextMessage(
          jsonEncode(<String, dynamic>{
            'event': 'media.sources',
            'data': <String, dynamic>{
              'audio': !_muted,
              'camera': _cameraActive,
              'screen': _screenActive,
            },
          }),
        ),
      ),
    );
  }

  Future<void> _endCall() async {
    await _shutdown(sendStop: true);
    if (mounted) Navigator.of(context).pop();
  }

  String get _duration {
    final minutes = (_seconds ~/ 60).toString().padLeft(2, '0');
    final seconds = (_seconds % 60).toString().padLeft(2, '0');
    return '$minutes:$seconds';
  }

  bool _pcmHasSpeechEnergy(Uint8List pcm) {
    if (pcm.length < 2) return false;
    final data = ByteData.sublistView(pcm);
    var total = 0;
    var samples = 0;
    for (var offset = 0; offset + 1 < pcm.length; offset += 32) {
      total += data.getInt16(offset, Endian.little).abs();
      samples++;
    }
    return samples > 0 && total / samples >= 850;
  }

  @override
  Widget build(BuildContext context) {
    final name = widget.characterName.trim().isEmpty ? 'Amitia' : widget.characterName.trim();
    final initial = name.characters.first;
    final connected = _state == 'connected';
    final callMode = _screenActive
        ? '屏幕通话'
        : _cameraActive
            ? '视频通话'
            : switch (widget.initialMode) {
                RealtimeCallMode.video => '视频通话',
                RealtimeCallMode.screen => '屏幕通话',
                RealtimeCallMode.voice => '语音通话',
              };
    final statusText = _state == 'connecting'
        ? '正在连接实时通话…'
        : connected
            ? (_aiSpeaking ? '对方正在说话' : (_muted ? '麦克风已静音' : '$callMode中'))
            : _state == 'error'
                ? (_error ?? '连接失败')
                : '通话已结束';

    return SafeArea(
      top: false,
      child: SizedBox(
        height: MediaQuery.sizeOf(context).height * 0.82,
        child: Padding(
          padding: EdgeInsets.fromLTRB(AppSpacing.lg, AppSpacing.md, AppSpacing.lg, AppSpacing.lg),
          child: Column(
            children: [
              Container(
                width: 40,
                height: 4,
                decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2)),
              ),
              const SizedBox(height: 18),
              Align(
                alignment: Alignment.centerLeft,
                child: Text(
                  callMode,
                  style: AppTypography.bodySmall(context).copyWith(color: context.textSecondary, fontWeight: FontWeight.w600),
                ),
              ),
              const SizedBox(height: 14),
              Expanded(
                child: _buildVisualStage(context, initial),
              ),
              const SizedBox(height: 14),
              Text(name, style: AppTypography.pageTitle(context)),
              const SizedBox(height: 7),
              Text(
                connected ? '$statusText · $_duration${_visionStatus == null ? '' : ' · $_visionStatus'}' : statusText,
                textAlign: TextAlign.center,
                style: AppTypography.bodySmall(context).copyWith(color: _state == 'error' ? context.error : context.textSecondary),
              ),
              const SizedBox(height: 18),
              if (_state == 'error')
                Row(
                  children: [
                    Expanded(child: AmitiaButton(label: '关闭', isSecondary: true, onPressed: () => Navigator.of(context).pop())),
                    const SizedBox(width: 12),
                    Expanded(
                      child: AmitiaButton(
                        label: '重新连接',
                        onPressed: () async {
                          await _shutdown(sendStop: false);
                          await _connect();
                        },
                      ),
                    ),
                  ],
                )
              else
                SingleChildScrollView(
                  scrollDirection: Axis.horizontal,
                  child: Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      _RealtimeCallControl(
                        icon: _muted ? Icons.mic_off_outlined : Icons.mic_none_outlined,
                        label: _muted ? '取消静音' : '静音',
                        selected: _muted,
                        enabled: connected,
                        onTap: _toggleMute,
                      ),
                      const SizedBox(width: 18),
                      _RealtimeCallControl(
                        icon: _cameraActive ? Icons.videocam_off_outlined : Icons.videocam_outlined,
                        label: _cameraActive ? '关闭视频' : '视频',
                        selected: _cameraActive,
                        enabled: connected && _cameraSupported,
                        onTap: _toggleCamera,
                      ),
                      const SizedBox(width: 18),
                      _RealtimeCallControl(
                        icon: _screenActive ? Icons.stop_screen_share_outlined : Icons.screen_share_outlined,
                        label: _screenActive ? '停止共享' : '共享屏幕',
                        selected: _screenActive,
                        enabled: connected && _screenSupported,
                        onTap: _toggleScreen,
                      ),
                      const SizedBox(width: 18),
                      _RealtimeCallControl(
                        icon: Icons.call_end,
                        label: '结束',
                        destructive: true,
                        enabled: true,
                        onTap: _endCall,
                      ),
                    ],
                  ),
                ),
              const SizedBox(height: 8),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildVisualStage(BuildContext context, String initial) {
    final primary = _screenActive ? _latestScreenFrame : _cameraActive ? _latestCameraFrame : null;
    if (primary == null) {
      return Center(
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 180),
          width: 104,
          height: 104,
          decoration: BoxDecoration(
            color: _state == 'error' ? context.error.withValues(alpha: 0.12) : context.accentPrimary,
            borderRadius: BorderRadius.circular(30),
            border: Border.all(color: _aiSpeaking ? context.accentPrimary : context.borderPrimary, width: _aiSpeaking ? 4 : 1),
          ),
          alignment: Alignment.center,
          child: _state == 'error'
              ? Icon(Icons.error_outline, size: 38, color: context.error)
              : Text(initial, style: const TextStyle(color: Colors.white, fontSize: 30, fontWeight: FontWeight.w700)),
        ),
      );
    }
    return ClipRRect(
      borderRadius: BorderRadius.circular(18),
      child: Stack(
        fit: StackFit.expand,
        children: [
          Container(color: Colors.black),
          Image.memory(primary, fit: _screenActive ? BoxFit.contain : BoxFit.cover, gaplessPlayback: true),
          if (_screenActive && _cameraActive && _latestCameraFrame != null)
            Positioned(
              right: 12,
              bottom: 12,
              width: 116,
              height: 156,
              child: ClipRRect(
                borderRadius: BorderRadius.circular(14),
                child: DecoratedBox(
                  decoration: BoxDecoration(border: Border.all(color: Colors.white24), color: Colors.black),
                  child: Image.memory(_latestCameraFrame!, fit: BoxFit.cover, gaplessPlayback: true),
                ),
              ),
            ),
        ],
      ),
    );
  }
}

class _RealtimeCallControl extends StatelessWidget {
  final IconData icon;
  final String label;
  final bool selected;
  final bool destructive;
  final bool enabled;
  final VoidCallback onTap;

  const _RealtimeCallControl({
    required this.icon,
    required this.label,
    required this.enabled,
    required this.onTap,
    this.selected = false,
    this.destructive = false,
  });

  @override
  Widget build(BuildContext context) {
    final background = destructive
        ? context.error
        : selected
            ? context.accentSoft
            : context.surfaceSecondary;
    final foreground = destructive
        ? Colors.white
        : selected
            ? context.accentPrimary
            : enabled
                ? context.textPrimary
                : context.textTertiary;
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: enabled ? onTap : null,
      child: SizedBox(
        width: 72,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 54,
              height: 54,
              decoration: BoxDecoration(
                color: background,
                shape: BoxShape.circle,
                border: destructive ? null : Border.all(color: context.borderPrimary),
              ),
              alignment: Alignment.center,
              child: Icon(icon, size: 23, color: foreground),
            ),
            const SizedBox(height: 7),
            Text(label, style: AppTypography.label(context).copyWith(color: foreground), textAlign: TextAlign.center),
          ],
        ),
      ),
    );
  }
}
