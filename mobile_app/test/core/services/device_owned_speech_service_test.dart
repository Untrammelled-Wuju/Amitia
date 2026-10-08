import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/device_owned_speech_service.dart';
import 'package:amitia_app/core/services/owned_speech_player.dart';
import 'package:amitia_app/core/services/voice_service.dart';
import 'package:crypto/crypto.dart';
import 'package:flutter_test/flutter_test.dart';

class _Api extends Fake implements BackendServiceApi {
  @override
  int generation = 1;
  String core = 'core-b';
  bool saved = true;
  bool validHash = true;
  bool foreignAck = false;
  bool lateSwitch = false;
  bool missingScope = false;
  int permission = 1;
  int writes = 0;
  Completer<void>? policyGate;
  Map<String, dynamic>? submitted;
  final scope = <String, dynamic>{
    'spaceId': 'space-a',
    'initiatorDeviceId': 'device-a',
    'targetDeviceId': 'device-a',
    'coreId': 'core-b',
    'providerEpoch': 1,
    'targetProviderEpoch': 1,
    'coordinated': false,
    'modeRevision': 1,
    'permissionRevision': 1,
    'targetPermissionRevision': 1,
    'roleId': 'role-a',
    'roleRevision': 1,
    'authorizationRealm': 'core-b',
    'roleOwnerId': 'device-a',
    'resourceOwnerId': 'device-a',
    'requestId': 'data-query',
  };

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    dynamic data;
    if (path.endsWith('/coordination/me')) {
      await policyGate?.future;
      data = {
        'coreId': core,
        'coordinationAvailable': true,
        'policy': {
          'deviceId': 'device-a',
          'coordinated': scope['coordinated'],
          'providerEpoch': 1,
          'modeRevision': 1,
          'permissionRevision': permission,
        },
      };
    } else if (path.endsWith('/business/roles')) {
      data = {
        'roles': [
          {'id': 'role-a', 'revision': 1},
        ],
        'roleOwnerId': scope['roleOwnerId'],
        'executionScope': {...scope},
      };
    } else {
      data = {
        'executionScope': missingScope ? null : {...scope},
        'snapshot': {'resources': []},
      };
    }
    return fromJson != null ? fromJson(data) : data as T;
  }

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    expect(path, '/api/device-mesh/v1/business/speech');
    writes++;
    submitted = Map<String, dynamic>.from(data as Map);
    expect(submitted!.containsKey('speakerId'), false);
    expect(submitted!.containsKey('voiceConfigId'), false);
    final id = submitted!['requestId'] as String;
    final digest = sha256
        .convert(utf8.encode('core-b\u0000device-a\u0000$id'))
        .toString();
    final resource = 'speech/${digest.substring(0, 32)}';
    final bytes = [1, 2, 3, 4];
    final result = {
      'requestId': id,
      'saved': saved,
      'executionScope': {
        ...scope,
        'requestId': 'speech/$id',
        'turnId': 'speech-turn',
        'executionId': 'speech-execution',
      },
      'audio': {
        'mime': 'audio/mpeg',
        'data': base64Encode(bytes),
        'sha256': validHash ? sha256.convert(bytes).toString() : '0' * 64,
      },
      'acknowledgement': {
        'ownerId': foreignAck ? 'other-owner' : scope['resourceOwnerId'],
        'requestId': 'speech/$id',
        'versions': {'tool-result/$resource': 1, 'checkpoint/$resource': 2},
      },
    };
    if (lateSwitch) core = 'core-c';
    return fromJson != null ? fromJson(result) : result as T;
  }
}

