import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/workspace_service.dart';
import 'package:flutter_test/flutter_test.dart';

class _FakeBackendApi extends Fake implements BackendServiceApi {
  String? getPath;
  String? postPath;
  Object? postData;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    getPath = path;
    final data = <dynamic>[
      <String, dynamic>{
        'id': 'mount-1',
        'name': '项目',
        'kind': 'saf',
        'rootUri': 'content://workspace/mount-1',
        'readOnly': false,
        'available': true,
        'status': 'ready',
      },
    ];
    return fromJson == null ? data as T? : fromJson(data);
  }

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    postPath = path;
    postData = data;
    final response = <String, dynamic>{
      'id': 'mount-1',
      'name': '项目',
      'kind': 'saf',
      'rootUri': 'content://workspace/mount-1',
      'readOnly': false,
      'available': true,
      'status': 'ready',
    };
    return fromJson == null ? response as T? : fromJson(response);
  }
}

void main() {
  test('workspace mounts use the backend workspace routes', () async {
    final api = _FakeBackendApi();
    final service = WorkspaceService(api);

    final mounts = await service.listLocal();
    expect(api.getPath, '/api/workspaces');
    expect(mounts.single.kind, 'saf');

    await service.registerLocal(name: '项目', localRoot: '/tmp/project');
    expect(api.postPath, '/api/workspaces/local');
    expect((api.postData as Map<String, dynamic>)['localRoot'], '/tmp/project');

    await service.registerSaf(
      name: '项目',
      grantId: 'content://workspace/mount-1',
    );
    expect(api.postPath, '/api/workspaces/saf');
    expect(
      (api.postData as Map<String, dynamic>)['grantId'],
      'content://workspace/mount-1',
    );

    await service.touchLocal('mount-1');
    expect(api.postPath, '/api/workspaces/mount-1/touch');
  });
}
