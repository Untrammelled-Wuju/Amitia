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
  });

  factory ProjectDto.fromJson(Map<String, dynamic> json) {
    return ProjectDto(
      id: (json['id'] ?? '').toString(),
      name: (json['name'] ?? '').toString(),
      workspaceId: (json['workspaceId'] ?? '').toString(),
      deviceId: (json['deviceId'] ?? '').toString(),
      rootUri: (json['rootUri'] ?? '').toString(),
      workspaceKind: (json['workspaceKind'] ?? '').toString(),
      available: json['available'] != false,
      status: (json['status'] ?? 'ready').toString(),
      statusReason: (json['statusReason'] ?? '').toString(),
      conversationCount: (json['conversationCount'] as num?)?.toInt() ?? 0,
      pinnedAt: (json['pinnedAt'] ?? '').toString(),
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
