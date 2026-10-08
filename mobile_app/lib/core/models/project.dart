import 'conversation.dart';

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
  final bool logical;
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
    this.logical = false,
    this.executionScope,
  });

  factory ProjectDto.fromJson(Map<String, dynamic> json) {
    return ProjectDto(
      id: (json['id'] ?? '').toString(),
      name: (json['name'] ?? json['title'] ?? '').toString(),
      workspaceId: (json['workspaceId'] ?? '').toString(),
      deviceId: (json['deviceId'] ?? '').toString(),
      rootUri: (json['rootUri'] ?? '').toString(),
      workspaceKind: (json['workspaceKind'] ?? '').toString(),
      available: json['available'] != false,
      status: (json['status'] ?? 'ready').toString(),
      statusReason: (json['statusReason'] ?? '').toString(),
      conversationCount: (json['conversationCount'] as num?)?.toInt() ?? 0,
      pinnedAt: (json['pinnedAt'] ?? '').toString(),
      ownerId: (json['ownerId'] ?? '').toString(),
      roleId: (json['roleId'] ?? '').toString(),
      revision: (json['revision'] as num?)?.toInt() ?? 0,
      readOnly: json['readOnly'] == true,
      logical: json['logical'] == true,
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

  const ConversationSidebarDto({
    this.pinned = const <ConversationDto>[],
    this.recent = const <ConversationDto>[],
    this.projects = const <ProjectDto>[],
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
    );
  }
}
