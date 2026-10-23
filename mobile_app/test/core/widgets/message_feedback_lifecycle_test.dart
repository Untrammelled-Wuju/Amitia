import 'dart:async';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/models/feedback.dart';
import 'package:amitia_app/core/services/feedback_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/features/conversation/rendering/amitia_message_view.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

class _Api extends Fake implements BackendServiceApi {}

class _Feedback extends FeedbackService {
  _Feedback() : super(_Api());
  final calls = <String>[];
  final pending = <Completer<FeedbackDto?>>[];
  @override
  Future<FeedbackDto?> getByMessage(
    String messageId, {
    String characterId = '',
    Map<String, dynamic>? expectedExecutionScope,
  }) {
    calls.add(characterId);
    final result = Completer<FeedbackDto?>();
    pending.add(result);
    return result.future;
  }
}

void main() {
  testWidgets('feedback waits for saved reply and refreshes on role change', (
    tester,
  ) async {
    final service = _Feedback();
    Future<void> show(MessageStatus status, int? revision, String role) async {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [feedbackServiceProvider.overrideWithValue(service)],
          child: MaterialApp(
            home: Scaffold(
              body: AmitiaMessageView(
                key: const ValueKey('reply'),
                characterId: role,
                message: ChatMessage(
                  id: 'reply',
                  role: MessageRole.assistant,
                  type: MessageType.text,
                  content: 'hello',
                  time: DateTime(2026),
                  status: status,
                  sourceRevision: revision,
                  sourceOwnerId: 'owner',
                  sourceScope: {'roleId': role},
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pump();
    }

    await show(MessageStatus.streaming, null, 'one');
    expect(service.calls, isEmpty);
    await show(MessageStatus.sent, null, 'one');
    expect(service.calls, isEmpty);
    await show(MessageStatus.sent, 1, 'one');
    expect(service.calls, ['one']);
    await show(MessageStatus.sent, 1, 'two');
    expect(service.calls, ['one', 'two']);
    service.pending[1].complete(null);
    service.pending[0].complete(
      FeedbackDto.fromJson({
        'id': 1,
        'messageId': 'reply',
        'feedbackType': 'good',
        'revision': 1,
      }),
    );
    await tester.pump();
    expect(find.byTooltip('撤销反馈'), findsNothing);
    await tester.pumpWidget(const SizedBox());
  });
}
