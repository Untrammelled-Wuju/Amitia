class ModelConfigDto {
  final String id;
  final String name;
  final String provider;
  final String model;
  final String baseUrl;
  final int isActive;
  final int maxTokens;
  final double temperature;

  ModelConfigDto({
    required this.id,
    this.name = '',
    this.provider = '',
    this.model = '',
    this.baseUrl = '',
    this.isActive = 0,
    this.maxTokens = 4096,
    this.temperature = 0.7,
  });

  factory ModelConfigDto.fromJson(Map<String, dynamic> json) {
    return ModelConfigDto(
      id: (json['id'] ?? '').toString(),
      name: json['name'] as String? ?? '',
      provider: (json['apiType'] ?? json['provider'] ?? '').toString(),
      model: (json['modelName'] ?? json['model'] ?? '').toString(),
      baseUrl: json['baseUrl'] as String? ?? '',
      isActive: json['isActive'] as int? ?? 0,
      maxTokens: json['maxTokens'] as int? ?? 4096,
      temperature: (json['temperature'] as num?)?.toDouble() ?? 0.7,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'name': name,
      'apiType': provider,
      'modelName': model,
      'baseUrl': baseUrl,
      'isActive': isActive,
      'maxTokens': maxTokens,
      'temperature': temperature,
    };
  }
}
