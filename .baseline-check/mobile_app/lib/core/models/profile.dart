class ProfileDto {
  final String id;
  final String userId;
  final String characterId;
  final String category;
  final String attributeName;
  final String attributeValue;
  final int confidence;
  final String source;
  final String sourceConvId;
  final String verifiedAt;
  final String createdAt;
  final String updatedAt;

  const ProfileDto({
    required this.id,
    this.userId = '',
    this.characterId = '',
    this.category = 'personal_info',
    this.attributeName = '',
    this.attributeValue = '',
    this.confidence = 50,
    this.source = '',
    this.sourceConvId = '',
    this.verifiedAt = '',
    this.createdAt = '',
    this.updatedAt = '',
  });

  factory ProfileDto.fromJson(Map<String, dynamic> json) {
    return ProfileDto(
      id: (json['id'] ?? '').toString(),
      userId: (json['userId'] ?? '').toString(),
      characterId: (json['characterId'] ?? '').toString(),
      category: (json['category'] ?? 'personal_info').toString(),
      attributeName: (json['attributeName'] ?? '').toString(),
      attributeValue: (json['attributeValue'] ?? '').toString(),
      confidence: (json['confidence'] as num?)?.toInt() ?? 50,
      source: (json['source'] ?? '').toString(),
      sourceConvId: (json['sourceConvId'] ?? '').toString(),
      verifiedAt: (json['verifiedAt'] ?? '').toString(),
      createdAt: (json['createdAt'] ?? '').toString(),
      updatedAt: (json['updatedAt'] ?? '').toString(),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'userId': userId,
        'characterId': characterId,
        'category': category,
        'attributeName': attributeName,
        'attributeValue': attributeValue,
        'confidence': confidence,
        'source': source,
        'sourceConvId': sourceConvId,
        'verifiedAt': verifiedAt,
        'createdAt': createdAt,
        'updatedAt': updatedAt,
      };
}
