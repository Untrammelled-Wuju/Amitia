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
        .whereType<Map>()
        .map((e) => WorkspaceMountDto.fromJson(Map<String, dynamic>.from(e)))
        .toList();
  }

  Future<WorkspaceMountDto?> getById(String id) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/workspaces/$id',
      fromJson: (e) => Map<String, dynamic>.from(e as Map),
    );
    if (resp == null) return null;
    return WorkspaceMountDto.fromJson(resp);
  }

  Future<bool> remove(String id) async {
    await _api.delete('/api/workspaces/$id');
    return true;
  }

  Future<Map<String, dynamic>> gitStatus(String workspaceUri, {bool includeIgnored = false}) =>
      _postMap('/api/workspaces/git/status', {
        'workspaceUri': workspaceUri,
        'includeIgnored': includeIgnored,
      });

  Future<Map<String, dynamic>> gitDiff(
    String workspaceUri, {
    String mode = 'worktree',
    String base = '',
    String target = '',
    List<String> paths = const [],
    int maxBytes = 1048576,
  }) =>
      _postMap('/api/workspaces/git/diff', {
        'workspaceUri': workspaceUri,
        'mode': mode,
        'base': base,
        'target': target,
        if (paths.isNotEmpty) 'paths': paths,
        'maxBytes': maxBytes,
      });

  Future<Map<String, dynamic>> gitLog(
    String workspaceUri, {
    int limit = 50,
    String path = '',
    String refName = '',
  }) =>
      _postMap('/api/workspaces/git/log', {
        'workspaceUri': workspaceUri,
        'limit': limit,
        if (path.isNotEmpty) 'path': path,
        if (refName.isNotEmpty) 'ref': refName,
      });

  Future<Map<String, dynamic>> gitAdd(
    String workspaceUri, {
    List<String> paths = const [],
    bool all = false,
    bool force = false,
  }) =>
      _postMap('/api/workspaces/git/add', {
        'workspaceUri': workspaceUri,
        'paths': paths,
        'all': all,
        'force': force,
      });

  Future<Map<String, dynamic>> gitRestore(
    String workspaceUri, {
    required List<String> paths,
    String source = '',
    bool staged = false,
    bool worktree = true,
  }) =>
      _postMap('/api/workspaces/git/restore', {
        'workspaceUri': workspaceUri,
        'paths': paths,
        if (source.isNotEmpty) 'source': source,
        'staged': staged,
        'worktree': worktree,
      });

  Future<Map<String, dynamic>> gitCommit(
    String workspaceUri,
    String message, {
    String authorName = '',
    String authorEmail = '',
  }) =>
      _postMap('/api/workspaces/git/commit', {
        'workspaceUri': workspaceUri,
        'message': message,
        if (authorName.isNotEmpty || authorEmail.isNotEmpty)
          'author': {'name': authorName, 'email': authorEmail},
      });

  Future<Map<String, dynamic>> gitBranches(String workspaceUri) =>
      _postMap('/api/workspaces/git/branches', {'workspaceUri': workspaceUri});

  Future<Map<String, dynamic>> gitCheckout(
    String workspaceUri,
    String branch, {
    bool create = false,
    String fromRef = '',
    bool detach = false,
    bool force = false,
  }) =>
      _postMap('/api/workspaces/git/checkout', {
        'workspaceUri': workspaceUri,
        'branch': branch,
        'create': create,
        if (fromRef.isNotEmpty) 'fromRef': fromRef,
        'detach': detach,
        'force': force,
      });

  Future<Map<String, dynamic>> gitFetch(
    String workspaceUri, {
    String remote = '',
    int depth = 0,
    int deepen = 0,
  }) =>
      _postMap('/api/workspaces/git/fetch', {
        'workspaceUri': workspaceUri,
        if (remote.isNotEmpty) 'remote': remote,
        if (depth > 0) 'depth': depth,
        if (deepen > 0) 'deepen': deepen,
      });

  Future<Map<String, dynamic>> gitPull(
    String workspaceUri, {
    String remote = '',
    String branch = '',
  }) =>
      _postMap('/api/workspaces/git/pull', {
        'workspaceUri': workspaceUri,
        if (remote.isNotEmpty) 'remote': remote,
        if (branch.isNotEmpty) 'branch': branch,
      });

  Future<Map<String, dynamic>> gitPush(
    String workspaceUri, {
    required String remote,
    required String localRef,
    required String remoteRef,
    bool setUpstream = false,
  }) =>
      _postMap('/api/workspaces/git/push', {
        'workspaceUri': workspaceUri,
        'remote': remote,
        'localRef': localRef,
        'remoteRef': remoteRef,
        'setUpstream': setUpstream,
      });

  Future<Map<String, dynamic>> gitRemotes(String workspaceUri) =>
      _postMap('/api/workspaces/git/remotes', {'workspaceUri': workspaceUri});

  Future<Map<String, dynamic>> createIsolated({
    required String name,
    required String mode,
    String sourceWorkspaceUri = '',
    String remoteUrl = '',
    String remoteId = '',
    String refName = '',
    int depth = 0,
    bool readOnly = false,
    String lifetime = '',
  }) =>
      _postMap('/api/workspaces/isolated', {
        'name': name,
        'mode': mode,
        if (sourceWorkspaceUri.isNotEmpty) 'sourceWorkspaceUri': sourceWorkspaceUri,
        if (remoteUrl.isNotEmpty || remoteId.isNotEmpty)
          'gitRemote': {
            if (remoteId.isNotEmpty) 'remoteId': remoteId,
            if (remoteUrl.isNotEmpty) 'url': remoteUrl,
            if (refName.isNotEmpty) 'ref': refName,
          },
        if (refName.isNotEmpty) 'ref': refName,
        if (depth > 0) 'depth': depth,
        'readOnly': readOnly,
        if (lifetime.isNotEmpty) 'lifetime': lifetime,
      });

  Future<Map<String, dynamic>> isolatedInfo(String workspaceUri) =>
      _postMap('/api/workspaces/isolated/info', {'workspaceUri': workspaceUri});

  Future<void> deleteIsolated(String workspaceUri, {bool force = false}) async {
    await _api.post<Map<String, dynamic>>(
      '/api/workspaces/isolated/delete',
      data: {'workspaceUri': workspaceUri, 'force': force},
      fromJson: (e) => Map<String, dynamic>.from(e as Map),
    );
  }

  Future<Map<String, dynamic>> _postMap(String path, Map<String, dynamic> data) async {
    return await _api.post<Map<String, dynamic>>(
          path,
          data: data,
          fromJson: (e) => Map<String, dynamic>.from(e as Map),
        ) ??
        <String, dynamic>{};
  }
}
