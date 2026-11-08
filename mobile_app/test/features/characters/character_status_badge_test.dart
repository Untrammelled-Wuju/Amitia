import 'package:amitia_app/app/theme/app_theme.dart';
import 'package:amitia_app/core/widgets/amitia_character_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('多个启用角色只标记一个当前使用角色', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: AppTheme.lightTheme(),
        home: Scaffold(
          body: Column(
            children: [
              AmitiaCharacterCard(
                characterId: 'role-current',
                name: '当前角色',
                status: 'enabled',
                identity: '',
                avatarInitial: '当',
                avatarColor: '#5F6872',
                mood: '',
                lastActive: '当前使用',
                isCurrent: true,
              ),
              AmitiaCharacterCard(
                characterId: 'role-other',
                name: '其他角色',
                status: 'enabled',
                identity: '',
                avatarInitial: '其',
                avatarColor: '#5F6872',
                mood: '',
                lastActive: '未使用',
              ),
            ],
          ),
        ),
      ),
    );

    expect(find.text('使用中'), findsOneWidget);
    expect(find.text('已启用'), findsOneWidget);
    expect(find.text('enabled'), findsNothing);
    expect(
      find.byKey(const ValueKey('character-online/role-current')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('character-online/role-other')),
      findsNothing,
    );
  });
}
