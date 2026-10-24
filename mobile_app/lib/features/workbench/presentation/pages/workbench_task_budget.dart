class WorkbenchTaskBudget {
  static String notice(Map<String, dynamic> task, {required DateTime now, required bool stale}) {
    if (stale || task['recoveryState'] == 'needs-reconciliation') {
      return '预算状态不可确认；请取得新的授权持久快照，不能据此判定执行停止。';
    }
    final deadlineValue = task['deadlineAt'];
    if (deadlineValue == null) {
      return '未记录协调时间截止。新的多 Agent 委派可选 deadline_minutes（1–240 分钟）建立持久时间限制；Token、工具次数和费用限制须结合父任务配置及预算账本核对。';
    }
    if (deadlineValue is! String) return '预算状态不可确认；请刷新持久快照。';
    final deadline = DateTime.tryParse(deadlineValue);
    if (deadline == null) return '预算状态不可确认；请刷新持久快照。';
    if (const {'succeeded', 'failed', 'cancelled'}.contains(task['status'])) {
      return '任务已报告终态；截止时间不能证明任务在时限内完成，请以持久回执为准。';
    }
    if (!deadline.isAfter(now)) {
      return '协调截止时间已过；执行是否停止及外部副作用是否撤销仍须通过正式回执核查。';
    }
    return '协调时间预算尚未到期；不代表存在 Token、调用次数或费用总额硬限制。';
  }
}
