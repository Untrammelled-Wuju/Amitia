class WorldBookDto {
  final String id;
  final String title;
  final String content;
  final List<String> keywords;
  final int priority;
  final int enabled;
  final String createdAt;

  WorldBookDto({
    required this.id,
    this.title = '',
    this.content = '',
    this.keywords = const [],
    this.priority = 0,
    this.enabled = 1,
    this.createdAt = '',
  });

  factory WorldBookDto.fromJson(Map<String, dynamic> json) {
    return WorldBookDto(
      id: (json['id'] ?? '').toString(),
      title: json['title'] as String? ?? '',
      content: json['content'] as String? ?? '',
      keywords: (json['keywords'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          [],
      priority: json['priority'] as int? ?? 0,
      enabled: json['enabled'] as int? ?? 1,
      createdAt: json['createdAt'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'title': title,
      'content': content,
      'keywords': keywords,
      'priority': priority,
      'enabled': enabled,
    };
  }
}
