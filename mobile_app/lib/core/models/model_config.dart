class ModelConfigDto {
  final String id;
  final String name;
  final String provider;
  final String model;
  final String baseUrl;
  final int isActive;
  final int maxTokens;
  final double temperature;
  final int timeoutSeconds;
  final int retryCount;
  final bool hasApiKey;
  final bool supportsReasoning;
  final String defaultReasoningEffort;

  ModelConfigDto({
    required this.id,
    this.name = '',
    this.provider = '',
    this.model = '',
    this.baseUrl = '',
    this.isActive = 0,
    this.maxTokens = 4096,
    this.temperature = 0.7,
    this.timeoutSeconds = 60,
    this.retryCount = 1,
    this.hasApiKey = false,
    this.supportsReasoning = true,
    this.defaultReasoningEffort = 'high',
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
      timeoutSeconds: (json['timeoutSeconds'] as num?)?.toInt() ?? 60,
      retryCount: (json['retryCount'] as num?)?.toInt() ?? 1,
      hasApiKey: json['hasApiKey'] == true,
      supportsReasoning: json['supportsReasoning'] == null
          ? true
          : json['supportsReasoning'] == true,
      defaultReasoningEffort: (json['defaultReasoningEffort'] ?? 'high')
          .toString(),
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
      'timeoutSeconds': timeoutSeconds,
      'retryCount': retryCount,
      'hasApiKey': hasApiKey,
      'supportsReasoning': supportsReasoning,
      'defaultReasoningEffort': defaultReasoningEffort,
    };
  }
}
