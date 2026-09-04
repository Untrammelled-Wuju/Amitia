class CharacterDto {
  final String id;
  final String name;
  final String avatar;
  final String identity;
  final String personality;
  final String speakingStyle;
  final String description;
  final String status;
  final int isActive;
  final bool isDefault;
  final String createdAt;
  final String? voiceType;
  final double? voiceSpeed;

  CharacterDto({
    required this.id,
    required this.name,
    this.avatar = '',
    this.identity = '',
    this.personality = '',
    this.speakingStyle = '',
    this.description = '',
    this.status = '',
    this.isActive = 0,
    this.isDefault = false,
    this.createdAt = '',
    this.voiceType,
    this.voiceSpeed,
  });

  factory CharacterDto.fromJson(Map<String, dynamic> json) {
    return CharacterDto(
      id: (json['id'] ?? '').toString(),
      name: json['name'] as String? ?? '',
      avatar: json['avatar'] as String? ?? '',
      identity: json['identity'] as String? ?? '',
      personality: json['personality'] as String? ?? '',
      speakingStyle: json['speakingStyle'] as String? ?? '',
      description: json['description'] as String? ?? '',
      status: json['status'] as String? ?? '',
      isActive: (json['isActive'] as num?)?.toInt() ?? 0,
      isDefault: json['isDefault'] == true || json['isDefault'] == 1,
      createdAt: json['createdAt'] as String? ?? '',
      voiceType: json['voiceType'] as String?,
      voiceSpeed: (json['voiceSpeed'] as num?)?.toDouble(),
    );
  }
}
