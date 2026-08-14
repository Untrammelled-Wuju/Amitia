import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_access/business_backend_unavailable.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_phase.dart';
import 'package:amitia_app/core/services/character_service.dart';

void main() {
  group('CharacterService + businessAvailable Gate', () {
    test('gate=false blocks list() request, network count = 0', () async {
      final fakeApi = _GateBlockedApi();
      final service = CharacterService(fakeApi);

      expect(() => service.list(),
          throwsA(isA<BusinessBackendUnavailable>()));
      expect(fakeApi.sendCount, 0);
    });

    test('gate=false blocks getById() request, network count = 0', () async {
      final fakeApi = _GateBlockedApi();
      final service = CharacterService(fakeApi);

      expect(() => service.getById('char-001'),
          throwsA(isA<BusinessBackendUnavailable>()));
      expect(fakeApi.sendCount, 0);
    });

    test('gate=false blocks create() request, network count = 0', () async {
      final fakeApi = _GateBlockedApi();
      final service = CharacterService(fakeApi);

      expect(() => service.create({'name': 'test'}),
          throwsA(isA<BusinessBackendUnavailable>()));
      expect(fakeApi.sendCount, 0);
    });

    test('gate=false blocks update() request, network count = 0', () async {
      final fakeApi = _GateBlockedApi();
      final service = CharacterService(fakeApi);

      expect(() => service.update('char-001', {'name': 'test'}),
          throwsA(isA<BusinessBackendUnavailable>()));
      expect(fakeApi.sendCount, 0);
    });

    test('gate=false blocks delete() request, network count = 0', () async {
      final fakeApi = _GateBlockedApi();
      final service = CharacterService(fakeApi);

      expect(() => service.delete('char-001'),
          throwsA(isA<BusinessBackendUnavailable>()));
      expect(fakeApi.sendCount, 0);
    });

    test('gate=true allows all CharacterService requests', () async {
      final fakeApi = _GatePassingApi(responseData: []);
      final service = CharacterService(fakeApi);

      await service.list();
      await service.getById('char-001');
      await service.create({'name': 'test'});
      await service.update('char-001', {'name': 'test'});
      await service.delete('char-001');

      expect(fakeApi.sendCount, 5);
    });

    test('gate transition true→false→true: requests blocked then allowed',
        () async {
      final gateState = _GateState(available: true);
      final fakeApi = _DynamicGateApi(
        gateState: gateState,
        responseData: [
          {
            'id': 'char-001',
            'name': 'Test',
            'avatar': '',
            'identity': '',
            'personality': '',
            'speakingStyle': '',
            'description': '',
            'status': 'enabled',
            'isActive': 0,
            'createdAt': '2026-01-01 00:00:00',
          }
        ],
      );
      final service = CharacterService(fakeApi);

      // Gate true: request succeeds
      final result1 = await service.list();
      expect(result1, hasLength(1));
      expect(fakeApi.sendCount, 1);

      // Gate false: request blocked
      gateState.available = false;
      expect(() => service.list(),
          throwsA(isA<BusinessBackendUnavailable>()));
      expect(fakeApi.sendCount, 1);

      // Gate true again: request succeeds
      gateState.available = true;
      final result2 = await service.list();
      expect(result2, hasLength(1));
      expect(fakeApi.sendCount, 2);
    });
  });

  group('CharacterService Generation Mismatch', () {
    test('generation mismatch: old generation request blocked', () async {
      final fakeApi = _GenerationMismatchApi(currentGeneration: 5);
      final service = CharacterService(fakeApi);

      expect(() => service.list(),
          throwsA(isA<BusinessBackendUnavailable>()));
      expect(fakeApi.sendCount, 0);
    });
  });
}

class _GateState {
  bool available;
  _GateState({required this.available});
}

class _GateBlockedApi implements BackendServiceApi {
  int sendCount = 0;

  void _check() {
    throw const BusinessBackendUnavailable(
      phase: RuntimeStatusPhase.degraded,
      generation: 0,
      primaryError: null,
    );
  }

  @override
  int get generation => 0;

  @override
  Future<T?> get<T>(String path,
      {Map<String, dynamic>? queryParameters,
      T Function(dynamic)? fromJson}) async {
    _check();
    sendCount++;
    return null;
  }

