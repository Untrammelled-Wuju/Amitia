class VoiceConfigDto {
  final String id;
  final String name;
  final String provider;
  final String apiKey;
  final String baseUrl;
  final String resourceId;
  final String voiceId;
  final double speed;
  final double pitch;
  final double volume;
  final int isActive;
  final String customVoiceId;
  final String realtimeAppId;
  final String realtimeAccessToken;
  final String realtimeSecretKey;

  VoiceConfigDto({
    required this.id,
    this.name = '',
    this.provider = '',
    this.apiKey = '',
    this.baseUrl = '',
    this.resourceId = '',
    this.voiceId = '',
    this.speed = 1,
    this.pitch = 1,
    this.volume = 1,
    this.isActive = 0,
    this.customVoiceId = '',
    this.realtimeAppId = '',
    this.realtimeAccessToken = '',
    this.realtimeSecretKey = '',
  });

  factory VoiceConfigDto.fromJson(Map<String, dynamic> json) {
    return VoiceConfigDto(
      id: (json['id'] ?? '').toString(),
      name: json['name'] as String? ?? '',
      provider: (json['apiType'] ?? json['provider'] ?? '').toString(),
      apiKey: json['apiKey'] as String? ?? '',
      baseUrl: json['baseUrl'] as String? ?? '',
      resourceId: (json['resourceId'] ?? json['resource'] ?? '').toString(),
      voiceId: (json['voiceType'] ?? json['voiceId'] ?? '').toString(),
      speed: (json['speed'] as num?)?.toDouble() ?? 1,
      pitch: (json['pitch'] as num?)?.toDouble() ?? 1,
      volume: (json['volume'] as num?)?.toDouble() ?? 1,
      isActive: (json['isActive'] as num?)?.toInt() ?? 0,
      customVoiceId: json['customVoiceId'] as String? ?? '',
      realtimeAppId: json['realtimeAppId'] as String? ?? '',
      realtimeAccessToken: json['realtimeAccessToken'] as String? ?? '',
      realtimeSecretKey: json['realtimeSecretKey'] as String? ?? '',
    );
  }

  String get realtimeApiKey => realtimeAccessToken.isNotEmpty ? realtimeAccessToken : apiKey;

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'name': name,
      'apiType': provider,
      'apiKey': apiKey,
      'baseUrl': baseUrl,
      'resourceId': resourceId,
      'voiceType': voiceId,
      'speed': speed,
      'pitch': pitch,
      'volume': volume,
      'isActive': isActive,
      'customVoiceId': customVoiceId,
      'realtimeAppId': realtimeAppId,
      'realtimeAccessToken': realtimeAccessToken,
      'realtimeSecretKey': realtimeSecretKey,
    };
  }
}
