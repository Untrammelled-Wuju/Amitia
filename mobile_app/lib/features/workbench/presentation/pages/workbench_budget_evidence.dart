class WorkbenchBudgetEvidencePage {
  const WorkbenchBudgetEvidencePage(this.records, this.canReview, this.next);
  final List<Map<String, dynamic>> records;
  final bool canReview;
  final Map<String, dynamic>? next;

  factory WorkbenchBudgetEvidencePage.parse(Map<String, dynamic> data, String coordinationId, String parentInteractionId, String kind) {
    if (data['protocolVersion'] != 1 || data['source'] != 'durable-budget-ledger' || data['coordinationId'] != coordinationId || data['parentInteractionId'] != parentInteractionId || data['kind'] != kind || data['records'] is! List || data['nextPagePossible'] is! bool || data['canReview'] is! bool) {
      throw const FormatException('预算证据不属于当前父任务');
    }
    final raw = data['records'] as List;
    if (raw.length > 50) throw const FormatException('预算证据数量异常');
    String text(Object? value, int maximum, {bool empty = false}) {
      if (value is! String || value.length > maximum || (!empty && value.isEmpty)) throw const FormatException('预算证据字段无效');
      return value;
    }
    final rows = <Map<String, dynamic>>[];
    for (final row in raw) {
      if (row is! Map) throw const FormatException('预算证据记录无效');
      final evidence = text(row['reviewEvidenceRef'] ?? '', 512, empty: true);
      final actor = text(row['reviewActorId'] ?? '', 160, empty: true);
      final recordedAt = text(row['reviewRecordedAt'] ?? '', 80, empty: true);
      if (evidence.isNotEmpty && (actor.isEmpty || DateTime.tryParse(recordedAt) == null)) throw const FormatException('预算登记回执无效');
      if (kind == 'model') {
        for (final field in const ['reservedTokens', 'reservedCostMicroUSD']) {
          if (row[field] is! int || (row[field] as int) < 0) throw const FormatException('预算预留金额无效');
        }
        if (!const {'running', 'interrupted', 'failed', 'cancelled', 'succeeded'}.contains(row['status']) || row['usageReported'] is! bool || (row['budgetOverrun'] != null && row['budgetOverrun'] is! bool) || (row['budgetOverrun'] == true && row['usageReported'] == true)) throw const FormatException('模型结算证据无效');
      }
      rows.add({
        'id': text(row[kind == 'model' ? 'id' : 'EffectID'], 255),
        'startedAt': text(row[kind == 'model' ? 'startedAt' : 'CreatedAt'], 80),
        'status': kind == 'model' ? row['status'] : 'unconfirmed',
        'detail': kind == 'model' ? '预留 ${row['reservedTokens']} Token · ${row['reservedCostMicroUSD']} 微美元' : text(row['ToolName'], 256, empty: true),
        'reviewEvidenceRef': evidence,
        'reviewActorId': actor,
        'reviewRecordedAt': recordedAt,
      });
    }
    Map<String, dynamic>? next;
    if (data['nextPagePossible'] == true) {
      if (rows.length != 50) throw const FormatException('预算分页证据不完整');
      if (kind == 'model') {
        if (data['nextAfterSequence'] is! int || (data['nextAfterSequence'] as int) < 1) throw const FormatException('模型预算游标无效');
        next = {'afterSequence': data['nextAfterSequence']};
      } else {
        final date = text(data['nextAfterCreatedAt'], 80);
        if (DateTime.tryParse(date) == null) throw const FormatException('工具预算游标无效');
        next = {'afterCreatedAt': date, 'afterEffectId': text(data['nextAfterEffectId'], 255)};
      }
    }
    return WorkbenchBudgetEvidencePage(rows, data['canReview'] == true, next);
  }
}
