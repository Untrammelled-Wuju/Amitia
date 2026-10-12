import 'conversation.dart';

class ProjectWorkspaceDto {
  final String id;
  final String deviceId;
  final String name;
  final String kind;
  final String rootUri;
  final bool readOnly;

  const ProjectWorkspaceDto({
    required this.id,
    required this.deviceId,
    required this.name,
    required this.kind,
    required this.rootUri,
    this.readOnly = false,
  });

  factory ProjectWorkspaceDto.fromJson(Map<String, dynamic> json) {
    return ProjectWorkspaceDto(
      id: (json['id'] ?? '').toString(),
      deviceId: (json['deviceId'] ?? '').toString(),
      name: (json['name'] ?? '').toString(),
      kind: (json['kind'] ?? 'local').toString(),
      rootUri: (json['rootUri'] ?? '').toString(),
      readOnly: json['readOnly'] == true,
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
    'id': id,
    'deviceId': deviceId,
    'name': name,
    'kind': kind,
    'rootUri': rootUri,
    'readOnly': readOnly,
  };
}

class ProjectDto {
  final String id;
  final String name;
  final String workspaceId;
  final String deviceId;
  final String rootUri;
  final String workspaceKind;
  final bool available;
  final String status;
  final String statusReason;
  final int conversationCount;
  final List<ConversationDto> conversations;
  final String pinnedAt;
  final String ownerId;
  final String roleId;
  final int revision;
  final bool readOnly;
  final List<ProjectWorkspaceDto> workspaces;
  final String primaryWorkspaceId;
  final String hiddenAt;
  final Map<String, dynamic>? executionScope;

  const ProjectDto({
    required this.id,
    required this.name,
    required this.workspaceId,
    this.deviceId = '',
    this.rootUri = '',
    this.workspaceKind = '',
    this.available = true,
    this.status = 'ready',
    this.statusReason = '',
    this.conversationCount = 0,
    this.conversations = const <ConversationDto>[],
    this.pinnedAt = '',
    this.ownerId = '',
    this.roleId = '',
    this.revision = 0,
    this.readOnly = false,
    this.workspaces = const <ProjectWorkspaceDto>[],
    this.primaryWorkspaceId = '',
    this.hiddenAt = '',
    this.executionScope,
  });

  factory ProjectDto.fromJson(Map<String, dynamic> json) {
    final workspaces = ((json['workspaces'] as List<dynamic>?) ?? const [])
        .whereType<Map>()
        .map(
          (item) =>
              ProjectWorkspaceDto.fromJson(Map<String, dynamic>.from(item)),
        )
        .where((workspace) => workspace.id.isNotEmpty)
        .toList(growable: false);
    final primaryWorkspaceId = (json['primaryWorkspaceId'] ?? '').toString();
    ProjectWorkspaceDto? primary;
    for (final workspace in workspaces) {
      if (workspace.id == primaryWorkspaceId) {
        primary = workspace;
        break;
      }
    }
    if (primary == null && workspaces.isNotEmpty) primary = workspaces.first;
    return ProjectDto(
      id: (json['id'] ?? '').toString(),
      name: (json['name'] ?? json['title'] ?? '').toString(),
      workspaceId: (json['workspaceId'] ?? primary?.id ?? '').toString(),
      deviceId: (json['deviceId'] ?? primary?.deviceId ?? '').toString(),
      rootUri: (json['rootUri'] ?? primary?.rootUri ?? '').toString(),
      workspaceKind: (json['workspaceKind'] ?? primary?.kind ?? '').toString(),
      available: json['available'] != false,
      status: (json['status'] ?? 'ready').toString(),
      statusReason: (json['statusReason'] ?? '').toString(),
      conversationCount: (json['conversationCount'] as num?)?.toInt() ?? 0,
      pinnedAt: (json['pinnedAt'] ?? '').toString(),
      ownerId: (json['ownerId'] ?? '').toString(),
      roleId: (json['roleId'] ?? '').toString(),
      revision: (json['revision'] as num?)?.toInt() ?? 0,
      readOnly: json['readOnly'] == true,
      workspaces: workspaces,
      primaryWorkspaceId: primaryWorkspaceId.isNotEmpty
          ? primaryWorkspaceId
          : (workspaces.isEmpty ? '' : workspaces.first.id),
      hiddenAt: (json['hiddenAt'] ?? '').toString(),
      executionScope: json['executionScope'] is Map
          ? Map<String, dynamic>.unmodifiable(
              Map<String, dynamic>.from(json['executionScope'] as Map),
            )
          : null,
      conversations: ((json['conversations'] as List<dynamic>?) ?? const [])
          .whereType<Map>()
          .map(
            (item) => ConversationDto.fromJson(Map<String, dynamic>.from(item)),
          )
          .where((conversation) => conversation.channel == 'web')
          .toList(growable: false),
    );
  }
}

class ConversationSidebarDto {
  final List<ConversationDto> pinned;
  final List<ConversationDto> recent;
  final List<ProjectDto> projects;
  final List<ProjectDto> removedProjects;

  const ConversationSidebarDto({
    this.pinned = const <ConversationDto>[],
    this.recent = const <ConversationDto>[],
    this.projects = const <ProjectDto>[],
    this.removedProjects = const <ProjectDto>[],
  });

  factory ConversationSidebarDto.fromJson(Map<String, dynamic> json) {
    return ConversationSidebarDto(
      pinned: ((json['pinned'] as List<dynamic>?) ?? const [])
          .whereType<Map>()
          .map(
            (item) => ConversationDto.fromJson(Map<String, dynamic>.from(item)),
          )
          .where((conversation) => conversation.channel == 'web')
          .toList(growable: false),
      recent: ((json['recent'] as List<dynamic>?) ?? const [])
          .whereType<Map>()
          .map(
            (item) => ConversationDto.fromJson(Map<String, dynamic>.from(item)),
          )
          .toList(growable: false),
      projects: ((json['projects'] as List<dynamic>?) ?? const [])
          .whereType<Map>()
          .map((item) => ProjectDto.fromJson(Map<String, dynamic>.from(item)))
          .toList(growable: false),
      removedProjects: ((json['removedProjects'] as List<dynamic>?) ?? const [])
          .whereType<Map>()
          .map((item) => ProjectDto.fromJson(Map<String, dynamic>.from(item)))
          .toList(growable: false),
    );
  }
}
