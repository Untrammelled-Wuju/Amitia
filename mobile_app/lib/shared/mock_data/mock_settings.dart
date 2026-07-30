import 'package:flutter/material.dart';
import '../models/models.dart';

class MockSettings {
  MockSettings._();

  static List<ModelConfig> modelConfigs = [
    ModelConfig(id: 'mc1', name: 'GPT-4 主力', type: ModelType.llm, provider: 'OpenAI', baseUrl: 'https://api.openai.com/v1', modelName: 'gpt-4', isActive: true, assignedScene: '默认对话'),
    ModelConfig(id: 'mc2', name: 'Claude 备选', type: ModelType.llm, provider: 'Anthropic', baseUrl: 'https://api.anthropic.com', modelName: 'claude-3'),
    ModelConfig(id: 'mc3', name: 'GPT-4o 语音', type: ModelType.voice, provider: 'OpenAI', baseUrl: 'https://api.openai.com/v1', modelName: 'gpt-4o-mini-tts', isActive: true),
    ModelConfig(id: 'mc4', name: '默认向量', type: ModelType.embedding, provider: 'OpenAI', baseUrl: 'https://api.openai.com/v1', modelName: 'text-embedding-3-small', isActive: true),
    ModelConfig(id: 'mc5', name: 'GPT-4o 视觉', type: ModelType.vision, provider: 'OpenAI', baseUrl: 'https://api.openai.com/v1', modelName: 'gpt-4o', isActive: true),
    ModelConfig(id: 'mc6', name: 'DALL-E 3', type: ModelType.imagegen, provider: 'OpenAI', baseUrl: 'https://api.openai.com/v1', modelName: 'dall-e-3'),
  ];

  static SafetySettings safetySettings = SafetySettings();
  static AiConfig aiConfig = AiConfig();
  static UserSettings userSettings = UserSettings();

  static List<MaintenanceCheck> maintenanceChecks = [
    MaintenanceCheck(name: '后端服务', status: '正常'),
    MaintenanceCheck(name: '数据库连接', status: '正常'),
    MaintenanceCheck(name: 'SurrealDB', status: '正常'),
    MaintenanceCheck(name: 'Qdrant 向量库', status: '正常'),
    MaintenanceCheck(name: 'MCP Runtime', status: '已停止', detail: '无运行中的 MCP 服务'),
    MaintenanceCheck(name: '文件系统', status: '正常'),
    MaintenanceCheck(name: '缓存状态', status: '正常'),
    MaintenanceCheck(name: '数据一致性', status: '正常'),
  ];

  static List<StorageInfo> storageInfo = [
    StorageInfo(category: '对话数据', size: '1.2 GB', percentage: 49, color: const Color(0xFF7668EE)),
    StorageInfo(category: '媒体文件', size: '680 MB', percentage: 28, color: const Color(0xFF52B788)),
    StorageInfo(category: '记忆数据', size: '320 MB', percentage: 13, color: const Color(0xFFE9A23B)),
    StorageInfo(category: '扩展数据', size: '150 MB', percentage: 6, color: const Color(0xFF6C8FEA)),
    StorageInfo(category: '缓存', size: '83 MB', percentage: 4, color: const Color(0xFFE66767)),
  ];

  static List<PrivacyScanResult> privacyScanResults = [
    PrivacyScanResult(category: '对话记录', riskCount: 2, riskLevel: '中风险'),
    PrivacyScanResult(category: '记忆数据', riskCount: 1, riskLevel: '低风险'),
    PrivacyScanResult(category: '用户画像', riskCount: 0, riskLevel: '安全'),
    PrivacyScanResult(category: '聊天记录', riskCount: 3, riskLevel: '高风险'),
  ];

  static List<TimeAnchor> timeAnchors = [
    TimeAnchor(id: 'ta1', name: '起床', type: '周期锚点', value: '07:00'),
    TimeAnchor(id: 'ta2', name: '午休', type: '周期锚点', value: '12:30'),
    TimeAnchor(id: 'ta3', name: '睡觉', type: '周期锚点', value: '23:00'),
    TimeAnchor(id: 'ta4', name: '生日', type: '特殊日期', value: '1995-06-15'),
  ];

  static DeploymentConfig deploymentConfig = DeploymentConfig();
}
