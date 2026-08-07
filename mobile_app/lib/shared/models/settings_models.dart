import 'package:flutter/material.dart';

enum ModelType { llm, voice, embedding, vision, imagegen }

class ModelConfig {
  final String id;
  final String name;
  final ModelType type;
  final String provider;
  final String baseUrl;
  final String modelName;
  final String apiKey;
  final bool isActive;
  final String? assignedScene;
  final Map<String, dynamic> params;

  ModelConfig({
    required this.id,
    required this.name,
    required this.type,
    required this.provider,
    this.baseUrl = '',
    this.modelName = '',
    this.apiKey = '',
    this.isActive = false,
    this.assignedScene,
    this.params = const {},
  });
}

class SafetySettings {
  final String permissionMode;
  final bool sensitiveOperationApproval;
  final bool dataAccessApproval;
  final bool modelOutputBoundary;
  final int safetyEventCount;
  final int privacyIssueCount;

  SafetySettings({
    this.permissionMode = '手动审批',
    this.sensitiveOperationApproval = true,
    this.dataAccessApproval = true,
    this.modelOutputBoundary = true,
    this.safetyEventCount = 3,
    this.privacyIssueCount = 1,
  });
}

class MaintenanceCheck {
  final String name;
  final String status;
  final String? detail;

  MaintenanceCheck({required this.name, required this.status, this.detail});
}

class StorageInfo {
  final String category;
  final String size;
  final double percentage;
  final Color color;

  StorageInfo({
    required this.category,
    required this.size,
    required this.percentage,
    required this.color,
  });
}

class ThemeSettings {
  final ThemeMode mode;
  final double fontScale;
  final bool animationEnabled;

  ThemeSettings({
    this.mode = ThemeMode.light,
    this.fontScale = 1.0,
    this.animationEnabled = true,
  });
}

class UserSettings {
  final String avatar;
  final String username;
  final String nickname;
  final String userLabel;
  final String bio;

  UserSettings({
    this.avatar = 'U',
    this.username = 'user',
    this.nickname = '用户',
    this.userLabel = '主人',
    this.bio = '',
  });
}

class PrivacyScanResult {
  final String category;
  final int riskCount;
  final String riskLevel;

  PrivacyScanResult({
    required this.category,
    required this.riskCount,
    required this.riskLevel,
  });
}

class TimeAnchor {
  final String id;
  final String name;
  final String type;
  final String value;

  TimeAnchor({
    required this.id,
    required this.name,
    required this.type,
    required this.value,
  });
}

class DeploymentConfig {
  final String mode;
  final String description;

  DeploymentConfig({
    this.mode = '本地',
    this.description = '完整功能本地运行',
  });
}

class AiConfig {
  final String defaultCharacter;
  final String defaultModel;
  final String contextStrategy;
  final bool streamingOutput;
  final bool messageSplitting;
  final bool toolCalls;
  final String errorFallback;

  AiConfig({
    this.defaultCharacter = 'Amitia',
    this.defaultModel = 'GPT-4',
    this.contextStrategy = '滑动窗口',
    this.streamingOutput = true,
    this.messageSplitting = true,
    this.toolCalls = true,
    this.errorFallback = '简单回复',
  });
}
