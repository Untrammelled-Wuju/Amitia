class ExtensionDto {
  final String id;
  final String name;
  final String description;
  final String version;
  final int enabled;
  final String author;
  final String icon;

  ExtensionDto({
    required this.id,
    this.name = '',
    this.description = '',
    this.version = '',
    this.enabled = 0,
    this.author = '',
    this.icon = '',
  });

  factory ExtensionDto.fromJson(Map<String, dynamic> json) {
    return ExtensionDto(
      id: (json['id'] ?? '').toString(),
      name: json['name'] as String? ?? '',
      description: json['description'] as String? ?? '',
      version: json['version'] as String? ?? '',
      enabled: json['enabled'] as int? ?? 0,
      author: json['author'] as String? ?? '',
      icon: json['icon'] as String? ?? '',
    );
  }
}

class AgentSkillDto {
  final String id;
  final String name;
  final String description;
  final int enabled;
  final int status;
  final String createdAt;

  AgentSkillDto({
    required this.id,
    this.name = '',
    this.description = '',
    this.enabled = 0,
    this.status = 0,
    this.createdAt = '',
  });

  factory AgentSkillDto.fromJson(Map<String, dynamic> json) {
    return AgentSkillDto(
      id: (json['id'] ?? '').toString(),
      name: json['name'] as String? ?? '',
      description: json['description'] as String? ?? '',
      enabled: json['enabled'] as int? ?? 0,
      status: json['status'] as int? ?? 0,
      createdAt: json['createdAt'] as String? ?? '',
    );
  }
}
