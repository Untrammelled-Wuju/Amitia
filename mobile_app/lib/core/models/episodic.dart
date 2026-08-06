class EpisodicDto {
  final String id;
  final String title;
  final String content;
  final String summary;
  final String timestamp;
  final String emotion;
  final String createdAt;

  EpisodicDto({
    required this.id,
    this.title = '',
    this.content = '',
    this.summary = '',
    this.timestamp = '',
    this.emotion = '',
    this.createdAt = '',
  });

  factory EpisodicDto.fromJson(Map<String, dynamic> json) {
    return EpisodicDto(
      id: (json['id'] ?? '').toString(),
      title: json['title'] as String? ?? '',
      content: json['content'] as String? ?? '',
      summary: json['summary'] as String? ?? '',
      timestamp: json['timestamp'] as String? ?? '',
      emotion: json['emotion'] as String? ?? '',
      createdAt: json['createdAt'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'title': title,
      'content': content,
      'summary': summary,
      'timestamp': timestamp,
      'emotion': emotion,
    };
  }
}
