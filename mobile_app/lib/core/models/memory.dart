class MemoryDto {
  final String id;
  final String characterId;
  final String memoryType;
  final String source;
  final String scope;
  final String key;
  final String value;
  final int importance;
  final int confidence;
  final String verifiedStatus;
  final int useCount;
  final String sensitivityLevel;
  final bool allowProactiveMention;
  final bool requiresConfirmation;
  final String createdAt;
  final String updatedAt;

  const MemoryDto({
    required this.id,
    this.characterId = '',
    this.memoryType = 'custom',
    this.source = 'manual',
    this.scope = 'character',
    this.key = '',
    this.value = '',
    this.importance = 0,
    this.confidence = 50,
    this.verifiedStatus = 'unverified',
    this.useCount = 0,
    this.sensitivityLevel = 'internal',
    this.allowProactiveMention = true,
    this.requiresConfirmation = false,
    this.createdAt = '',
    this.updatedAt = '',
  });

  /// Backward-compatible aliases used by existing UI widgets.
  String get content => value;
  String get type => memoryType;
  String get status => verifiedStatus;

  factory MemoryDto.fromJson(Map<String, dynamic> json) {
    return MemoryDto(
      id: (json['id'] ?? '').toString(),
      characterId: (json['characterId'] ?? '').toString(),
      memoryType: (json['memoryType'] ?? json['type'] ?? 'custom').toString(),
      source: (json['source'] ?? 'manual').toString(),
      scope: (json['scope'] ?? 'character').toString(),
      key: (json['key'] ?? '').toString(),
      value: (json['value'] ?? json['content'] ?? '').toString(),
      importance: _asInt(json['importance']),
      confidence: _asInt(json['confidence'], fallback: 50),
      verifiedStatus: (json['verifiedStatus'] ?? json['status'] ?? 'unverified').toString(),
      useCount: _asInt(json['useCount']),
      sensitivityLevel: (json['sensitivityLevel'] ?? 'internal').toString(),
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
      'source': source,
      'scope': scope,
      'key': key,
      'value': value,
      'importance': importance,
      'confidence': confidence,
      'verifiedStatus': verifiedStatus,
      'sensitivityLevel': sensitivityLevel,
      'allowProactiveMention': allowProactiveMention,
      'requiresConfirmation': requiresConfirmation,
    };
  }

  static int _asInt(dynamic value, {int fallback = 0}) {
    if (value is int) return value;
    if (value is num) return value.round();
    return int.tryParse(value?.toString() ?? '') ?? fallback;
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
    this.importance = 5,
    this.confidence = 0,
    this.sourceText = '',
    this.candidateKind = '',
    this.proposedAction = '',
    this.reason = '',
    this.createdAt = '',
  });

  /// Backward-compatible aliases for existing candidate UI.
  String get content => value.isNotEmpty ? value : sourceText;
  String get source => sourceText;
  String get status => proposedAction.isNotEmpty ? proposedAction : candidateKind;

  factory MemoryCandidateDto.fromJson(Map<String, dynamic> json) {
    return MemoryCandidateDto(
      id: (json['id'] ?? '').toString(),
      characterId: (json['characterId'] ?? '').toString(),
      key: (json['key'] ?? '').toString(),
      value: (json['value'] ?? json['content'] ?? '').toString(),
      memoryType: (json['memoryType'] ?? json['type'] ?? 'custom').toString(),
      importance: MemoryDto._asInt(json['importance'], fallback: 5),
      confidence: _asDouble(json['confidenceReal'] ?? json['confidence']),
      sourceText: (json['sourceText'] ?? json['source'] ?? '').toString(),
      candidateKind: (json['candidateKind'] ?? '').toString(),
      proposedAction: (json['proposedAction'] ?? json['status'] ?? '').toString(),
      reason: (json['reason'] ?? '').toString(),
      createdAt: (json['createdAt'] ?? '').toString(),
    );
  }

  static double _asDouble(dynamic value) {
    if (value is num) return value.toDouble();
    return double.tryParse(value?.toString() ?? '') ?? 0;
  }
}
