class VoiceConfigDto {
  final String id;
  final String name;
  final String provider;
  final String voiceId;
  final int speed;
  final int pitch;
  final int isActive;

  VoiceConfigDto({
    required this.id,
    this.name = '',
    this.provider = '',
    this.voiceId = '',
    this.speed = 1,
    this.pitch = 1,
    this.isActive = 0,
  });

  factory VoiceConfigDto.fromJson(Map<String, dynamic> json) {
    return VoiceConfigDto(
      id: (json['id'] ?? '').toString(),
      name: json['name'] as String? ?? '',
      provider: json['provider'] as String? ?? '',
      voiceId: json['voiceId'] as String? ?? '',
      speed: json['speed'] as int? ?? 1,
      pitch: json['pitch'] as int? ?? 1,
      isActive: json['isActive'] as int? ?? 0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'name': name,
      'provider': provider,
      'voiceId': voiceId,
      'speed': speed,
      'pitch': pitch,
      'isActive': isActive,
    };
  }
}
