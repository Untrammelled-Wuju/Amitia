class WorldBookDto {
  final String id;
  final String matchType;
  final String matchPattern;
  final String matchScope;
  final String injectContent;
  final int priority;
  final int hitCount;
  final String characterId;
  final String configJson;
  final String createdAt;
  final String updatedAt;

  const WorldBookDto({
    required this.id,
    this.matchType = 'keyword',
    this.matchPattern = '',
    this.matchScope = 'full_context',
    this.injectContent = '',
    this.priority = 0,
    this.hitCount = 0,
    this.characterId = '',
    this.configJson = '{}',
    this.createdAt = '',
    this.updatedAt = '',
  });

  factory WorldBookDto.fromJson(Map<String, dynamic> json) => WorldBookDto(
        id: (json['id'] ?? '').toString(),
        matchType: (json['matchType'] ?? 'keyword').toString(),
        matchPattern: (json['matchPattern'] ?? '').toString(),
        matchScope: (json['matchScope'] ?? 'full_context').toString(),
        injectContent: (json['injectContent'] ?? '').toString(),
        priority: (json['priority'] as num?)?.toInt() ?? 0,
        hitCount: (json['hitCount'] as num?)?.toInt() ?? 0,
        characterId: (json['characterId'] ?? '').toString(),
        configJson: (json['configJson'] ?? '{}').toString(),
        createdAt: (json['createdAt'] ?? '').toString(),
        updatedAt: (json['updatedAt'] ?? '').toString(),
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'matchType': matchType,
        'matchPattern': matchPattern,
        'matchScope': matchScope,
        'injectContent': injectContent,
        'priority': priority,
        'hitCount': hitCount,
        'characterId': characterId,
        'configJson': configJson,
        'createdAt': createdAt,
        'updatedAt': updatedAt,
      };
}

class WorldBookMatchDto {
  final WorldBookDto entry;
  final String matchScope;
  final String hitText;

  const WorldBookMatchDto({required this.entry, this.matchScope = '', this.hitText = ''});

  factory WorldBookMatchDto.fromJson(Map<String, dynamic> json) => WorldBookMatchDto(
        entry: WorldBookDto.fromJson(Map<String, dynamic>.from(json['entry'] as Map? ?? const {})),
        matchScope: (json['matchScope'] ?? '').toString(),
        hitText: (json['hitText'] ?? '').toString(),
      );
}
