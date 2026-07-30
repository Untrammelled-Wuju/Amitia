import '../models/models.dart';

class MockWorkshop {
  MockWorkshop._();

  static List<WorkshopSession> skillSessions = [
    WorkshopSession(id: 'ws1', title: '文件分类助手', type: 'Skill', status: '已完成', updated: DateTime(2026, 7, 29)),
    WorkshopSession(id: 'ws2', title: '邮件摘要生成', type: 'Skill', status: '进行中', updated: DateTime(2026, 7, 30)),
    WorkshopSession(id: 'ws3', title: '代码审查员', type: 'Skill', status: '草稿', updated: DateTime(2026, 7, 28)),
  ];

  static List<SkillDraft> skillDrafts = [
    SkillDraft(id: 'sd1', name: '文件分类助手', description: '根据文件类型自动分类到不同目录', metadata: '版本 1.0.0', inputSchema: '{"type": "object", "properties": {"path": {"type": "string"}}}', outputSchema: '{"type": "object", "properties": {"result": {"type": "string"}}}', riskAssessment: '低风险', testResult: '通过', status: '已完成', updated: DateTime(2026, 7, 29)),
    SkillDraft(id: 'sd2', name: '邮件摘要生成', description: '自动生成邮件摘要', metadata: '版本 0.9.0', status: '草稿', updated: DateTime(2026, 7, 30)),
  ];

  static List<PetTask> petTasks = [
    PetTask(id: 'pt1', name: '阿米娅桌宠', characterName: '阿米娅', totalActions: 8, completedActions: 8, status: PetTaskStatus.completed, progress: 100, createdAt: DateTime(2026, 7, 25)),
    PetTask(id: 'pt2', name: '小雨桌宠', characterName: '小雨', totalActions: 8, completedActions: 5, status: PetTaskStatus.processing, progress: 62, createdAt: DateTime(2026, 7, 28)),
    PetTask(id: 'pt3', name: 'Karin 桌宠', characterName: 'Karin', totalActions: 8, completedActions: 0, status: PetTaskStatus.pending, progress: 0, createdAt: DateTime(2026, 7, 30)),
  ];

  static List<ProcessingTask> processingTasks(String petTaskId) => [
    ProcessingTask(
      id: 'pr1', petTaskId: petTaskId, actionKey: 'idle', actionName: '待机', totalFrames: 8, completedFrames: 8,
      status: ProcessingStatus.approved, qualityStatus: '高质量',
      frames: List.generate(8, (i) => FrameEntry(index: i, status: '已完成', qualityLabel: i % 3 == 0 ? '高质量' : '合格')),
      attempts: [AttemptEntry(id: 'a1', label: '第1次', isSelected: true), AttemptEntry(id: 'a2', label: '第2次')],
    ),
    ProcessingTask(
      id: 'pr2', petTaskId: petTaskId, actionKey: 'wave', actionName: '招手', totalFrames: 8, completedFrames: 6,
      status: ProcessingStatus.reviewing, qualityStatus: '待审核',
      frames: List.generate(8, (i) => FrameEntry(index: i, status: i < 6 ? '已完成' : '等待中', qualityLabel: i < 6 ? '合格' : null)),
      attempts: [AttemptEntry(id: 'a1', label: '第1次', isSelected: true)],
    ),
    ProcessingTask(
      id: 'pr3', petTaskId: petTaskId, actionKey: 'happy', actionName: '开心', totalFrames: 8, completedFrames: 4,
      status: ProcessingStatus.reviewing, qualityStatus: '部分不合格',
      frames: List.generate(8, (i) => FrameEntry(index: i, status: i < 4 ? '已完成' : '等待中', qualityLabel: i < 4 ? (i == 1 ? '不合格' : '合格') : null)),
      attempts: [AttemptEntry(id: 'a1', label: '第1次'), AttemptEntry(id: 'a2', label: '第2次', isSelected: true)],
    ),
    ProcessingTask(
      id: 'pr4', petTaskId: petTaskId, actionKey: 'speaking', actionName: '说话', totalFrames: 8, completedFrames: 0,
      status: ProcessingStatus.pending, qualityStatus: '未开始',
      frames: List.generate(8, (i) => FrameEntry(index: i, status: '等待中')),
      attempts: [],
    ),
  ];

  static List<PetInstallation> installations = [
    PetInstallation(id: 'pi1', name: '阿米娅桌宠', characterName: '阿米娅', isEnabled: true, isRunning: true, scale: 1.0, defaultAction: 'idle', actions: ['idle', 'wave', 'happy', 'speaking']),
    PetInstallation(id: 'pi2', name: '小雨桌宠', characterName: '小雨', isEnabled: false, isRunning: false, scale: 0.8, defaultAction: 'idle', actions: ['idle', 'wave']),
  ];
}
