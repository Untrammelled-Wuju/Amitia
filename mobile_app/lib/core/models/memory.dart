class MemoryDto {
  final String id;
  final String content;
  final String type;
  final int importance;
  final String status;
  final int useCount;
  final String createdAt;
  final String updatedAt;

  MemoryDto({
    required this.id,
    this.content = '',
    this.type = '',
    this.importance = 0,
    this.status = '',
    this.useCount = 0,
    this.createdAt = '',
    this.updatedAt = '',
  });

  factory MemoryDto.fromJson(Map<String, dynamic> json) {
    return MemoryDto(
      id: (json['id'] ?? '').toString(),
      content: json['content'] as String? ?? '',
      type: json['type'] as String? ?? '',
      importance: json['importance'] as int? ?? 0,
      status: json['status'] as String? ?? '',
      useCount: json['useCount'] as int? ?? 0,
      createdAt: json['createdAt'] as String? ?? '',
      updatedAt: json['updatedAt'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'content': content,
      'type': type,
      'importance': importance,
      'status': status,
    };
  }
}

class MemoryCandidateDto {
  final String id;
  final String content;
  final String source;
  final String status;
  final String createdAt;

  MemoryCandidateDto({
    required this.id,
    this.content = '',
    this.source = '',
    this.status = '',
    this.createdAt = '',
  });

  factory MemoryCandidateDto.fromJson(Map<String, dynamic> json) {
    return MemoryCandidateDto(
      id: (json['id'] ?? '').toString(),
      content: json['content'] as String? ?? '',
      source: json['source'] as String? ?? '',
      status: json['status'] as String? ?? '',
      createdAt: json['createdAt'] as String? ?? '',
    );
  }
}
