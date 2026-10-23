import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/features/workshop/data/pet_release_service.dart';
import 'package:flutter_test/flutter_test.dart';

class _PetReleaseApi extends Fake implements BackendServiceApi {
  final List<String> paths = [];
  final List<Object?> bodies = [];
  final List<Map<String, String>?> requestHeaders = [];
  int operationPolls = 0;

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    paths.add(path);
    bodies.add(data);
    requestHeaders.add(headers);
    if (path == '/api/desktop-pets/releases/build') {
      return {
            'release': {
              'id': 'release-one',
              'petId': 'pet-one',
              'version': '1.0.1',
              'lifecycle': 'ready',
              'defaultActionKey': 'idle_normal',
              'actionCount': 2,
            },
          }
          as T;
    }
    if (path.endsWith('/install') ||
        path.endsWith('/enable') ||
        path.endsWith('/disable') ||
        path.endsWith('/recenter')) {
      return {
            'operationId': 'operation-one',
            'installationId': 'installation-one',
            'status': 'waiting_runtime_ack',
          }
          as T;
    }
    throw StateError('unexpected POST $path');
  }

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    paths.add(path);
    if (path == '/api/desktop-pets/releases') {
      return {
            'items': [
              {
                'id': 'release-one',
                'petId': 'pet-one',
                'version': '1.0.1',
                'lifecycle': 'ready',
                'defaultActionKey': 'idle_normal',
                'actionCount': 2,
              },
              {
                'id': 'release-draft',
                'petId': 'pet-one',
                'version': '1.0.2',
                'lifecycle': 'building',
              },
            ],
            'total': 2,
          }
          as T;
    }
    if (path == '/api/desktop-pets/installations') {
      expect(headers, {'X-Amitia-Device-ID': 'phone-one'});
      return {
            'items': [
              {'id': 'installation-one', 'currentReleaseId': 'release-one'},
            ],
            'total': 1,
          }
          as T;
    }
    if (path == '/api/desktop-pets/operations/operation-one') {
      operationPolls++;
      return {
            'operation': {
              'id': 'operation-one',
              'installationId': 'installation-one',
              'status': 'completed',
            },
          }
          as T;
    }
    throw StateError('unexpected GET $path');
  }

  @override
  Future<T?> patch<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    paths.add(path);
    bodies.add(data);
    requestHeaders.add(headers);
    return {
          'operationId': 'operation-patch',
          'installationId': 'installation-one',
          'status': 'waiting_runtime_ack',
        }
        as T;
  }

  @override
  Future<T?> deleteWithResponse<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    paths.add(path);
    bodies.add(data);
    requestHeaders.add(headers);
    return {
          'operationId': 'operation-delete',
          'installationId': 'installation-one',
          'status': 'waiting_runtime_ack',
        }
        as T;
  }
}

void main() {
  test(
    'release build uses canonical release endpoint and idempotency key',
    () async {
      final api = _PetReleaseApi();
      final service = PetReleaseService(api);

      final release = await service.buildRelease(
        processingTaskId: 'processing-one',
        defaultAction: 'idle_normal',
        includedActions: const ['idle_normal', 'wave'],
      );

      expect(api.paths, ['/api/desktop-pets/releases/build']);
      expect(api.bodies.single, isA<Map<String, dynamic>>());
      final body = api.bodies.single as Map<String, dynamic>;
      expect(body['processingTaskId'], 'processing-one');
      expect(body['idempotencyKey'], startsWith('mobile-pet-release-'));
      expect(release.id, 'release-one');
      expect(release.version, '1.0.1');
    },
  );

  test('release list excludes non-ready records', () async {
    final service = PetReleaseService(_PetReleaseApi());
    final releases = await service.listReleases();
    expect(releases.map((item) => item.id), ['release-one']);
  });

  test('install tracks the coordinator operation to completion', () async {
    final api = _PetReleaseApi();
    final service = PetReleaseService(api);
    const release = PetRelease(
      id: 'release-one',
      petId: 'pet-one',
      version: '1.0.1',
      lifecycle: 'ready',
      defaultActionKey: 'idle_normal',
      actionCount: 2,
      contentRootHash: '',
      manifestHash: '',
    );

    final submitted = await service.install('phone-one', release);
    final completed = await service.waitForOperation(
      'phone-one',
      submitted,
      attempts: 2,
      interval: Duration.zero,
    );

    expect(api.paths, [
      '/api/desktop-pets/pets/pet-one/releases/release-one/install',
      '/api/desktop-pets/operations/operation-one',
    ]);
    expect(completed.status, 'completed');
    expect(api.operationPolls, 1);
  });

  test('all installation mutations use one operation contract', () async {
    final api = _PetReleaseApi();
    final service = PetReleaseService(api);

    final enabled = await service.setEnabled(
      'phone-one',
      'installation-one',
      true,
    );
    final action = await service.changeDefaultAction(
      'phone-one',
      'installation-one',
      'idle_normal',
    );
    final recentered = await service.recenter('phone-one', 'installation-one');
    final removed = await service.uninstall('phone-one', 'installation-one');

    expect(enabled.status, 'waiting_runtime_ack');
    expect(action.id, 'operation-patch');
    expect(recentered.id, 'operation-one');
    expect(removed.id, 'operation-delete');
    expect(api.paths, [
      '/api/desktop-pets/installations/installation-one/enable',
      '/api/desktop-pets/installations/installation-one/default-action',
      '/api/desktop-pets/installations/installation-one/recenter',
      '/api/desktop-pets/installations/installation-one',
    ]);
    expect(api.bodies[1], {'actionKey': 'idle_normal'});
    for (final headers in api.requestHeaders) {
      expect(headers?['X-Amitia-Device-ID'], 'phone-one');
      expect(headers?['Idempotency-Key'], isNotEmpty);
    }
  });
}
