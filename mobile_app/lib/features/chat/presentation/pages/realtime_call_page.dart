import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/notification_runtime_bootstrap.dart';
import '../../../../core/backend_connection/backend_connection_availability.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/backend_transport/websocket/backend_websocket_client.dart';
import '../../../../core/backend_transport/websocket/backend_websocket_message.dart';
import '../../../../core/backend_transport/websocket/backend_websocket_session.dart';
import '../../../../core/notifications/notification_coordinator.dart';
import '../../../../core/realtime/realtime_audio_bridge.dart';
import '../../../../core/realtime/realtime_voice_activity_detector.dart';
import '../../../../core/realtime/realtime_visual_bridge.dart';
import '../../../../core/ui_runtime/mobile_style_runtime.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/settings/current_character_provider.dart';
import '../../../../core/runtime/backend/mobile_backend_providers.dart';
import '../../../../core/services/device_owned_realtime_service.dart';
import '../../../../core/services/owned_realtime_invitation.dart';
import '../../../../core/services/owned_conversation_reference.dart';
import '../../../../core/services/owned_speech_player.dart';
import '../widgets/realtime_call_layout.dart';

enum RealtimeCallMode { voice, video, screen }

class RealtimeCallPage extends ConsumerStatefulWidget {
  const RealtimeCallPage({
    super.key,
    required this.conversationId,
    required this.characterName,
    this.characterAvatar = '',
    this.initialMode = RealtimeCallMode.voice,
    this.incomingCallId = '',
    this.initialAction = '',
    this.characterId = '',
    this.executionScope,
    this.historicalRoleId = '',
    this.ownedInvitation = false,
    this.voiceType = '',
    this.resourceId = '',
  });

  final String conversationId;
  final String characterName;
  final String characterAvatar;
  final RealtimeCallMode initialMode;
  final String incomingCallId;
  final String initialAction;
  final String characterId;
  final Map<String, dynamic>? executionScope;
  final String historicalRoleId;
  final bool ownedInvitation;
  final String voiceType;
  final String resourceId;

  @override
  ConsumerState<RealtimeCallPage> createState() => _RealtimeCallPageState();
}

class _RealtimeCallPageState extends ConsumerState<RealtimeCallPage> {
  final RealtimeAudioBridge _audio = RealtimeAudioBridge();
  final RealtimeVisualBridge _visual = RealtimeVisualBridge();
  late final NotificationCoordinator _notifications;

  BackendWebSocketClient? _client;
  BackendWebSocketSession? _session;
  DeviceOwnedRealtimeService? _owned;
  String? _ownedRoleAtStart;
  String _ownedTurn = '';
  String _ownedPending = '';
  String _ownedCompleted = '';
  String? _ownedSavedConversation;
  Future<void> _ownedQueue = Future.value();
  Future<void> _ownedIncoming = Future.value();
  Timer? _ownedWatch;
  bool _ownedChecking = false;
  StreamSubscription? _wsSubscription;
  StreamSubscription<Uint8List>? _audioSubscription;
  StreamSubscription<RealtimeVisualFrame>? _visualFrameSubscription;
  StreamSubscription<NotificationCallLifecycleEvent>?
  _callLifecycleSubscription;
  Timer? _durationTimer;
  final RealtimeVoiceActivityDetector _vad = RealtimeVoiceActivityDetector();

