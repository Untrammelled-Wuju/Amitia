import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/character_detail_service.dart';
import 'package:flutter_test/flutter_test.dart';

class _RuntimeStateApi extends Fake implements BackendServiceApi {
  String? requestedPath;
  String? postedPath;
  Object? postedData;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    requestedPath = path;
    if (path == '/api/device-mesh/v1/business/data') {
      final data = <String, dynamic>{
        'executionScope': <String, dynamic>{
          'roleId': 'role',
          'resourceOwnerId': 'device',
        },
        'snapshot': <String, dynamic>{
          'ownerId': 'device',
          'resources': <dynamic>[
            <String, dynamic>{
              'kind': 'emotion',
              'ownerId': 'device',
              'roleId': 'role',
              'body': <String, dynamic>{
                'id': 'emotion/request',
                'createdAt': '2026-10-23T07:00:00Z',
                'affectLabel': '积极',
                'emotionLabel': 'positive',
              },
            },
          ],
        },
      };
      return fromJson != null ? fromJson(data) : data as T;
    }
    if (path == '/api/device-mesh/v1/business/resources') {
      final data = <String, dynamic>{
        'resource': <String, dynamic>{
          'kind': 'emotion',
          'id': 'emotion/request',
          'ownerId': 'device',
          'roleId': 'role',
          'revision': 2,
          'deleted': false,
        },
        'executionScope': <String, dynamic>{
          'roleId': 'role',
          'resourceOwnerId': 'device',
          'coreId': 'core',
          'modeRevision': 1,
        },
      };
      return fromJson != null ? fromJson(data) : data as T;
    }
    final data = <String, dynamic>{
      'updatedAt': '2026-10-21T02:00:00Z',
      'psyche': <String, dynamic>{
        'valence': 0.7,
        'arousal': 0.4,
        'dominance': 0.6,
        'moodValence': 0.65,
        'moodArousal': 0.2,
        'stress': 0.1,
        'fatigue': 0.25,
      },
      'relationship': <String, dynamic>{
        'trust': 0.7,
        'familiarity': 0.4,
        'security': 0.6,
        'tension': 0.1,
        'repairConfidence': 0.8,
        'boundary': 0.5,
      },
      'needs': <dynamic>[
        <String, dynamic>{'kind': 'connection', 'level': 0.55},
      ],
      'beliefs': <dynamic>[
        <String, dynamic>{'key': '偏好', 'value': '茶', 'confidence': 0.9},
      ],
    };
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
    postedPath = path;
    postedData = data;
    final result = <String, dynamic>{
      'ownerId': 'device',
      'versions': <String, dynamic>{'emotion/emotion/request': 3},
    };
    return fromJson != null ? fromJson(result) : result as T;
  }
}

void main() {
  test('心理页通过 Owner 运行状态接口读取并投影', () async {
    final api = _RuntimeStateApi();
    final state = await CharacterDetailService(api).psycheSnapshot('role');
    expect(api.requestedPath, '/api/device-mesh/v1/business/runtime-state');
    expect((state?['emotion'] as Map)['positive'], 0.7);
    expect((state?['needs'] as Map)['connection'], 0.55);
    expect((state?['relationship'] as Map)['familiarity'], 0.4);
  });

  test('心理页通过 Owner data 接口读取情绪历史', () async {
    final api = _RuntimeStateApi();
    final history = await CharacterDetailService(api).emotionHistory('role');
    expect(api.requestedPath, '/api/device-mesh/v1/business/data');
    expect(history, hasLength(1));
    expect(history.single['affectLabel'], '积极');
  });

  test('心理页通过 Owner 编辑接口删除情绪历史', () async {
    final api = _RuntimeStateApi();
    await CharacterDetailService(
      api,
    ).deleteEmotionHistory('role', 'emotion/request');
    expect(api.postedPath, '/api/device-mesh/v1/business/resources/edit');
    expect(api.postedData, isA<Map>());
    final request = api.postedData as Map;
    expect(request['kind'], 'emotion');
    expect(request['expectedRevision'], 2);
    expect(request['deleted'], true);
  });
}
