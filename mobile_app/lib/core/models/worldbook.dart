class WorldBookDto {
  final String id;
  final String scope;
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
  final bool enabled;
  final String ownerId;
  final String roleId;
  final int revision;
  final List<String> secondaryKeys;
  final bool? caseSensitive;
  final bool? selective;
  final bool? constant;
  final int insertionOrder;
  final String? position;
  final Map<String, dynamic>? extensions;
  final String characterBookEntryName;
  final dynamic characterBookEntryId;
  final String sourceCardHash;
  final int sourceEntryIndex;

  const WorldBookDto({
    required this.id,
    this.scope = 'role',
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
    this.enabled = true,
    this.ownerId = '',
    this.roleId = '',
    this.revision = 0,
    this.secondaryKeys = const [],
    this.caseSensitive,
    this.selective,
    this.constant,
    this.insertionOrder = 0,
    this.position,
    this.extensions,
    this.characterBookEntryName = '',
    this.characterBookEntryId,
    this.sourceCardHash = '',
    this.sourceEntryIndex = 0,
  });

  factory WorldBookDto.fromJson(Map<String, dynamic> json) => WorldBookDto(
    id: (json['id'] ?? '').toString(),
    scope: (json['scope'] ?? 'role').toString(),
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
    enabled: json['enabled'] != false,
    ownerId: (json['ownerId'] ?? '').toString(),
    roleId: (json['roleId'] ?? '').toString(),
    revision: (json['revision'] as num?)?.toInt() ?? 0,
    secondaryKeys:
        (json['secondaryKeys'] as List?)
            ?.map((value) => value.toString())
            .toList(growable: false) ??
        const [],
    caseSensitive: json['caseSensitive'] as bool?,
    selective: json['selective'] as bool?,
    constant: json['constant'] as bool?,
    insertionOrder: (json['insertionOrder'] as num?)?.toInt() ?? 0,
    position: json['position']?.toString(),
    extensions: json['extensions'] is Map
        ? Map<String, dynamic>.from(json['extensions'] as Map)
        : null,
    characterBookEntryName: (json['characterBookEntryName'] ?? '').toString(),
    characterBookEntryId: json['characterBookEntryId'],
    sourceCardHash: (json['sourceCardHash'] ?? '').toString(),
    sourceEntryIndex: (json['sourceEntryIndex'] as num?)?.toInt() ?? 0,
  );

  Map<String, dynamic> toJson() => {
    'id': id,
    'scope': scope,
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
    'enabled': enabled,
    'ownerId': ownerId,
    'roleId': roleId,
    'revision': revision,
    'secondaryKeys': secondaryKeys,
    'caseSensitive': caseSensitive,
    'selective': selective,
    'constant': constant,
    'insertionOrder': insertionOrder,
    'position': position,
    'extensions': extensions,
    'characterBookEntryName': characterBookEntryName,
    'characterBookEntryId': characterBookEntryId,
    'sourceCardHash': sourceCardHash,
    'sourceEntryIndex': sourceEntryIndex,
  };
}

class WorldBookMatchDto {
  final WorldBookDto entry;
  final String matchScope;
  final String hitText;

  const WorldBookMatchDto({
    required this.entry,
    this.matchScope = '',
    this.hitText = '',
  });

  factory WorldBookMatchDto.fromJson(Map<String, dynamic> json) =>
      WorldBookMatchDto(
        entry: WorldBookDto.fromJson(
          Map<String, dynamic>.from(json['entry'] as Map? ?? const {}),
        ),
        matchScope: (json['matchScope'] ?? '').toString(),
        hitText: (json['hitText'] ?? '').toString(),
      );
}