  String _state = 'connecting';
  String? _error;
  String? _visionStatus;
  String _liveTranscript = '';
  String _liveReply = '';
  bool _aiSpeaking = false;
  bool _muted = false;
  bool _wakeArmed = false;
  bool _cameraActive = false;
  bool _screenActive = false;
  bool _cameraSupported = true;
  bool _screenSupported = true;
  bool _initialMediaApplied = false;
  bool _awaitingAnswer = false;
  bool _answerAcknowledged = false;
  bool _callEndReported = false;
  int _seconds = 0;
  Uint8List? _latestCameraFrame;
  Uint8List? _latestScreenFrame;
  @override
  void initState() {
    super.initState();
    registerRealtimeCallStyleTargets();
    _notifications = ref.read(notificationCoordinatorProvider);
    _callLifecycleSubscription = _notifications.callLifecycleEvents.listen(
      _handleCallLifecycleEvent,
    );
    final incoming = widget.incomingCallId.trim().isNotEmpty;
    final action = widget.initialAction.trim().toLowerCase();
    _awaitingAnswer = incoming && action == 'incoming';
    if (_awaitingAnswer) {
      _state = 'ringing';
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      if (_awaitingAnswer) {
        setState(() {});
        return;
      }
      if (incoming && action == 'answer') {
        unawaited(_answerIncomingCall());
      } else {
        unawaited(_connect());
      }
    });
  }

  @override
  void dispose() {
    if (widget.incomingCallId.trim().isNotEmpty &&
        !_callEndReported &&
        !_awaitingAnswer) {
      unawaited(_reportIncomingCallEnd('page_disposed'));
    }
    unawaited(_callLifecycleSubscription?.cancel() ?? Future<void>.value());
    unawaited(_shutdown(sendStop: true));
    super.dispose();
  }

  void _handleCallLifecycleEvent(NotificationCallLifecycleEvent event) {
    final callId = widget.incomingCallId.trim();
    if (!mounted || callId.isEmpty || event.callId != callId) return;
    _callEndReported = true;
    _awaitingAnswer = false;
    unawaited(() async {
      await _shutdown(sendStop: false);
      if (mounted) {
        await Navigator.of(context).maybePop();
      }
    }());
  }

  Future<void> _answerIncomingCall() async {
    if (!mounted) return;
    _awaitingAnswer = false;
    setState(() {
      _state = 'connecting';
      _error = null;
    });
    if (!widget.ownedInvitation) await _acknowledgeIncomingCallAnswer();
    await _connect();
  }

  Future<void> _acknowledgeIncomingCallAnswer() async {
    final callId = widget.incomingCallId.trim();
    if (callId.isEmpty || _answerAcknowledged) return;
    _answerAcknowledged = await _notifications.answerIncomingCall(
      callId: callId,
      conversationId: widget.conversationId,
    );
  }

  Future<void> _declineIncomingCall() async {
    await _reportIncomingCallEnd('user_declined');
    if (mounted) Navigator.of(context).pop();
  }

  Future<void> _reportIncomingCallEnd(String reason) async {
    final callId = widget.incomingCallId.trim();
    if (callId.isEmpty || _callEndReported) return;
    _callEndReported = true;
    await _notifications.endIncomingCall(
      callId: callId,
      conversationId: widget.conversationId,
      reason: reason,
    );
  }

  Future<void> _connect() async {
    if ((_state == 'connected' || _state == 'connecting') && _session != null) {
      return;
    }
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
      final api = ref.read(rawBackendServiceApiProvider);
      if (api == null) throw StateError('Core 当前不可用');
      final deployment = ref.read(mobileDeploymentConfigProvider);
      _ownedRoleAtStart = widget.characterId;
      bool current() =>
          mounted &&
          identical(ref.read(rawBackendServiceApiProvider), api) &&
          ref.read(mobileDeploymentConfigProvider) == deployment &&
          ref.read(currentCharacterIdProvider) == _ownedRoleAtStart;
      if (widget.ownedInvitation) {
        final accepted = await acceptOwnedRealtimeInvitation(
          api: api,
          isCurrent: current,
          callId: widget.incomingCallId.trim(),
          characterId: widget.characterId,
        );
        _owned = accepted.service;
        _answerAcknowledged = true;
        await _connectOwned(availability, acceptedTicket: accepted.ticket);
        return;
      }
      final scope = widget.executionScope;
      if (scope == null || widget.characterId.isEmpty) {
        throw StateError('请从已加载的角色对话发起通话，当前缺少完整角色与权限范围');
      }
      _owned = DeviceOwnedRealtimeService(
        api: api,
        isCurrent: current,
        scope: scope,
        characterId: widget.characterId,
        conversationId:
            parseConversationReference(widget.conversationId)?['id'] ??
            widget.conversationId,
        conversationOrigin: parseConversationReference(widget.conversationId),
        historicalRoleId: widget.historicalRoleId,
        voiceType: widget.voiceType,
        resourceId: widget.resourceId,
      );
      await _connectOwned(availability);
    } catch (error) {
      await _shutdown(sendStop: false);
      if (widget.incomingCallId.trim().isNotEmpty && !_awaitingAnswer) {
        unawaited(_reportIncomingCallEnd('connect_failed'));
      }
      _fail(error.toString().replaceFirst('Bad state: ', ''));
    }
  }

  Future<void> _connectOwned(
    BackendConnectionAvailable availability, {
    Map<String, dynamic>? acceptedTicket,
  }) async {
    final owned = _owned!;
    final ticket = acceptedTicket ?? await owned.ticket();
    owned.validateScope(ticket);
    owned.assertConnection();
    try {
      final visual = await _visual.status();
      _cameraSupported = visual.cameraSupported;
      _screenSupported = visual.screenSupported;
    } catch (_) {
      _cameraSupported = false;
      _screenSupported = false;
    }
    final client = BackendWebSocketClient(availability.config);
    _client = client;
    final session = await client.connect(
      ticket['wsPath'] as String,
      queryParameters: {'ticket': ticket['ticket']},
    );
    owned.assertConnection();
    _session = session;
    _wsSubscription = session.messages.listen(
      (message) {
        if (message is WebSocketErrorMessage) {
          _stopOwned(message.error.toString());
          return;
        }
        if (message is! WebSocketTextMessage) return;
        _ownedIncoming = _ownedIncoming.then<void>(
          (_) => _ownedMessage(message.data),
        );
      },
      onError: (Object error) => _stopOwned(error.toString()),
      onDone: () => _stopOwned('Core 通话连接已断开，当前通话已停止'),
    );
    _audioSubscription = _audio.inputPcm.listen((pcm) {
      if (_state != 'connected' || _muted || !identical(_owned, owned)) return;
      final event = _vad.process(pcm);
      if (event == RealtimeVadEvent.speechStart) {
        unawaited(ref.read(ownedSpeechPlayerProvider).stop());
        _aiSpeaking = false;
        if (_ownedPending.isNotEmpty) {
          _queueOwned(const {'type': 'interrupt'});
          if (mounted) setState(() => _visionStatus = '已停止上一轮，请重新开始说话');
          return;
        }
        _ownedTurn = ownedRealtimeRequestId();
        _queueOwned({
          'type': 'turn_start',
          'requestId': _ownedTurn,
          'expectedExecutionScope': owned.scope,
        });
      }
      if (_ownedTurn.isNotEmpty) {
        final bytes = Uint8List.fromList(pcm);
        _ownedQueue = _ownedQueue
            .then<void>((_) async {
              owned.assertConnection();
              await session.send(WebSocketBinaryMessage(bytes));
            })
            .catchError((Object error) => _stopOwned(error.toString()));
      }
      if (event == RealtimeVadEvent.speechEnd && _ownedTurn.isNotEmpty) {
        _ownedPending = _ownedTurn;
        _ownedTurn = '';
        _ownedCompleted = '';
        _queueOwned(const {'type': 'turn_end'});
      }
    }, onError: (Object error) => _stopOwned('麦克风采集失败：$error'));
    _visualFrameSubscription = _visual.frames.listen(_handleVisualFrame);
    _ownedWatch = Timer.periodic(const Duration(seconds: 1), (_) {
      if (_ownedChecking || !identical(_owned, owned)) return;
      _ownedChecking = true;
      unawaited(
        owned
            .assertCurrent()
            .catchError((Object error) => _stopOwned(error.toString()))
            .whenComplete(() => _ownedChecking = false),
      );
    });
    await _audio.startCapture();
  }

  void _queueOwned(Map<String, dynamic> value) {
    final owned = _owned;
    final session = _session;
    if (owned == null || session == null) return;
    _ownedQueue = _ownedQueue
        .then<void>((_) async {
          owned.assertConnection();
          await session.send(WebSocketTextMessage(jsonEncode(value)));
        })
        .catchError((Object error) => _stopOwned(error.toString()));
  }

  void _stopOwned(String message) {
    if (_owned == null) return;
    _fail(message);
    unawaited(_shutdown(sendStop: false));
  }

  Future<void> _ownedMessage(String payload) async {
    final owned = _owned;
    if (owned == null) return;
    try {
      final decoded = jsonDecode(payload);
      if (decoded is! Map) throw StateError('实时通话响应无效');
      owned.assertConnection();
      switch (decoded['type']) {
        case 'ready':
          owned.validateScope(decoded);
          if (mounted) setState(() => _state = 'connected');
          _durationTimer ??= Timer.periodic(const Duration(seconds: 1), (_) {
            if (mounted && _state == 'connected') setState(() => _seconds++);
          });
          await _applyInitialMediaMode();
        case 'asr_final':
          final data = decoded['data'];
          if (decoded['requestId'] != _ownedPending || data is! Map) {
            return;
          }
          final transcript = data['transcript']?.toString().trim() ?? '';
          if (transcript.isEmpty) return;
          final eventId = data['eventId']?.toString().trim() ?? '';
          if (mounted) {
            setState(() {
              _liveTranscript = transcript;
              _liveReply = '';
            });
          }
          if (eventId.isNotEmpty) {
            unawaited(
              _audio.emitWorkflowAsrFinal(
                transcript: transcript,
                eventId: eventId,
                sessionId: data['turnId']?.toString(),
                conversationId: data['conversationId']?.toString(),
              ),
            );
          }
        case 'assistant_text':
          final data = decoded['data'];
          if (decoded['requestId'] != _ownedPending || data is! Map) {
            return;
          }
          final text = data['text']?.toString() ?? '';
          if (text.isEmpty) return;
          if (mounted) {
            setState(
              () => _liveReply = data['delta'] == true
                  ? '$_liveReply$text'
                  : text,
            );
          }
        case 'vision_status':
          if (mounted) {
            setState(() => _visionStatus = '视觉已更新');
          }
        case 'wake_status':
          final armed = decoded['armed'] == true;
          if (mounted)
            setState(() {
              _wakeArmed = armed;
              _visionStatus = armed
                  ? '唤醒已开启'
                  : decoded['message']?.toString() ?? '唤醒已关闭';
            });
        case 'wake_detected':
          if (mounted)
            setState(() {
              _wakeArmed = false;
              _visionStatus = '已检测到唤醒词';
            });
        case 'playback_status':
        case 'tts_ended':
          break;
        case 'event':
          if (decoded['requestId'] != _ownedPending ||
              decoded['data'] is! Map) {
            return;
          }
          owned.validateScope(decoded['data'] as Map);
        case 'completed':
          if (decoded['requestId'] != _ownedPending ||
              decoded['data'] is! Map) {
            return;
          }
          owned.validateCompleted(decoded['data'] as Map, _ownedPending);
          if (mounted)
            setState(
              () => _liveReply =
                  ((decoded['data'] as Map)['reply']?.toString() ?? '').trim(),
            );
          _ownedSavedConversation = conversationReference({
            'ownerId': owned.scope['resourceOwnerId'],
            'id': (decoded['data'] as Map)['conversationId'],
          });
          _ownedCompleted = _ownedPending;
        case 'audio':
          final request = _ownedPending;
          if (request.isEmpty ||
              decoded['requestId'] != request ||
              _ownedCompleted != request ||
              decoded['data'] is! Map) {
            return;
          }
          final audio = await owned.audio(
            Map<String, dynamic>.from(decoded['data'] as Map),
            request,
          );
          owned.assertConnection();
          if (_ownedPending != request || !identical(_owned, owned)) return;
          await ref
              .read(ownedSpeechPlayerProvider)
              .play(
                audio,
                onProgress: (played, received) => _queueOwned({
                  'type': 'playback_progress',
                  'requestId': request,
                  'generationId': 1,
                  'played_audio_ms': played,
                  'received_audio_ms': received,
                }),
                onEnded: () => _queueOwned({
                  'type': 'tts_playback_ended',
                  'requestId': request,
                  'generationId': 1,
                }),
              );
          owned.assertConnection();
          if (mounted) setState(() => _aiSpeaking = true);
        case 'turn_ready':
          if (decoded['requestId'] == _ownedPending) {
            _ownedPending = '';
          }
        case 'error':
          if (decoded['requestId'] != null &&
              decoded['requestId'] != _ownedPending) {
            return;
          }
          final saved = decoded['saved'] == true;
          _ownedCompleted = '';
          if (mounted) {
            amitiaSnackBar(
              context,
              '${saved ? '对话已保存，朗读失败：' : '本轮通话未完成：'}${decoded['message'] ?? 'Core 服务异常'}',
            );
          }
      }
    } catch (error) {
      _stopOwned(error.toString());
    }
  }

  void _handleVisualFrame(RealtimeVisualFrame frame) {
    if (mounted) {
      setState(() {
        if (frame.source == 'camera') _latestCameraFrame = frame.bytes;
        if (frame.source == 'screen') _latestScreenFrame = frame.bytes;
      });
    }
    final owned = _owned;
    if (owned != null) {
      if (_state == 'connected' && _ownedTurn.isNotEmpty) {
        try {
          _queueOwned({
            'type': 'visual',
            'attachment': owned.image(frame.bytes, frame.mime),
          });
        } catch (error) {
          if (mounted) setState(() => _visionStatus = error.toString());
        }
      }
      return;
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
    final owned = _owned;
    _owned = null;
    owned?.close();
    _ownedWatch?.cancel();
    _ownedWatch = null;
    _ownedTurn = '';
    _ownedPending = '';
    if (owned != null) await ref.read(ownedSpeechPlayerProvider).stop();
    _durationTimer?.cancel();
    _durationTimer = null;
    _vad.reset();
    final session = _session;
    _session = null;
    if (sendStop) {
      try {
        await session?.send(
          WebSocketTextMessage(jsonEncode(const {'type': 'stop'})),
        );
      } catch (_) {}
    }
    await _audioSubscription?.cancel();
    _audioSubscription = null;
    await _visualFrameSubscription?.cancel();
    _visualFrameSubscription = null;
    await _wsSubscription?.cancel();
    _wsSubscription = null;
    try {
      await _audio.reset();
    } catch (_) {}
    try {
      await _visual.reset();
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
    _wakeArmed = false;
    _cameraActive = false;
    _screenActive = false;
    _initialMediaApplied = false;
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
        setState(
          () => _visionStatus =
              '初始媒体采集失败：${error.toString().replaceFirst('Bad state: ', '')}',
        );
      }
    }
  }

  Future<void> _toggleMute() async {
    if (_state != 'connected') return;
    try {
      if (_muted) {
        await _audio.startCapture();
      } else {
        if (_vad.isSpeechActive && _ownedTurn.isNotEmpty) {
          _ownedPending = _ownedTurn;
          _ownedTurn = '';
          _queueOwned(const {'type': 'turn_end'});
        }
        _vad.reset();
        await _audio.stopCapture();
      }
      if (mounted) setState(() => _muted = !_muted);
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
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '摄像头切换失败：$error');
    }
  }

  void _toggleWake() {
    if (_state != 'connected') return;
    _queueOwned(
      _wakeArmed ? const {'type': 'wake_disarm'} : const {'type': 'wake_arm'},
    );
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
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '屏幕共享切换失败：$error');
    }
  }

  Future<void> _endCall() async {
    if (_awaitingAnswer) {
      await _declineIncomingCall();
      return;
    }
    if (widget.incomingCallId.trim().isNotEmpty) {
      await _reportIncomingCallEnd('user_ended');
    }
    await _shutdown(sendStop: true);
    if (mounted) Navigator.of(context).pop(_ownedSavedConversation);
  }

  String get _duration {
    final minutes = (_seconds ~/ 60).toString().padLeft(2, '0');
    final seconds = (_seconds % 60).toString().padLeft(2, '0');
    return '$minutes:$seconds';
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(rawBackendServiceApiProvider, (_, __) {
      if (_owned != null) _stopOwned('Core 连接已切换，实时通话已中断');
    });
    ref.listen(mobileDeploymentConfigProvider, (_, __) {
      if (_owned != null) _stopOwned('设备服务模式已变化，实时通话已中断');
    });
    ref.listen(currentCharacterIdProvider, (_, next) {
      if (_owned != null && next != _ownedRoleAtStart) {
        _stopOwned('角色已切换，实时通话已中断');
      }
    });
    final name = widget.characterName.trim().isEmpty
        ? 'Amitia'
        : widget.characterName.trim();
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
    final statusText = _awaitingAnswer
        ? '邀请你进行$callMode'
        : _state == 'connecting'
        ? '正在连接实时通话…'
        : connected
        ? (_aiSpeaking ? '对方正在说话' : (_muted ? '麦克风已静音' : '$callMode中'))
        : _state == 'error'
        ? (_error ?? '连接失败')
        : '通话已结束';
    final detailText = (_error ?? '').trim().isNotEmpty
        ? _error!.trim()
        : (_visionStatus ?? '').trim();
    final systemPadding = MediaQuery.paddingOf(context);
    final primaryFrame = _screenActive
        ? _latestScreenFrame
        : _cameraActive
        ? _latestCameraFrame
        : null;

    return AmitiaStyleTarget(
      target: 'realtime.call.page',
      child: const SizedBox.shrink(),
      builder: (context, pageStyle) {
        final pageBackground =
            pageStyle.color('backgroundColor') ??
            pageStyle.color('background') ??
            const Color(0xFF121212);
        final pagePadding = pageStyle.edgeInsets('padding');
        final pageOpacity = pageStyle.number('opacity');
        final page = Scaffold(
          backgroundColor: pageBackground,
          body: Padding(
            padding: pagePadding ?? EdgeInsets.zero,
            child: RealtimeCallLayout(
              background: AmitiaStyleTarget(
                target: 'realtime.call.background',
                child: const SizedBox.shrink(),
                builder: (context, style) =>
                    _buildBackground(initial, primaryFrame, style),
              ),
              overlays: [
                if (_screenActive &&
                    _cameraActive &&
                    _latestCameraFrame != null)
                  Positioned(
                    right: 16,
                    top: systemPadding.top + 96,
                    width: 104,
                    height: 140,
                    child: ClipRRect(
                      borderRadius: BorderRadius.circular(16),
                      child: DecoratedBox(
                        decoration: BoxDecoration(
                          color: Colors.black,
                          border: Border.all(color: Colors.white24),
                        ),
                        child: Image.memory(
                          _latestCameraFrame!,
                          fit: BoxFit.cover,
                          gaplessPlayback: true,
                        ),
                      ),
                    ),
                  ),
              ],
              details: AmitiaStyleTarget(
                target: 'realtime.call.details',
                child: const SizedBox.shrink(),
                builder: (context, style) => _styleRealtimeSurface(
                  style: style,
                  child: _buildCenterSection(
                    name: name,
                    initial: initial,
                    callMode: callMode,
                    statusText: statusText,
                    detailText: detailText,
                    style: style,
                  ),
                ),
              ),
              controls: AmitiaStyleTarget(
                target: 'realtime.call.control',
                child: const SizedBox.shrink(),
                builder: (context, style) => _styleRealtimeSurface(
                  style: style,
                  child: _buildControlBar(connected, style),
                ),
              ),
            ),
          ),
        );
        return pageOpacity == null
            ? page
            : Opacity(
                opacity: pageOpacity.toDouble().clamp(0.0, 1.0).toDouble(),
                child: page,
              );
      },
    );
  }

  Widget _styleRealtimeSurface({
    required AmitiaResolvedStyle style,
    required Widget child,
  }) {
    final background =
        style.color('backgroundColor') ?? style.color('background');
    final borderSide = style.borderSide();
    final radius = style.borderRadius('borderRadius');
    final shadow = style.boxShadow();
    final padding = style.edgeInsets('padding');
    final margin = style.edgeInsets('margin');
    final opacity = style.number('opacity');
    Widget surface = Container(
      margin: margin,
      padding: padding,
      decoration:
          background == null &&
              borderSide == null &&
              radius == null &&
              shadow == null
          ? null
          : BoxDecoration(
              color: background,
              border: borderSide == null
                  ? null
                  : Border.fromBorderSide(borderSide),
              borderRadius: radius,
              boxShadow: shadow,
            ),
      child: child,
    );
    if (opacity != null) {
      surface = Opacity(
        opacity: opacity.toDouble().clamp(0.0, 1.0).toDouble(),
        child: surface,
      );
    }
    return surface;
  }

  Widget _buildBackground(
    String initial,
    Uint8List? primaryFrame,
    AmitiaResolvedStyle style,
  ) {
    final background =
        style.color('backgroundColor') ??
        style.color('background') ??
        const Color(0xFF121212);
    final radius = style.borderRadius('borderRadius');
    final borderSide = style.borderSide();
    final opacity = style.number('opacity');
    Widget content = Stack(
      fit: StackFit.expand,
      clipBehavior: Clip.hardEdge,
      children: [
        ColoredBox(color: background),
        Opacity(
          opacity: 0.72,
          child: ImageFiltered(
            imageFilter: ui.ImageFilter.blur(sigmaX: 36, sigmaY: 36),
            child: Transform.scale(
              scale: 1.24,
              child: _buildAvatarImage(initial),
            ),
          ),
        ),
        if (primaryFrame != null)
          ColoredBox(
            color: Colors.black,
            child: Image.memory(
              primaryFrame,
              fit: _screenActive ? BoxFit.contain : BoxFit.cover,
              gaplessPlayback: true,
            ),
          ),
        const DecoratedBox(
          decoration: BoxDecoration(
            gradient: LinearGradient(
              begin: Alignment.topCenter,
              end: Alignment.bottomCenter,
              colors: [Color(0x4D000000), Color(0x73000000), Color(0xA6000000)],
              stops: [0, 0.55, 1],
            ),
          ),
        ),
      ],
    );
    if (radius != null) {
      content = ClipRRect(borderRadius: radius, child: content);
    }
    if (borderSide != null) {
      content = DecoratedBox(
        decoration: BoxDecoration(
          borderRadius: radius,
          border: Border.fromBorderSide(borderSide),
        ),
        child: content,
      );
    }
    if (opacity != null) {
      content = Opacity(
        opacity: opacity.toDouble().clamp(0.0, 1.0).toDouble(),
        child: content,
      );
    }
    return content;
  }

  Widget _buildAvatarImage(String initial) {
    final avatar = widget.characterAvatar.trim();
    final fallback = Container(
      color: const Color(0xFF2B2B2D),
      alignment: Alignment.center,
      child: Text(
        initial,
        style: const TextStyle(
          color: Colors.white,
          fontSize: 34,
          fontWeight: FontWeight.w700,
        ),
      ),
    );
    if (avatar.isEmpty) return fallback;
    return Image.network(
      avatar,
      fit: BoxFit.cover,
      errorBuilder: (_, __, ___) => fallback,
    );
  }

  Widget _buildCenterSection({
    required String name,
    required String initial,
    required String callMode,
    required String statusText,
    required String detailText,
    required AmitiaResolvedStyle style,
  }) {
    final stateLabel = _state == 'connecting'
        ? '正在建立通话'
        : _state == 'error'
        ? '通话连接失败'
        : _state == 'connected'
        ? '通话中'
        : '通话已结束';
    final detail = detailText.isNotEmpty
        ? detailText
        : _state == 'connected'
        ? statusText
        : callMode;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        AnimatedContainer(
          duration: const Duration(milliseconds: 180),
          width: 96,
          height: 96,
          padding: const EdgeInsets.all(2),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(24),
            border: Border.all(
              color: _aiSpeaking
                  ? style.color('foregroundColor') ??
                        style.color('textColor') ??
                        Colors.white
                  : style.color('borderColor') ?? Colors.white24,
              width: _aiSpeaking ? 3 : 1,
            ),
          ),
          child: ClipRRect(
            borderRadius: BorderRadius.circular(21),
            child: _buildAvatarImage(initial),
          ),
        ),
        const SizedBox(height: 16),
        Text(
          name,
          textAlign: TextAlign.center,
          style: style.applyTextStyle(
            const TextStyle(
              color: Colors.white,
              fontSize: 21,
              fontWeight: FontWeight.w600,
              fontVariations: <ui.FontVariation>[ui.FontVariation('wght', 650)],
            ),
          ),
        ),
        const SizedBox(height: 7),
        Text(
          '$stateLabel · $_duration · AI 实时语音',
          textAlign: TextAlign.center,
          style: style.applyTextStyle(
            const TextStyle(
              color: Color(0xFFBCBCC0),
              fontSize: 13,
              fontWeight: FontWeight.w400,
            ),
          ),
        ),
        const SizedBox(height: 14),
        ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 290),
          child: Text(
            detail,
            textAlign: TextAlign.center,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: style.applyTextStyle(
              const TextStyle(
                color: Color(0xFFBCBCC0),
                fontSize: 14,
                fontWeight: FontWeight.w400,
              ),
            ),
          ),
        ),
        if (_liveTranscript.isNotEmpty) ...[
          const SizedBox(height: 12),
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 290),
            child: Text(
              '你：$_liveTranscript',
              textAlign: TextAlign.center,
              maxLines: 3,
              overflow: TextOverflow.ellipsis,
              style: style.applyTextStyle(
                const TextStyle(color: Color(0xFFD5D5D8), fontSize: 13),
              ),
            ),
          ),
        ],
        if (_liveReply.isNotEmpty) ...[
          const SizedBox(height: 8),
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 290),
            child: Text(
              'AI：$_liveReply',
              textAlign: TextAlign.center,
              maxLines: 4,
              overflow: TextOverflow.ellipsis,
              style: style.applyTextStyle(
                const TextStyle(color: Colors.white, fontSize: 13),
              ),
            ),
          ),
        ],
        if (_state == 'error' && detailText.isNotEmpty)
          RealtimeCallErrorDetails(message: detailText),
      ],
    );
  }

  Widget _buildControlBar(bool connected, AmitiaResolvedStyle style) {
    final labelStyle = style.applyTextStyle(
      const TextStyle(
        color: Color(0xFFC3C3C6),
        fontSize: 12,
        fontWeight: FontWeight.w400,
      ),
    );
    if (_awaitingAnswer) {
      return Row(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: _RealtimeCallControl(
              icon: Icons.call_end,
              label: '拒绝',
              destructive: true,
              enabled: true,
              labelStyle: labelStyle,
              onTap: _declineIncomingCall,
            ),
          ),
          const SizedBox(width: 24),
          Expanded(
            child: _RealtimeCallControl(
              icon: Icons.call,
              label: '接听',
              selected: true,
              enabled: true,
              labelStyle: labelStyle,
              onTap: _answerIncomingCall,
            ),
          ),
        ],
      );
    }
    if (_state == 'error') return _buildErrorControls(labelStyle);
    final modeControl = widget.initialMode == RealtimeCallMode.screen
        ? _RealtimeCallControl(
            icon: _screenActive
                ? Icons.stop_screen_share_outlined
                : Icons.screen_share_outlined,
            label: _screenActive ? '停止共享' : '共享屏幕',
            selected: _screenActive,
            enabled: connected && _screenSupported,
            labelStyle: labelStyle,
            onTap: _toggleScreen,
          )
        : _RealtimeCallControl(
            icon: _cameraActive
                ? Icons.videocam_off_outlined
                : Icons.videocam_outlined,
            label: _cameraActive ? '关闭视频' : '视频',
            selected: _cameraActive,
            enabled: connected && _cameraSupported,
            labelStyle: labelStyle,
            onTap: _toggleCamera,
          );
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: _RealtimeCallControl(
            icon: _muted ? Icons.mic_off_outlined : Icons.mic_none_outlined,
            label: _muted ? '取消静音' : '静音',
            selected: _muted,
            enabled: connected,
            labelStyle: labelStyle,
            onTap: _toggleMute,
          ),
        ),
        const SizedBox(width: 16),
        Expanded(
          child: _RealtimeCallControl(
            icon: _wakeArmed
                ? Icons.notifications_active_outlined
                : Icons.notifications_none_outlined,
            label: _wakeArmed ? '关闭唤醒' : '开启唤醒',
            selected: _wakeArmed,
            enabled: connected,
            labelStyle: labelStyle,
            onTap: _toggleWake,
          ),
        ),
        const SizedBox(width: 16),
        Expanded(child: modeControl),
        const SizedBox(width: 16),
        Expanded(
          child: _RealtimeCallControl(
            icon: Icons.call_end,
            label: '挂断',
            destructive: true,
            enabled: true,
            labelStyle: labelStyle,
            onTap: _endCall,
          ),
        ),
      ],
    );
  }

  Widget _buildErrorControls(TextStyle labelStyle) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: _RealtimeCallControl(
            icon: Icons.refresh,
            label: '重试',
            enabled: true,
            labelStyle: labelStyle,
            onTap: () async {
              await _shutdown(sendStop: false);
              await _connect();
            },
          ),
        ),
        const SizedBox(width: 16),
        Expanded(
          child: _RealtimeCallControl(
            icon: Icons.call_end,
            label: '挂断',
            destructive: true,
            enabled: true,
            labelStyle: labelStyle,
            onTap: _endCall,
          ),
        ),
        const SizedBox(width: 16),
        Expanded(
          child: _RealtimeCallControl(
            icon: Icons.close,
            label: '关闭',
            enabled: true,
            labelStyle: labelStyle,
            onTap: _endCall,
          ),
        ),
      ],
    );
  }
}

