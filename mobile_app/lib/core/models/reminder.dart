class ReminderDto {
  final String id;
  final String title;
  final String content;
  final String cronExpr;
  final int enabled;
  final String lastTriggeredAt;
  final String createdAt;

  ReminderDto({
    required this.id,
    this.title = '',
    this.content = '',
    this.cronExpr = '',
    this.enabled = 1,
    this.lastTriggeredAt = '',
    this.createdAt = '',
  });

  factory ReminderDto.fromJson(Map<String, dynamic> json) {
    return ReminderDto(
      id: (json['id'] ?? '').toString(),
      title: json['title'] as String? ?? '',
      content: json['content'] as String? ?? '',
      cronExpr: json['cronExpr'] as String? ?? '',
      enabled: json['enabled'] as int? ?? 1,
      lastTriggeredAt: json['lastTriggeredAt'] as String? ?? '',
      createdAt: json['createdAt'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'title': title,
      'content': content,
      'cronExpr': cronExpr,
      'enabled': enabled,
    };
  }
}
