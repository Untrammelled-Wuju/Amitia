class SafetyConfigDto {
  final bool preventEmotionalBlackmail;
  final bool preventExclusiveDependency;
  final bool preventRealityIsolation;
  final bool preventPunitiveExpression;
  final bool preventPretendingHuman;
  final bool preventSensitiveProactiveMention;
  final bool restrictAdultContent;
  final int negativeEmotionCap;
  final int intimacyExpressionCap;
  final String violationAction;
  final int auditLogRetentionDays;

  const SafetyConfigDto({
    this.preventEmotionalBlackmail = true,
    this.preventExclusiveDependency = true,
    this.preventRealityIsolation = true,
    this.preventPunitiveExpression = true,
    this.preventPretendingHuman = true,
    this.preventSensitiveProactiveMention = true,
    this.restrictAdultContent = true,
    this.negativeEmotionCap = 5,
    this.intimacyExpressionCap = 7,
    this.violationAction = 'block',
    this.auditLogRetentionDays = 30,
  });

  factory SafetyConfigDto.fromJson(Map<String, dynamic> json) {
    bool boolValue(String key, bool fallback) {
      final value = json[key];
      if (value is bool) return value;
      if (value is num) return value != 0;
      if (value is String) {
        final normalized = value.trim().toLowerCase();
        if (normalized == 'true' || normalized == '1') return true;
        if (normalized == 'false' || normalized == '0') return false;
      }
      return fallback;
    }

    int intValue(String key, int fallback) {
      final value = json[key];
      if (value is num) return value.toInt();
      return int.tryParse(value?.toString() ?? '') ?? fallback;
    }

    return SafetyConfigDto(
      preventEmotionalBlackmail: boolValue('preventEmotionalBlackmail', true),
      preventExclusiveDependency: boolValue('preventExclusiveDependency', true),
      preventRealityIsolation: boolValue('preventRealityIsolation', true),
      preventPunitiveExpression: boolValue('preventPunitiveExpression', true),
      preventPretendingHuman: boolValue('preventPretendingHuman', true),
      preventSensitiveProactiveMention: boolValue('preventSensitiveProactiveMention', true),
      restrictAdultContent: boolValue('restrictAdultContent', true),
      negativeEmotionCap: intValue('negativeEmotionCap', 5).clamp(0, 10).toInt(),
      intimacyExpressionCap: intValue('intimacyExpressionCap', 7).clamp(0, 10).toInt(),
      violationAction: (json['violationAction'] ?? 'block').toString(),
      auditLogRetentionDays: intValue('auditLogRetentionDays', 30).clamp(1, 3650).toInt(),
    );
  }

  Map<String, dynamic> toJson() => {
        'preventEmotionalBlackmail': preventEmotionalBlackmail,
        'preventExclusiveDependency': preventExclusiveDependency,
        'preventRealityIsolation': preventRealityIsolation,
        'preventPunitiveExpression': preventPunitiveExpression,
        'preventPretendingHuman': preventPretendingHuman,
        'preventSensitiveProactiveMention': preventSensitiveProactiveMention,
        'restrictAdultContent': restrictAdultContent,
        'negativeEmotionCap': negativeEmotionCap,
        'intimacyExpressionCap': intimacyExpressionCap,
        'violationAction': violationAction,
        'auditLogRetentionDays': auditLogRetentionDays,
      };
}