void main() {
  DeviceOwnedSpeechService service(_Api api) => DeviceOwnedSpeechService(
    api: api,
    currentApi: () => api,
    providerKey: () => 'https://core-b.example',
  );

  test('普通 OFF 与 ON 设备均通过角色 scope 和 owner ACK 获取音频', () async {
    for (final coordinated in [false, true]) {
      final api = _Api();
      api.scope['coordinated'] = coordinated;
      api.scope['roleOwnerId'] = coordinated ? 'core-b' : 'device-a';
      api.scope['resourceOwnerId'] = api.scope['roleOwnerId'];
      final audio = await service(api).synthesize('role-a', '你好');
      expect(audio.bytes, [1, 2, 3, 4]);
      expect(
        audio.executionScope['resourceOwnerId'],
        api.scope['resourceOwnerId'],
      );
      expect(api.writes, 1);
      expect(api.submitted!['expectedExecutionScope'], api.scope);
    }
  });

  test('未 saved、错误 owner ACK、音频 hash 损坏均不可播放', () async {
    for (final modify in <void Function(_Api)>[
      (api) => api.saved = false,
      (api) => api.foreignAck = true,
      (api) => api.validHash = false,
      (api) => api.lateSwitch = true,
    ]) {
      final api = _Api();
      modify(api);
      await expectLater(
        service(api).synthesize('role-a', '你好'),
        throwsStateError,
      );
    }
  });

  test('缺少服务器 scope 和超长文本不能发起语音生成', () async {
    final api = _Api()..missingScope = true;
    await expectLater(
      service(api).synthesize('role-a', '你好'),
      throwsStateError,
    );
    await expectLater(
      service(api).synthesize('role-a', '汉' * 3000),
      throwsStateError,
    );
    expect(api.writes, 0);
  });

  test('bound 三个旧 TTS 入口都在请求前拒绝', () async {
    final api = _Api();
    final tts = TTSService(api, isBound: () => true);
    await expectLater(tts.synthesize('你好'), throwsStateError);
    await expectLater(
      tts.synthesizeForCharacter('role-a', '你好'),
      throwsStateError,
    );
    expect(
      () => tts.synthesizeWithSpeaker('speaker-id', '你好'),
      throwsStateError,
    );
    expect(api.writes, 0);
  });

  test('ACK 后切 Core 不会启动本机播放', () async {
    final api = _Api();
    final audio = await service(api).synthesize('role-a', '你好');
    api.core = 'core-c';
    final operations = <String>[];
    final player = OwnedSpeechPlayer(
      platform: 'android',
      execute: (request) async {
        operations.add(request['operation'] as String);
        return {'status': 'success'};
      },
    );
    addTearDown(player.close);
    await expectLater(player.play(audio), throwsStateError);
    expect(operations, isNot(contains('media.audio.play_file')));
  });

  test('播放保存确认的字节，stop 清理文件并停止原生播放', () async {
    final api = _Api();
    final audio = await service(api).synthesize('role-a', '你好');
    audio.bytes[0] = 99;
    String? playedPath;
    final operations = <String>[];
    final player = OwnedSpeechPlayer(
      platform: 'android',
      execute: (request) async {
        operations.add(request['operation'] as String);
        if (request['operation'] == 'media.audio.play_file') {
          playedPath = request['payload']['path'] as String;
          expect(await File(playedPath!).readAsBytes(), [1, 2, 3, 4]);
        }
        return {
          'status': 'success',
          'result': {'durationMs': 60000},
        };
      },
    );
    addTearDown(player.close);
    await player.play(audio);
    await player.stop();
    expect(await File(playedPath!).exists(), false);
    expect(operations.last, 'media.audio.stop');
  });

  test('播放启动等待期间 stop 能阻止迟到播放', () async {
    final api = _Api();
    final audio = await service(api).synthesize('role-a', '你好');
    api.policyGate = Completer<void>();
    final operations = <String>[];
    final player = OwnedSpeechPlayer(
      platform: 'android',
      execute: (request) async {
        operations.add(request['operation'] as String);
        return {'status': 'success'};
      },
    );
    addTearDown(player.close);
    final playing = player.play(audio);
    await Future<void>.delayed(Duration.zero);
    await player.stop();
    api.policyGate!.complete();
    await expectLater(playing, throwsStateError);
    expect(operations, isNot(contains('media.audio.play_file')));
  });

  test('播放中 Core 切换被检查后停止并清理音频', () async {
    final api = _Api();
    final audio = await service(api).synthesize('role-a', '你好');
    final operations = <String>[];
    String? path;
    final player = OwnedSpeechPlayer(
      platform: 'android',
      execute: (request) async {
        operations.add(request['operation'] as String);
        if (request['operation'] == 'media.audio.play_file') {
          path = request['payload']['path'] as String;
        }
        return {
          'status': 'success',
          'result': {'durationMs': 60000},
        };
      },
    );
    addTearDown(player.close);
    await player.play(audio);
    api.core = 'core-c';
    await Future<void>.delayed(const Duration(milliseconds: 1200));
    expect(operations.last, 'media.audio.stop');
    expect(await File(path!).exists(), false);
  });
}
