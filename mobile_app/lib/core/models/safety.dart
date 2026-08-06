class SafetyConfigDto {
  final int enabled;
  final List<String> sensitiveWords;
  final int blockLevel;

  SafetyConfigDto({
    this.enabled = 1,
    this.sensitiveWords = const [],
    this.blockLevel = 1,
  });

  factory SafetyConfigDto.fromJson(Map<String, dynamic> json) {
    return SafetyConfigDto(
      enabled: json['enabled'] as int? ?? 1,
      sensitiveWords: (json['sensitiveWords'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          [],
      blockLevel: json['blockLevel'] as int? ?? 1,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'enabled': enabled,
      'sensitiveWords': sensitiveWords,
      'blockLevel': blockLevel,
    };
  }
}
