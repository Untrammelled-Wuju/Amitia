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
import '../../../../core/models/voice.dart';
import '../../../../core/realtime/realtime_audio_bridge.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';

class RealtimeVoiceCallSheet extends ConsumerStatefulWidget {
  const RealtimeVoiceCallSheet({
    super.key,
    required this.conversationId,
    required this.characterName,
  });

  final String conversationId;
  final String characterName;

  @override
  ConsumerState<RealtimeVoiceCallSheet> createState() =>
      _RealtimeVoiceCallSheetState();
}

class _RealtimeVoiceCallSheetState
    extends ConsumerState<RealtimeVoiceCallSheet> {
  final RealtimeAudioBridge _audio = RealtimeAudioBridge();

  BackendWebSocketClient? _client;
  BackendWebSocketSession? _session;
  StreamSubscription? _wsSubscription;
  StreamSubscription<Uint8List>? _audioSubscription;
  Timer? _durationTimer;

  String _state = 'connecting';
  String? _error;
  bool _aiSpeaking = false;
  bool _muted = false;
  int _seconds = 0;
  String? _dialogId;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _connect());
  }

  @override
  void dispose() {
    _shutdown(sendStop: true);
    super.dispose();
  }

  Future<void> _connect() async {
    if (_state == 'connected' || _state == 'connecting') {
      // Initial call enters with connecting; only continue if no session exists.
      if (_session != null) return;
    }
    if (mounted) {
      setState(() {
        _state = 'connecting';
        _error = null;
        _seconds = 0;
      });
    }

    try {
      final configs = await ref.read(ttsServiceProvider).listConfigs();
      final voice = _activeVoice(configs);
      if (voice == null) {
        throw StateError('请先在角色语音设置中创建并启用 TTS 配置');
      }
      if (voice.realtimeApiKey.trim().isEmpty) {
        throw StateError('当前 TTS 配置缺少实时语音 Access Token/API Key');
      }

      final availability = await ref.read(backendConnectionProvider.future);
      if (availability is! BackendConnectionAvailable) {
        throw StateError('后端当前不可用');
      }

      final client = BackendWebSocketClient(availability.config);
      final session = await client.connect(
        '/api/realtime/session',
        queryParameters: <String, dynamic>{
          'apiKey': voice.realtimeApiKey,
          if (voice.realtimeAppId.trim().isNotEmpty)
            'appId': voice.realtimeAppId.trim(),
          if (voice.voiceId.trim().isNotEmpty)
            'voiceType': voice.voiceId.trim(),
          if (voice.resourceId.trim().isNotEmpty)
            'resourceId': voice.resourceId.trim(),
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
          unawaited(
            active.send(
              WebSocketTextMessage(
                jsonEncode(<String, dynamic>{
                  'event': 'audio',
                  'data': base64Encode(pcm),
                }),
              ),
            ),
          );
        },
        onError: (Object error) => _fail('麦克风采集失败：$error'),
      );
      await _audio.startCapture();
    } catch (error) {
      await _shutdown(sendStop: false);
      _fail(error.toString().replaceFirst('Bad state: ', ''));
    }
  }

  VoiceConfigDto? _activeVoice(List<VoiceConfigDto> configs) {
    if (configs.isEmpty) return null;
    for (final config in configs) {
      if (config.isActive == 1) return config;
    }
    return configs.first;
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
          if (!mounted) return;
          setState(() => _state = 'connected');
          _durationTimer ??= Timer.periodic(const Duration(seconds: 1), (_) {
            if (mounted && _state == 'connected') {
              setState(() => _seconds++);
            }
          });
        case 'audio':
          final payload = decoded['data']?.toString() ?? '';
          if (payload.isEmpty) return;
          _aiSpeaking = true;
          final pcm = base64Decode(payload);
          unawaited(_audio.playPcm(Uint8List.fromList(pcm)));
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
            ),
          );
        case 'SessionFinished':
        case 'disconnected':
          unawaited(_shutdown(sendStop: false));
          if (mounted) setState(() => _state = 'idle');
        case 'error':
          _fail(decoded['data']?.toString() ?? '实时语音连接失败');
      }
    } catch (_) {
      // Ignore non-JSON/unknown compatibility frames.
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
    _session = null;
    if (sendStop && session != null) {
      try {
        await session.send(
          WebSocketTextMessage(jsonEncode(const {'event': 'stop'})),
        );
      } catch (_) {}
    }
    await _audioSubscription?.cancel();
    _audioSubscription = null;
    await _wsSubscription?.cancel();
    _wsSubscription = null;
    try {
      await _audio.reset();
    } catch (_) {}
    try {
      await session?.close();
    } catch (_) {}
    final client = _client;
    _client = null;
    try {
      await client?.close();
    } catch (_) {}
    _aiSpeaking = false;
    _muted = false;
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
    } catch (error) {
      if (mounted) {
        amitiaSnackBar(context, '麦克风切换失败：$error');
      }
    }
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

  @override
  Widget build(BuildContext context) {
    final name = widget.characterName.trim().isEmpty
        ? 'Amitia'
        : widget.characterName.trim();
    final initial = name.characters.first;
    final connected = _state == 'connected';
    final statusText = _state == 'connecting'
        ? '正在连接实时语音…'
        : connected
            ? (_aiSpeaking ? '对方正在说话' : (_muted ? '麦克风已静音' : '通话中'))
            : _state == 'error'
                ? (_error ?? '连接失败')
                : '通话已结束';

    return SafeArea(
      top: false,
      child: SizedBox(
        height: MediaQuery.sizeOf(context).height * 0.72,
        child: Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.lg,
            AppSpacing.md,
            AppSpacing.lg,
            AppSpacing.lg,
          ),
          child: Column(
            children: [
              Container(
                width: 40,
                height: 4,
                decoration: BoxDecoration(
                  color: context.borderPrimary,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
              const SizedBox(height: 18),
              Align(
                alignment: Alignment.centerLeft,
                child: Text(
                  '语音通话',
                  style: AppTypography.bodySmall(context).copyWith(
                    color: context.textSecondary,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              const Spacer(),
              AnimatedContainer(
                duration: const Duration(milliseconds: 180),
                width: 96,
                height: 96,
                decoration: BoxDecoration(
                  color: _state == 'error'
                      ? context.error.withValues(alpha: 0.12)
                      : context.accentPrimary,
                  borderRadius: BorderRadius.circular(30),
                  border: Border.all(
                    color: _aiSpeaking
                        ? context.accentPrimary
                        : context.borderPrimary,
                    width: _aiSpeaking ? 4 : 1,
                  ),
                  boxShadow: _aiSpeaking
                      ? [
                          BoxShadow(
                            color: context.accentPrimary.withValues(alpha: 0.20),
                            blurRadius: 24,
                            spreadRadius: 3,
                          ),
                        ]
                      : null,
                ),
                alignment: Alignment.center,
                child: _state == 'error'
                    ? Icon(Icons.error_outline, size: 38, color: context.error)
                    : Text(
                        initial,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 30,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
              ),
              const SizedBox(height: 18),
              Text(name, style: AppTypography.pageTitle(context)),
              const SizedBox(height: 7),
              Text(
                connected ? '$statusText · $_duration' : statusText,
                textAlign: TextAlign.center,
                style: AppTypography.bodySmall(context).copyWith(
                  color: _state == 'error' ? context.error : context.textSecondary,
                ),
              ),
              const Spacer(),
              if (_state == 'error')
                Row(
                  children: [
                    Expanded(
                      child: AmitiaButton(
                        label: '关闭',
                        isSecondary: true,
                        onPressed: () => Navigator.of(context).pop(),
                      ),
                    ),
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
                Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    _RealtimeCallControl(
                      icon: _muted ? Icons.mic_off_outlined : Icons.mic_none_outlined,
                      label: _muted ? '取消静音' : '静音',
                      selected: _muted,
                      enabled: connected,
                      onTap: _toggleMute,
                    ),
                    const SizedBox(width: 42),
                    _RealtimeCallControl(
                      icon: Icons.call_end,
                      label: '结束',
                      destructive: true,
                      enabled: true,
                      onTap: _endCall,
                    ),
                  ],
                ),
              const SizedBox(height: 8),
            ],
          ),
        ),
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
        width: 76,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: background,
                shape: BoxShape.circle,
                border: destructive ? null : Border.all(color: context.borderPrimary),
              ),
              alignment: Alignment.center,
              child: Icon(icon, size: 24, color: foreground),
            ),
            const SizedBox(height: 8),
            Text(
              label,
              style: AppTypography.label(context).copyWith(color: foreground),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }
}

