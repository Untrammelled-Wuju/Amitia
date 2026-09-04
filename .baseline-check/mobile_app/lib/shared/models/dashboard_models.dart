class DashboardRunInfo {
  final String backendStatus;
  final String agentRuntimeStatus;
  final String modelStatus;
  final String databaseStatus;
  final String channelStatus;
  final List<String> recentErrors;
  final String accessRisk;
  final List<String> recentTasks;
  final String psycheSummary;

  DashboardRunInfo({
    this.backendStatus = '运行中',
    this.agentRuntimeStatus = '空闲',
    this.modelStatus = 'GPT-4 已连接',
    this.databaseStatus = '正常',
    this.channelStatus = '微信已连接',
    this.recentErrors = const [],
    this.accessRisk = '低风险',
    this.recentTasks = const [],
    this.psycheSummary = '心情愉快，状态稳定',
  });
}

class DashboardDataInfo {
  final int conversationCount;
  final int characterCount;
  final int memoryCount;
  final int proactiveMessageCount;
  final int extensionCount;
  final String storageUsage;
  final List<String> recentImports;
  final int errorCount;

  DashboardDataInfo({
    this.conversationCount = 3421,
    this.characterCount = 4,
    this.memoryCount = 128,
    this.proactiveMessageCount = 56,
    this.extensionCount = 5,
    this.storageUsage = '2.43 GB',
    this.recentImports = const [],
    this.errorCount = 3,
  });
}
