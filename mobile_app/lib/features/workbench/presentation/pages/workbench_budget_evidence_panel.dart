import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/workbench_task_service.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import 'workbench_budget_evidence.dart';

class WorkbenchBudgetEvidencePanel extends ConsumerStatefulWidget {
  const WorkbenchBudgetEvidencePanel({super.key, required this.conversationId, required this.characterId, required this.coordinationId, required this.parentInteractionId, required this.onAuthorizationInvalidated});
  final String conversationId;
  final String characterId;
  final String coordinationId;
  final String parentInteractionId;
  final VoidCallback onAuthorizationInvalidated;
  @override
  ConsumerState<WorkbenchBudgetEvidencePanel> createState() => _WorkbenchBudgetEvidencePanelState();
}

class _WorkbenchBudgetEvidencePanelState extends ConsumerState<WorkbenchBudgetEvidencePanel> {
  String kind = 'model';
  String? error;
  bool busy = false;
  WorkbenchBudgetEvidencePage? page;
  int generation = 0;
  final evidence = TextEditingController();
  String? editingId;

  @override
  void didUpdateWidget(covariant WorkbenchBudgetEvidencePanel old) {
    super.didUpdateWidget(old);
    if (old.conversationId != widget.conversationId || old.characterId != widget.characterId || old.coordinationId != widget.coordinationId || old.parentInteractionId != widget.parentInteractionId) clear();
  }

  void clear() {
    generation++;
    page = null;
    editingId = null;
    evidence.clear();
    busy = false;
    error = null;
  }

  @override
  void dispose() {
    generation++;
    evidence.dispose();
    super.dispose();
  }

  void failed(Object cause) {
    setState(() { busy = false; error = cause.toString(); page = null; });
    if ((cause is WorkbenchTaskReadFailure && cause.invalidatesSnapshot) || cause is FormatException) {
      setState(clear);
      widget.onAuthorizationInvalidated();
    }
  }

  Future<void> load({Map<String, dynamic> cursor = const {}}) async {
    if (busy) return;
    final request = ++generation;
    setState(() { busy = true; error = null; editingId = null; evidence.clear(); });
    try {
      final response = await ref.read(workbenchTaskServiceProvider).readBudget(widget.conversationId, widget.characterId, coordinationId: widget.coordinationId, parentInteractionId: widget.parentInteractionId, kind: kind, cursor: cursor).timeout(const Duration(seconds: 25));
      if (!mounted || request != generation) return;
      final parsed = WorkbenchBudgetEvidencePage.parse(response, widget.coordinationId, widget.parentInteractionId, kind);
      setState(() { page = parsed; busy = false; });
    } catch (cause) {
      if (mounted && request == generation) failed(cause);
    }
  }

  Future<void> save() async {
    final id = editingId;
    final reference = evidence.text;
    if (busy || id == null || page?.canReview != true) return;
    if (reference.trim() != reference || utf8.encode(reference).length < 8 || utf8.encode(reference).length > 512 || reference.contains(RegExp(r'[\x00\r\n]'))) {
      setState(() { error = '证据引用须为 8–512 字节的单行文本，不能有首尾空白'; });
      return;
    }
    final request = ++generation;
    setState(() { busy = true; error = null; });
    try {
      final result = await ref.read(workbenchTaskServiceProvider).registerBudgetEvidence(widget.conversationId, widget.characterId, coordinationId: widget.coordinationId, parentInteractionId: widget.parentInteractionId, kind: kind, recordId: id, evidenceRef: reference).timeout(const Duration(seconds: 25));
      if (!mounted || request != generation) return;
      if (result['recorded'] != true || result['effect'] != 'evidence-only' || result['budgetReleased'] != false || result['executionConfirmed'] != false || result['coordinationId'] != widget.coordinationId || result['parentInteractionId'] != widget.parentInteractionId || result['kind'] != kind || result['recordId'] != id || result['evidenceRef'] != reference) throw const FormatException('预算证据登记回执不匹配');
      setState(() { busy = false; editingId = null; evidence.clear(); });
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('证据已登记；预算预留和执行状态保持原值')));
      await load();
    } catch (cause) {
      if (!mounted || request != generation) return;
      if (cause is FormatException || (cause is WorkbenchTaskReadFailure && cause.invalidatesSnapshot)) {
        failed(cause);
      } else {
        setState(() { busy = false; error = '登记结果尚未确认，可使用同一引用重试或刷新核对：$cause'; });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(rawBackendServiceApiProvider, (previous, next) {
      if (!identical(previous, next)) setState(clear);
    });
    return Padding(padding: const EdgeInsets.all(12), child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      const Text('预算核对：登记仅补充证据，不释放额度，也不证明工具动作已完成。'),
      Wrap(spacing: 8, children: [
        for (final option in const ['model', 'tool']) TextButton(onPressed: busy ? null : () { setState(() { clear(); kind = option; }); load(); }, child: Text(option == 'model' ? '模型预算证据' : '工具预算证据')),
        if (page != null) TextButton(onPressed: busy ? null : () => load(), child: const Text('刷新')),
      ]),
      if (busy) const LinearProgressIndicator(),
      if (error != null) Text(error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
      if (page != null && page!.records.isEmpty) const Text('当前页没有待核对记录'),
      for (final row in page?.records ?? <Map<String, dynamic>>[]) ListTile(
        dense: true,
        title: Text('${row['status']} · ${row['detail']}'),
        subtitle: Text('${row['startedAt']}\n${row['id']}${row['reviewEvidenceRef'] == '' ? '' : '\n证据 ${row['reviewEvidenceRef']}\n登记者 ${row['reviewActorId']} · ${row['reviewRecordedAt']}'}'),
        trailing: page!.canReview && row['reviewEvidenceRef'] == '' ? TextButton(onPressed: busy ? null : () => setState(() { editingId = row['id'] as String; evidence.clear(); }), child: const Text('登记证据')) : null,
      ),
      if (editingId != null && page?.canReview == true) ...[
        Text('登记记录 $editingId'),
        TextField(controller: evidence, enabled: !busy, maxLength: 512, decoration: const InputDecoration(labelText: '审计单号或可核验的证据引用')),
        TextButton(onPressed: busy ? null : save, child: const Text('确认登记')),
      ],
      if (page?.next != null) TextButton(onPressed: busy ? null : () => load(cursor: page!.next!), child: const Text('下一页')),
    ]));
  }
}
