import 'package:amitia_app/core/widgets/conversation_visibility.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('expands sidebar conversations five at a time', () {
    expect(expandSidebarConversationCount(5, 12), 10);
    expect(expandSidebarConversationCount(10, 12), 12);
    expect(expandSidebarConversationCount(5, 7), 7);
  });

  test('normalizes sidebar conversation count after data changes', () {
    expect(normalizeSidebarConversationCount(12, 3), 5);
    expect(normalizeSidebarConversationCount(12, 8), 8);
    expect(normalizeSidebarConversationCount(2, 12), 5);
  });
}
