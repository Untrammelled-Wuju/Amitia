import '../backend_transport/backend_service_api.dart';

class WorkspaceMountDto {
  final String id;
  final String name;
  final String kind;
  final String rootUri;
  final bool readOnly;
  final bool available;
  final String status;

  WorkspaceMountDto({
    required this.id,
    required this.name,
    required this.kind,
    required this.rootUri,
    required this.readOnly,
    required this.available,
    required this.status,
  });

  factory WorkspaceMountDto.fromJson(Map<String, dynamic> json) {
    return WorkspaceMountDto(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      kind: json['kind'] as String? ?? 'local',
      rootUri: json['rootUri'] as String? ?? '',
      readOnly: json['readOnly'] as bool? ?? false,
      available: json['available'] as bool? ?? false,
      status: json['status'] as String? ?? 'unavailable',
    );
  }
}

class WorkspaceService {
  final BackendServiceApi _api;

  WorkspaceService(this._api);

  Future<List<WorkspaceMountDto>> list() async {
    final resp = await _api.get<List<dynamic>>('/api/workspaces');
    if (resp == null) return [];
    return resp
        .map((e) => WorkspaceMountDto.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<WorkspaceMountDto?> getById(String id) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/workspaces/$id',
      fromJson: (e) => e as Map<String, dynamic>,
    );
    if (resp == null) return null;
    return WorkspaceMountDto.fromJson(resp);
  }

  Future<bool> remove(String id) async {
    await _api.delete('/api/workspaces/$id');
    return true;
  }
}
