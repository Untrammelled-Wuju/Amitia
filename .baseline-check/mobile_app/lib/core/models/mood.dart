class MoodDto {
  final String id;
  final String conversationId;
  final String mood;
  final int valence;
  final int arousal;
  final String createdAt;

  MoodDto({
    required this.id,
    this.conversationId = '',
    this.mood = '',
    this.valence = 0,
    this.arousal = 0,
    this.createdAt = '',
  });

  factory MoodDto.fromJson(Map<String, dynamic> json) {
    return MoodDto(
      id: (json['id'] ?? '').toString(),
      conversationId: (json['conversationId'] ?? '').toString(),
      mood: json['mood'] as String? ?? '',
      valence: json['valence'] as int? ?? 0,
      arousal: json['arousal'] as int? ?? 0,
      createdAt: json['createdAt'] as String? ?? '',
    );
  }
}