  @override
  Future<T?> post<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    _check();
    sendCount++;
    return null;
  }

  @override
  Future<T?> put<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    _check();
    sendCount++;
    return null;
  }

  @override
  Future<void> delete(String path) async {
    _check();
    sendCount++;
  }

  @override
  Future<T?> deleteWithResponse<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    _check();
    sendCount++;
    return null;
  }
}

class _GatePassingApi implements BackendServiceApi {
  final dynamic responseData;
  int sendCount = 0;

  _GatePassingApi({required this.responseData});

  dynamic _responseFor(String path) {
    if (responseData == null) return null;
    if (path.startsWith('/api/characters/') && path != '/api/characters') {
      return {
        'code': 200,
        'message': 'success',
        'data': {
          'id': path.split('/').last,
          'name': 'Single',
          'avatar': '',
          'identity': '',
          'personality': '',
          'speakingStyle': '',
          'description': '',
          'status': 'enabled',
          'isActive': 0,
          'createdAt': '2026-01-01 00:00:00',
        },
      };
    }
    return {'code': 200, 'message': 'success', 'data': responseData};
  }

  @override
  int get generation => 1;

  @override
  Future<T?> get<T>(String path,
      {Map<String, dynamic>? queryParameters,
      T Function(dynamic)? fromJson}) async {
    sendCount++;
    final wrapped = _responseFor(path);
    if (wrapped == null) return null;
    final data = wrapped['data'];
    if (fromJson != null && data != null) return fromJson(data);
    return data as T?;
  }

  @override
  Future<T?> post<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    sendCount++;
    return null;
  }

  @override
  Future<T?> put<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    sendCount++;
    return null;
  }

  @override
  Future<void> delete(String path) async {
    sendCount++;
  }

  @override
  Future<T?> deleteWithResponse<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    sendCount++;
    return null;
  }
}

class _DynamicGateApi implements BackendServiceApi {
  final _GateState gateState;
  final dynamic responseData;
  int sendCount = 0;

  _DynamicGateApi({required this.gateState, required this.responseData});

  void _check() {
    if (!gateState.available) {
      throw const BusinessBackendUnavailable(
        phase: RuntimeStatusPhase.degraded,
        generation: 0,
        primaryError: null,
      );
    }
  }

  dynamic _wrapResponse() {
    if (responseData == null) return null;
    return {'code': 200, 'message': 'success', 'data': responseData};
  }

  @override
  int get generation => 1;

  @override
  Future<T?> get<T>(String path,
      {Map<String, dynamic>? queryParameters,
      T Function(dynamic)? fromJson}) async {
    _check();
    sendCount++;
    final wrapped = _wrapResponse();
    if (wrapped == null) return null;
    final data = wrapped['data'];
    if (fromJson != null && data != null) return fromJson(data);
    return data as T?;
  }

  @override
  Future<T?> post<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    _check();
    sendCount++;
    return null;
  }

  @override
  Future<T?> put<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    _check();
    sendCount++;
    return null;
  }

  @override
  Future<void> delete(String path) async {
    _check();
    sendCount++;
  }

  @override
  Future<T?> deleteWithResponse<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    _check();
    sendCount++;
    return null;
  }
}

class _GenerationMismatchApi implements BackendServiceApi {
  final int currentGeneration;
  int sendCount = 0;

  _GenerationMismatchApi({required this.currentGeneration});

  @override
  int get generation => currentGeneration + 1;

  void _check() {
    if (generation != currentGeneration) {
      throw const BusinessBackendUnavailable(
        phase: RuntimeStatusPhase.degraded,
        generation: 0,
        primaryError: null,
      );
    }
  }

  @override
  Future<T?> get<T>(String path,
      {Map<String, dynamic>? queryParameters,
      T Function(dynamic)? fromJson}) async {
    _check();
    sendCount++;
    return null;
  }

  @override
  Future<T?> post<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    _check();
    sendCount++;
    return null;
  }

  @override
  Future<T?> put<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    _check();
    sendCount++;
    return null;
  }

  @override
  Future<void> delete(String path) async {
    _check();
    sendCount++;
  }

  @override
  Future<T?> deleteWithResponse<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    _check();
    sendCount++;
    return null;
  }
}