class _RealtimeCallControl extends StatelessWidget {
  final IconData icon;
  final String label;
  final bool selected;
  final bool destructive;
  final bool enabled;
  final TextStyle? labelStyle;
  final VoidCallback onTap;

  const _RealtimeCallControl({
    required this.icon,
    required this.label,
    required this.enabled,
    required this.onTap,
    this.selected = false,
    this.destructive = false,
    this.labelStyle,
  });

  @override
  Widget build(BuildContext context) {
    final background = destructive
        ? const Color(0xFFFF453A)
        : selected
        ? Colors.white
        : const Color(0xFF2B2B2D);
    final foreground = destructive
        ? Colors.white
        : selected
        ? const Color(0xFF111111)
        : enabled
        ? Colors.white
        : Colors.white38;
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: enabled ? onTap : null,
      child: SizedBox(
        width: 66,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 58,
              height: 58,
              decoration: BoxDecoration(
                color: background,
                shape: BoxShape.circle,
              ),
              alignment: Alignment.center,
              child: Icon(icon, size: 25, color: foreground),
            ),
            const SizedBox(height: 7),
            Text(
              label,
              style:
                  labelStyle ??
                  const TextStyle(
                    color: Color(0xFFC3C3C6),
                    fontSize: 12,
                    fontWeight: FontWeight.w400,
                  ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }
}

void registerRealtimeCallStyleTargets() {
  AmitiaStyleTargetRegistry.register(
    const AmitiaStyleTargetDescriptor(
      target: 'realtime.call.page',
      properties: <String, String>{
        'backgroundColor': 'color',
        'background': 'color',
        'padding': 'insets',
        'opacity': 'number',
      },
    ),
  );
  AmitiaStyleTargetRegistry.register(
    const AmitiaStyleTargetDescriptor(
      target: 'realtime.call.background',
      properties: <String, String>{
        'backgroundColor': 'color',
        'background': 'color',
        'borderColor': 'color',
        'borderWidth': 'number',
        'borderStyle': 'string',
        'borderRadius': 'length',
        'opacity': 'number',
      },
    ),
  );
  AmitiaStyleTargetRegistry.register(
    const AmitiaStyleTargetDescriptor(
      target: 'realtime.call.details',
      properties: <String, String>{
        'backgroundColor': 'color',
        'background': 'color',
        'foregroundColor': 'color',
        'textColor': 'color',
        'borderColor': 'color',
        'borderWidth': 'number',
        'borderStyle': 'string',
        'borderRadius': 'length',
        'shadowColor': 'color',
        'boxShadow': 'shadow',
        'padding': 'insets',
        'margin': 'insets',
        'fontSize': 'number',
        'fontWeight': 'number',
        'opacity': 'number',
      },
    ),
  );
  AmitiaStyleTargetRegistry.register(
    const AmitiaStyleTargetDescriptor(
      target: 'realtime.call.control',
      properties: <String, String>{
        'backgroundColor': 'color',
        'background': 'color',
        'foregroundColor': 'color',
        'textColor': 'color',
        'borderColor': 'color',
        'borderWidth': 'number',
        'borderStyle': 'string',
        'borderRadius': 'length',
        'shadowColor': 'color',
        'boxShadow': 'shadow',
        'padding': 'insets',
        'margin': 'insets',
        'fontSize': 'number',
        'fontWeight': 'number',
        'opacity': 'number',
      },
    ),
  );
}
