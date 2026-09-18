class MoodDto {
  final String id;
  final String messageId;
  final String conversationId;
  final String mood;
  final int valence;
  final int arousal;
  final String createdAt;

  MoodDto({
    required this.id,
    this.messageId = '',
    this.conversationId = '',
    this.mood = '',
    this.valence = 0,
    this.arousal = 0,
    this.createdAt = '',
  });

  factory MoodDto.fromJson(Map<String, dynamic> json) {
    final messageId = (json['messageId'] ?? json['id'] ?? '').toString();
    final mood = (json['moodLabel'] ?? json['mood'] ?? json['name'] ?? '').toString();
    return MoodDto(
      id: (json['id'] ?? messageId).toString(),
      messageId: messageId,
      conversationId: (json['conversationId'] ?? '').toString(),
      mood: mood,
      valence: _asInt(json['valence']),
      arousal: _asInt(json['arousal']),
      createdAt: (json['createdAt'] ?? json['lastDetected'] ?? '').toString(),
    );
  }

  static int _asInt(dynamic value) {
    if (value is num) return value.toInt();
    return int.tryParse(value?.toString() ?? '') ?? 0;
  }
}
