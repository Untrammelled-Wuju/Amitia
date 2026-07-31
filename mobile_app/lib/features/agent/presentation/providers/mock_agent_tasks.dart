import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/widgets/amitia_misc.dart';

enum MockAgentTaskStatus {
  pending,
  waitingApproval,
  running,
  paused,
  completed,
  failed,
  cancelled,
}

class MockAgentTask {
  final String id;
  final String title;
  final String description;
  final List<String> requiredAbilities;
  final List<String> steps;
  final MockAgentTaskStatus status;
  final int progress;
  final int currentStepIndex;
  final String elapsed;
  final String? result;
  final String? error;
  final DateTime createdAt;

  MockAgentTask({
    required this.id,
    required this.title,
    required this.description,
    required this.requiredAbilities,
    required this.steps,
    required this.status,
    required this.progress,
    required this.currentStepIndex,
    required this.elapsed,
    this.result,
    this.error,
    required this.createdAt,
  });

  MockAgentTask copyWith({
    String? id,
    String? title,
    String? description,
    List<String>? requiredAbilities,
    List<String>? steps,
    MockAgentTaskStatus? status,
    int? progress,
    int? currentStepIndex,
    String? elapsed,
    String? result,
    String? error,
    DateTime? createdAt,
  }) {
    return MockAgentTask(
      id: id ?? this.id,
      title: title ?? this.title,
      description: description ?? this.description,
      requiredAbilities: requiredAbilities ?? this.requiredAbilities,
      steps: steps ?? this.steps,
      status: status ?? this.status,
      progress: progress ?? this.progress,
      currentStepIndex: currentStepIndex ?? this.currentStepIndex,
      elapsed: elapsed ?? this.elapsed,
      result: result ?? this.result,
      error: error ?? this.error,
      createdAt: createdAt ?? this.createdAt,
    );
  }
}

String mockAgentTaskStatusLabel(MockAgentTaskStatus status) {
  switch (status) {
    case MockAgentTaskStatus.pending:
      return '待开始';
    case MockAgentTaskStatus.waitingApproval:
      return '待审批';
    case MockAgentTaskStatus.running:
      return '运行中';
    case MockAgentTaskStatus.paused:
      return '已暂停';
    case MockAgentTaskStatus.completed:
      return '已完成';
    case MockAgentTaskStatus.failed:
      return '已失败';
    case MockAgentTaskStatus.cancelled:
      return '已取消';
  }
}

BadgeType mockAgentTaskBadgeType(MockAgentTaskStatus status) {
  switch (status) {
    case MockAgentTaskStatus.pending:
      return BadgeType.neutral;
    case MockAgentTaskStatus.waitingApproval:
      return BadgeType.warning;
    case MockAgentTaskStatus.running:
      return BadgeType.accent;
    case MockAgentTaskStatus.paused:
      return BadgeType.neutral;
    case MockAgentTaskStatus.completed:
      return BadgeType.success;
    case MockAgentTaskStatus.failed:
      return BadgeType.error;
    case MockAgentTaskStatus.cancelled:
      return BadgeType.neutral;
  }
}

final agentTasksProvider = StateProvider<List<MockAgentTask>>((ref) {
  return [
    MockAgentTask(
      id: 't1',
      title: '整理下载目录',
      description: '扫描下载目录并整理文件分类',
      requiredAbilities: ['文件系统', '目录扫描'],
      steps: ['扫描下载目录', '识别重复文件', '整理文件分类', '生成结果报告'],
      status: MockAgentTaskStatus.running,
      progress: 32,
      currentStepIndex: 1,
      elapsed: '00:18',
      createdAt: DateTime(2026, 7, 30, 9, 18),
    ),
    MockAgentTask(
      id: 't2',
      title: '生成周报摘要',
      description: '分析本周工作记录并生成摘要',
      requiredAbilities: ['数据分析', '文本生成'],
      steps: ['收集工作记录', '分析主要进展', '生成摘要', '输出报告'],
      status: MockAgentTaskStatus.running,
      progress: 65,
      currentStepIndex: 1,
      elapsed: '00:42',
      createdAt: DateTime(2026, 7, 30, 9, 0),
    ),
    MockAgentTask(
      id: 't3',
      title: '安装开发工具包',
      description: '安装并配置开发工具包',
      requiredAbilities: ['运行 Shell 命令', '访问下载目录'],
      steps: ['下载工具包', '校验完整性', '执行安装', '验证安装'],
      status: MockAgentTaskStatus.waitingApproval,
      progress: 0,
      currentStepIndex: 0,
      elapsed: '00:00',
      createdAt: DateTime(2026, 7, 30, 8, 30),
    ),
    MockAgentTask(
      id: 't4',
      title: '备份工作文档',
      description: '备份工作文档到指定位置',
      requiredAbilities: ['读取文档目录', '写入备份位置'],
      steps: ['确认备份范围', '复制文件', '校验备份', '完成'],
      status: MockAgentTaskStatus.pending,
      progress: 0,
      currentStepIndex: 0,
      elapsed: '00:00',
      createdAt: DateTime(2026, 7, 30, 8, 45),
    ),
    MockAgentTask(
      id: 't5',
      title: '整理桌面文件',
      description: '整理桌面文件分类',
      requiredAbilities: ['文件系统'],
      steps: ['扫描桌面', '分类文件', '移动文件', '生成报告'],
      status: MockAgentTaskStatus.completed,
      progress: 100,
      currentStepIndex: 3,
      elapsed: '01:23',
      result: '已整理 156 个文件，分类为 8 个文件夹',
      createdAt: DateTime(2026, 7, 29, 15, 0),
    ),
    MockAgentTask(
      id: 't6',
      title: '生成周报摘要',
      description: '生成本周工作摘要',
      requiredAbilities: ['数据分析'],
      steps: ['收集记录', '分析进展', '生成摘要'],
      status: MockAgentTaskStatus.completed,
      progress: 100,
      currentStepIndex: 2,
      elapsed: '02:15',
      result: '已生成本周工作摘要，包含 5 个主要进展',
      createdAt: DateTime(2026, 7, 29, 14, 0),
    ),
    MockAgentTask(
      id: 't7',
      title: '备份工作文档',
      description: '备份文档到指定位置',
      requiredAbilities: ['文件系统'],
      steps: ['复制文件', '校验备份'],
      status: MockAgentTaskStatus.failed,
      progress: 45,
      currentStepIndex: 1,
      elapsed: '00:55',
      error: '备份目标位置空间不足',
      createdAt: DateTime(2026, 7, 28, 18, 0),
    ),
  ];
});
