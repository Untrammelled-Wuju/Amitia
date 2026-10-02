import '../../../core/models/conversation.dart';

class AutomationStatus {
  const AutomationStatus({
    required this.phase,
    required this.label,
    required this.detail,
    required this.count,
  });

  final String phase;
  final String label;
  final String detail;
  final int count;
}

String? automationAction(String name) {
  final normalized = name.toLowerCase().replaceAll('_', '.');
  if (normalized == 'browser.agent.run') return '正在自动化操作';
  if (!RegExp(
    r'^(android\.(interaction|virtual\.display|ui\.tree|screen)\.|browser\.(interact|dom|navigate|resource\.screenshot|press\.key))',
  ).hasMatch(normalized)) {
    return null;
  }
  if (RegExp(
    r'\.(status|list|get|permission|create|release|resize|start|stop)(\.|$)',
  ).hasMatch(normalized)) {
    return null;
  }
  if (RegExp(
    r'snapshot|screenshot|capture|visual\.locate|dom\.find|screen\.frame\.latest',
  ).hasMatch(normalized)) {
    return '正在观察画面';
  }
  if (RegExp(r'click|\.tap$').hasMatch(normalized)) return '正在点击';
  if (RegExp(r'input|\.text$|fill|clear\.text').hasMatch(normalized)) {
    return '正在输入';
  }
  if (RegExp(r'scroll|swipe').hasMatch(normalized)) return '正在滚动';
  if (RegExp(r'navigate|launch').hasMatch(normalized)) return '正在打开页面';
  return '正在操作';
}

AutomationStatus? projectAutomationStatus(List<AssistantTurnDto> turns) {
  final roots = turns.where((turn) => turn.parentTurnId.isEmpty).toList()
    ..sort((a, b) => b.sequence.compareTo(a.sequence));
  if (roots.isEmpty) return null;
  final root = roots.first;
  bool terminal(AssistantTurnDto turn) =>
      const ['completed', 'failed', 'interrupted'].contains(turn.status);
  if (terminal(root)) return null;
  final byId = {for (final turn in turns) turn.id: turn};
  bool belongsToRoot(AssistantTurnDto turn) {
    final visited = <String>{};
    AssistantTurnDto? current = turn;
    while (current != null && !visited.contains(current.id)) {
      if (terminal(current)) return false;
      if (current.id == root.id) return true;
      visited.add(current.id);
      current = byId[current.parentTurnId];
    }
    return false;
  }

  final calls =
      <({AssistantTurnDto turn, AssistantTurnItemDto item, String label})>[];
  for (final turn in turns.where(belongsToRoot)) {
    for (final item in turn.items) {
      if (item.type != 'tool_call' ||
          !const [
            'queued',
            'running',
            'waiting_approval',
          ].contains(item.status)) {
        continue;
      }
      final label = automationAction(item.toolName);
      if (label != null) calls.add((turn: turn, item: item, label: label));
    }
  }
  calls.sort((a, b) {
    final order = b.turn.sequence.compareTo(a.turn.sequence);
    return order == 0 ? b.item.sequence.compareTo(a.item.sequence) : order;
  });
  if (calls.isEmpty) return null;
  final latest = calls.first;
  final approval =
      root.status == 'waiting_approval' ||
      latest.turn.status == 'waiting_approval' ||
      latest.item.status == 'waiting_approval';
  final waiting = approval || latest.item.status == 'queued';
  final cancelling =
      root.status == 'cancelling' || latest.turn.status == 'cancelling';
  final phase = cancelling
      ? 'cancelling'
      : waiting
      ? 'waiting'
      : latest.label == '正在观察画面'
      ? 'observing'
      : 'acting';
  return AutomationStatus(
    phase: phase,
    label: cancelling
        ? '正在停止自动化'
        : approval
        ? '等待授权'
        : waiting
        ? '等待自动化执行'
        : latest.label,
    detail: cancelling
        ? '操作结束后撤下指示'
        : approval
        ? '请查看对话中的授权提示'
        : waiting
        ? '操作已排队'
        : 'AI 正在执行界面操作',
    count: calls.length,
  );
}
