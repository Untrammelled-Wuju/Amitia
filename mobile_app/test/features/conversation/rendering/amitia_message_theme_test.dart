import 'package:amitia_app/app/theme/app_colors.dart';
import 'package:amitia_app/features/conversation/rendering/amitia_message_theme.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('message colors follow the selected accent across theme changes', (tester) async {
    late AmitiaMessageTheme messageTheme;
    Future<void> render(ThemeData theme) async {
      await tester.pumpWidget(MaterialApp(
        theme: theme,
        home: Builder(builder: (context) {
          messageTheme = AmitiaMessageTheme.of(context);
          return const SizedBox();
        }),
      ));
      await tester.pumpAndSettle();
    }
    final lightColors = defaultLightColorTokens().copyWith(
      accentPrimary: const Color(0xFF52B788),
      accentSoft: const Color(0xFFEBF5EF),
    );
    await render(ThemeData(extensions: [lightColors]));
    expect(messageTheme.accent, lightColors.accentPrimary);
    expect(messageTheme.userBubble, lightColors.accentSoft);
    final darkColors = defaultDarkColorTokens().copyWith(
      accentPrimary: const Color(0xFFF0B65D),
      accentSoft: const Color(0xFF393025),
    );
    await render(ThemeData(brightness: Brightness.dark, extensions: [darkColors]));
    expect(messageTheme.accent, darkColors.accentPrimary);
    expect(messageTheme.accentSoft, darkColors.accentSoft);
    expect(messageTheme.userBubble, darkColors.accentSoft);
  });
}
