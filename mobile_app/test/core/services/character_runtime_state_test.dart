import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/character_detail_service.dart';
import 'package:flutter_test/flutter_test.dart';

class _RuntimeStateApi extends Fake implements BackendServiceApi {
  String? requestedPath;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    requestedPath = path;
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
}

void main() {
  test('心理页通过 Owner 运行状态接口读取并投影', () async {
    final api = _RuntimeStateApi();
    final state = await CharacterDetailService(api).psycheSnapshot('role');
    expect(
      api.requestedPath,
      '/api/device-mesh/v1/business/runtime-state',
    );
    expect((state?['emotion'] as Map)['positive'], 0.7);
    expect((state?['needs'] as Map)['connection'], 0.55);
    expect((state?['relationship'] as Map)['familiarity'], 0.4);
  });
}
