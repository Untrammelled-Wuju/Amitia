import 'dart:convert';

class CharacterDto {
  final String id;
  final String name;
  final String avatar;
  final String identity;
  final String personality;
  final String speakingStyle;
  final String relationshipStyle;
  final String characterBase;
  final String boundaryRules;
  final String description;
  final String basePrompt;
  final String status;
  final int isActive;
  final bool isDefault;
  final String createdAt;
  final String? voiceType;
  final double? voiceSpeed;
  final double? voicePitch;
  final double? voiceVolume;
  final String customVoiceId;
  final String gender;
  final String pronoun;
  final String selfReference;
  final String userAddressingStyle;
  final int genderExpression;
  final String lifeIdentity;
  final Map<String, dynamic> personalityConfig;
  final Map<String, dynamic> chatStyleConfig;
  final Map<String, dynamic> sceneRules;

  CharacterDto({
    required this.id,
    required this.name,
    this.avatar = '',
    this.identity = '',
    this.personality = '',
    this.speakingStyle = '',
    this.relationshipStyle = '',
    this.characterBase = '',
    this.boundaryRules = '',
    this.description = '',
    this.basePrompt = '',
    this.status = '',
    this.isActive = 0,
    this.isDefault = false,
    this.createdAt = '',
    this.voiceType,
    this.voiceSpeed,
    this.voicePitch,
    this.voiceVolume,
    this.customVoiceId = '',
    this.gender = '',
    this.pronoun = '',
    this.selfReference = '',
    this.userAddressingStyle = '',
    this.genderExpression = 30,
    this.lifeIdentity = '',
    this.personalityConfig = const <String, dynamic>{},
    this.chatStyleConfig = const <String, dynamic>{},
    this.sceneRules = const <String, dynamic>{},
  });

  factory CharacterDto.fromJson(Map<String, dynamic> json) {
    return CharacterDto(
      id: (json['id'] ?? '').toString(),
      name: json['name'] as String? ?? '',
      avatar: json['avatar'] as String? ?? '',
      identity: json['identity'] as String? ?? '',
      personality: json['personality'] as String? ?? '',
      speakingStyle: json['speakingStyle'] as String? ?? '',
      relationshipStyle: json['relationshipStyle'] as String? ?? '',
      characterBase: json['characterBase'] as String? ?? '',
      boundaryRules: json['boundaryRules'] as String? ?? '',
      description: json['description'] as String? ?? '',
      basePrompt: json['basePrompt'] as String? ?? '',
      status: json['status'] as String? ?? '',
      isActive: (json['isActive'] as num?)?.toInt() ?? 0,
      isDefault: json['isDefault'] == true || json['isDefault'] == 1,
      createdAt: json['createdAt'] as String? ?? '',
      voiceType: json['voiceType'] as String?,
      voiceSpeed: (json['voiceSpeed'] as num?)?.toDouble(),
      voicePitch: (json['voicePitch'] as num?)?.toDouble(),
      voiceVolume: (json['voiceVolume'] as num?)?.toDouble(),
      customVoiceId: json['customVoiceId'] as String? ?? '',
      gender: json['gender'] as String? ?? '',
      pronoun: json['pronoun'] as String? ?? '',
      selfReference: json['selfReference'] as String? ?? '',
      userAddressingStyle: json['userAddressingStyle'] as String? ?? '',
      genderExpression: (json['genderExpression'] as num?)?.toInt() ?? 30,
      lifeIdentity: json['lifeIdentity'] as String? ?? '',
      personalityConfig: _mapValue(json['personalityConfig']),
      chatStyleConfig: _mapValue(json['chatStyleConfig']),
      sceneRules: _mapValue(json['sceneRules']),
    );
  }

  static Map<String, dynamic> _mapValue(dynamic value) {
    if (value is Map<String, dynamic>) return Map<String, dynamic>.from(value);
    if (value is Map) {
      return value.map((key, item) => MapEntry(key.toString(), item));
    }
    if (value is String && value.trim().isNotEmpty) {
      try {
        final decoded = jsonDecode(value);
        if (decoded is Map) {
          return decoded.map((key, item) => MapEntry(key.toString(), item));
        }
      } catch (_) {}
    }
    return const <String, dynamic>{};
  }
}
