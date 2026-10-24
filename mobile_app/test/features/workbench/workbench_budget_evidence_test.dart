import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/workbench/presentation/pages/workbench_budget_evidence.dart';

void main() {
  Map<String, dynamic> response({String kind = 'model'}) => {
    'protocolVersion': 1, 'source': 'durable-budget-ledger', 'coordinationId': 'coord', 'parentInteractionId': 'parent', 'kind': kind,
    'nextPagePossible': false, 'canReview': true,
    'records': kind == 'model' ? [{'id': 'record', 'status': 'interrupted', 'startedAt': '2026-10-24T00:00:00Z', 'reservedTokens': 128, 'reservedCostMicroUSD': 32, 'usageReported': false}]
        : [{'EffectID': 'effect', 'ToolName': 'shell_run', 'CreatedAt': '2026-10-24T00:00:00Z', 'reviewEvidenceRef': 'case:12345678', 'reviewActorId': 'owner', 'reviewRecordedAt': '2026-10-24T00:01:00Z'}],
  };
  test('uses real model and tool ledger field casing and owner capability', () {
    final model = WorkbenchBudgetEvidencePage.parse(response(), 'coord', 'parent', 'model');
    expect(model.canReview, true);
    expect(model.records.single['detail'], contains('128'));
    final tool = WorkbenchBudgetEvidencePage.parse(response(kind: 'tool'), 'coord', 'parent', 'tool');
    expect(tool.records.single['status'], 'unconfirmed');
    expect(tool.records.single['reviewEvidenceRef'], 'case:12345678');
    expect(WorkbenchBudgetEvidencePage.parse({...response(), 'canReview': false}, 'coord', 'parent', 'model').canReview, false);
  });
  test('rejects foreign parents, incomplete review receipts and invalid pagination', () {
    expect(() => WorkbenchBudgetEvidencePage.parse(response(), 'coord', 'other', 'model'), throwsFormatException);
    expect(() => WorkbenchBudgetEvidencePage.parse({...response(), 'canReview': 'true'}, 'coord', 'parent', 'model'), throwsFormatException);
    expect(() => WorkbenchBudgetEvidencePage.parse({...response(), 'nextPagePossible': true}, 'coord', 'parent', 'model'), throwsFormatException);
    final invalid = response(kind: 'tool');
    (invalid['records'] as List).single['reviewActorId'] = '';
    expect(() => WorkbenchBudgetEvidencePage.parse(invalid, 'coord', 'parent', 'tool'), throwsFormatException);
  });
}
