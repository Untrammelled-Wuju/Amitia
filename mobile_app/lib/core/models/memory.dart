class MemoryPage {
  const MemoryPage({required this.items, required this.nextCursor});

  final List<MemoryDto> items;
  final String nextCursor;

  bool get hasMore => nextCursor.isNotEmpty;
}

class MemoryDto {
  final String id;
  final String characterId;
  final String memoryType;
  final String memorySubtype;
  final String source;
  final String scope;
  final String key;
  final String value;
  final int importance;
  final int confidence;
  final String verifiedStatus;
  final int useCount;
  final int retentionLevel;
  final double memoryStrength;
  final String? strengthUpdatedAt;
  final String? lastReinforcedAt;
  final int reinforceCount;
  final int retrievedCount;
  final int injectedCount;
  final String decayState;
  final bool pinned;
  final String? archivedAt;
  final String sensitivityLevel;
  final bool allowContextUse;
  final bool allowProactiveMention;
  final bool requiresConfirmation;
  final String createdAt;
  final String updatedAt;

  const MemoryDto({
    required this.id,
    this.characterId = '',
    this.memoryType = 'custom',
    this.memorySubtype = '',
    this.source = 'manual',
    this.scope = 'character',
    this.key = '',
    this.value = '',
    this.importance = 0,
    this.confidence = 50,
    this.verifiedStatus = 'unverified',
    this.useCount = 0,
    this.retentionLevel = 3,
    this.memoryStrength = 0.68,
    this.strengthUpdatedAt,
    this.lastReinforcedAt,
    this.reinforceCount = 0,
    this.retrievedCount = 0,
    this.injectedCount = 0,
    this.decayState = 'active',
    this.pinned = false,
    this.archivedAt,
    this.sensitivityLevel = 'internal',
    this.allowContextUse = true,
    this.allowProactiveMention = true,
    this.requiresConfirmation = false,
    this.createdAt = '',
    this.updatedAt = '',
  });

  factory MemoryDto.fromJson(Map<String, dynamic> json) {
    return MemoryDto(
      id: (json['id'] ?? '').toString(),
      characterId: (json['characterId'] ?? '').toString(),
      memoryType: (json['memoryType'] ?? 'custom').toString(),
      memorySubtype: (json['memorySubtype'] ?? '').toString(),
      source: (json['source'] ?? 'manual').toString(),
      scope: (json['scope'] ?? 'character').toString(),
      key: (json['key'] ?? '').toString(),
      value: (json['value'] ?? '').toString(),
      importance: _asInt(json['importance']),
      confidence: _asInt(json['confidence'], fallback: 50),
      verifiedStatus: (json['verifiedStatus'] ?? 'unverified').toString(),
      useCount: _asInt(json['useCount']),
      retentionLevel: _asInt(json['retentionLevel'], fallback: 3),
      memoryStrength: _asDouble(json['memoryStrength'], fallback: 0.68),
      strengthUpdatedAt: _nullableString(json['strengthUpdatedAt']),
      lastReinforcedAt: _nullableString(json['lastReinforcedAt']),
      reinforceCount: _asInt(json['reinforceCount']),
      retrievedCount: _asInt(json['retrievedCount']),
      injectedCount: _asInt(json['injectedCount']),
      decayState: (json['decayState'] ?? 'active').toString(),
      pinned: _asBool(json['pinned']),
      archivedAt: _nullableString(json['archivedAt']),
      sensitivityLevel: (json['sensitivityLevel'] ?? 'internal').toString(),
      allowContextUse: _asBool(json['allowContextUse'], fallback: true),
      allowProactiveMention: _asBool(json['allowProactiveMention'], fallback: true),
      requiresConfirmation: _asBool(json['requiresConfirmation']),
      createdAt: (json['createdAt'] ?? '').toString(),
      updatedAt: (json['updatedAt'] ?? '').toString(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'characterId': characterId,
      'memoryType': memoryType,
      'memorySubtype': memorySubtype,
      'source': source,
      'scope': scope,
      'key': key,
      'value': value,
      'importance': importance,
      'confidence': confidence,
      'verifiedStatus': verifiedStatus,
      'useCount': useCount,
      'retentionLevel': retentionLevel,
      'memoryStrength': memoryStrength,
      'strengthUpdatedAt': strengthUpdatedAt,
      'lastReinforcedAt': lastReinforcedAt,
      'reinforceCount': reinforceCount,
      'retrievedCount': retrievedCount,
      'injectedCount': injectedCount,
      'decayState': decayState,
      'pinned': pinned,
      'archivedAt': archivedAt,
      'sensitivityLevel': sensitivityLevel,
      'allowContextUse': allowContextUse,
      'allowProactiveMention': allowProactiveMention,
      'requiresConfirmation': requiresConfirmation,
      'createdAt': createdAt,
      'updatedAt': updatedAt,
    };
  }

  static int _asInt(dynamic value, {int fallback = 0}) {
    if (value is int) return value;
    if (value is num) return value.round();
    return int.tryParse(value?.toString() ?? '') ?? fallback;
  }

  static double _asDouble(dynamic value, {double fallback = 0}) {
    if (value is num) return value.toDouble();
    return double.tryParse(value?.toString() ?? '') ?? fallback;
  }

  static String? _nullableString(dynamic value) {
    final text = value?.toString().trim() ?? '';
    return text.isEmpty ? null : text;
  }

  static bool _asBool(dynamic value, {bool fallback = false}) {
    if (value is bool) return value;
    if (value is num) return value != 0;
    final normalized = value?.toString().toLowerCase();
    if (normalized == 'true' || normalized == '1') return true;
    if (normalized == 'false' || normalized == '0') return false;
    return fallback;
  }
}

class MemoryCandidateDto {
  final String id;
  final String characterId;
  final String key;
  final String value;
  final String memoryType;
  final String memorySubtype;
  final int importance;
  final double confidence;
  final String sourceText;
  final String candidateKind;
  final String proposedAction;
  final String reason;
  final String createdAt;

  const MemoryCandidateDto({
    required this.id,
    this.characterId = '',
    this.key = '',
    this.value = '',
    this.memoryType = 'custom',
    this.memorySubtype = '',
    this.importance = 5,
    this.confidence = 0,
    this.sourceText = '',
    this.candidateKind = '',
    this.proposedAction = '',
    this.reason = '',
    this.createdAt = '',
  });

  factory MemoryCandidateDto.fromJson(Map<String, dynamic> json) {
    return MemoryCandidateDto(
      id: (json['id'] ?? '').toString(),
      characterId: (json['characterId'] ?? '').toString(),
      key: (json['key'] ?? '').toString(),
      value: (json['value'] ?? '').toString(),
      memoryType: (json['memoryType'] ?? 'custom').toString(),
      memorySubtype: (json['memorySubtype'] ?? '').toString(),
      importance: MemoryDto._asInt(json['importance'], fallback: 5),
      confidence: _normalizedConfidence(
        json['confidenceReal'] ?? json['confidence'],
      ),
      sourceText: (json['sourceText'] ?? '').toString(),
      candidateKind: (json['candidateKind'] ?? '').toString(),
      proposedAction: (json['proposedAction'] ?? '').toString(),
      reason: (json['reason'] ?? '').toString(),
      createdAt: (json['createdAt'] ?? '').toString(),
    );
  }

  static double _normalizedConfidence(dynamic value) {
    final parsed = value is num
        ? value.toDouble()
        : double.tryParse(value?.toString() ?? '') ?? 0;
    return parsed > 1 ? parsed / 100.0 : parsed;
  }
}
