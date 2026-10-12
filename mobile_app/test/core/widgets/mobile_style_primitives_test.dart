import 'package:amitia_app/core/ui_runtime/mobile_style_runtime.dart';
import 'package:amitia_app/core/settings/chat_appearance_preferences.dart';
import 'package:amitia_app/core/widgets/amitia_message.dart';
import 'package:amitia_app/core/widgets/amitia_misc.dart';
import 'package:amitia_app/core/widgets/user_message_surface.dart';
import 'package:amitia_app/features/conversation/rendering/amitia_message_view.dart';
import 'package:amitia_app/features/conversation/rendering/amrp.dart';
import 'package:amitia_app/features/conversation/rendering/code/amitia_code_block.dart';
import 'package:amitia_app/features/conversation/rendering/rich_blocks/amitia_rich_blocks.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter/material.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter_test/flutter_test.dart';

Widget styleHost({
  required AmitiaStyleDocument document,
  required Widget child,
  String page = 'chat',
}) {
  return MaterialApp(
    home: AmitiaStyleScopeHost(
      scope: AmitiaStyleScopeData(platform: 'mobile', page: page),
      child: AmitiaStyleDocumentHost(
        document: document,
        child: Scaffold(body: child),
      ),
    ),
  );
}

void main() {
  testWidgets('AmitiaButton resolves registered target styles', (tester) async {
    final document = AmitiaStyleDocument(
      revision: 1,
      rules: <AmitiaStyleRule>[
        AmitiaStyleRule(
          id: 'button',
          scope: const AmitiaStyleScopeData(
            platform: 'mobile',
            page: 'settings',
          ),
          target: 'button.primary',
          properties: const <String, dynamic>{
            'backgroundColor': '#ff0000',
            'foregroundColor': '#ffffff',
          },
        ),
      ],
    );
    await tester.pumpWidget(
      MaterialApp(
        home: AmitiaStyleScopeHost(
          scope: const AmitiaStyleScopeData(
            platform: 'mobile',
            page: 'settings',
          ),
          child: AmitiaStyleDocumentHost(
            document: document,
            child: Scaffold(
              body: AmitiaButton(label: '保存', onPressed: () {}),
            ),
          ),
        ),
      ),
    );

    final button = tester.widget<TextButton>(find.byType(TextButton));
    expect(
      button.style?.backgroundColor?.resolve(<WidgetState>{}),
      const Color(0xFFFF0000),
    );
    expect(
      button.style?.foregroundColor?.resolve(<WidgetState>{}),
      const Color(0xFFFFFFFF),
    );
  });

  testWidgets('AmitiaStyleTarget exposes reusable resolved properties', (
    tester,
  ) async {
    final document = AmitiaStyleDocument(
      revision: 1,
      rules: <AmitiaStyleRule>[
        AmitiaStyleRule(
          id: 'target',
          scope: const AmitiaStyleScopeData(platform: 'mobile', page: 'chat'),
          target: 'plugin.card',
          properties: const <String, dynamic>{'backgroundColor': '#123456'},
        ),
      ],
    );
    await tester.pumpWidget(
      MaterialApp(
        home: AmitiaStyleScopeHost(
          scope: const AmitiaStyleScopeData(platform: 'mobile', page: 'chat'),
          child: AmitiaStyleDocumentHost(
            document: document,
            child: AmitiaStyleTarget(
              target: 'plugin.card',
              child: const SizedBox.shrink(),
              builder: (context, style) => Text(
                style.color('backgroundColor')!.toARGB32().toRadixString(16),
              ),
            ),
          ),
        ),
      ),
    );

    expect(find.text('ff123456'), findsOneWidget);
  });

  testWidgets('AmitiaTextField applies text_field target styles', (
    tester,
  ) async {
    final document = AmitiaStyleDocument(
      revision: 1,
      rules: <AmitiaStyleRule>[
        AmitiaStyleRule(
          id: 'text-field',
          scope: const AmitiaStyleScopeData(
            platform: 'mobile',
            page: 'settings',
          ),
          target: 'text_field',
          properties: const <String, dynamic>{
            'backgroundColor': '#101112',
            'foregroundColor': '#f5f6f7',
            'placeholderColor': '#808388',
            'borderColor': '#ef4444',
            'borderWidth': 2,
            'borderRadius': 14,
            'fontSize': 16,
            'height': 52,
          },
        ),
      ],
    );
    await tester.pumpWidget(
      styleHost(
        document: document,
        page: 'settings',
        child: const AmitiaTextField(hintText: '名称'),
      ),
    );

    final field = tester.widget<TextField>(find.byType(TextField));
    expect(field.style?.color, const Color(0xFFF5F6F7));
    expect(field.style?.fontSize, 16);
    expect(field.decoration?.filled, isTrue);
    expect(field.decoration?.fillColor, const Color(0xFF101112));
    expect(field.decoration?.hintStyle?.color, const Color(0xFF808388));
    final enabledBorder = field.decoration?.enabledBorder as OutlineInputBorder;
    expect(enabledBorder.borderSide.color, const Color(0xFFEF4444));
    expect(enabledBorder.borderSide.width, 2);
    expect(enabledBorder.borderRadius, BorderRadius.circular(14));
  });

  testWidgets('AmitiaSwitchTile applies switch target styles', (tester) async {
    final document = AmitiaStyleDocument(
      revision: 1,
      rules: <AmitiaStyleRule>[
        AmitiaStyleRule(
          id: 'switch',
          scope: const AmitiaStyleScopeData(
            platform: 'mobile',
            page: 'settings',
          ),
          target: 'switch',
          properties: const <String, dynamic>{
            'backgroundColor': '#202225',
            'foregroundColor': '#22c55e',
            'borderColor': '#f97316',
            'borderWidth': 2,
            'borderRadius': 12,
          },
        ),
      ],
    );
    await tester.pumpWidget(
      styleHost(
        document: document,
        page: 'settings',
        child: AmitiaSwitchTile(title: '启用', value: true, onChanged: (_) {}),
      ),
    );

    final tile = tester.widget<SwitchListTile>(find.byType(SwitchListTile));
    final title = tester.widget<Text>(find.text('启用'));
    expect(tile.tileColor, const Color(0xFF202225));
    expect(tile.activeTrackColor, const Color(0xFF22C55E));
    expect(title.style?.color, const Color(0xFF22C55E));
    final shape = tile.shape as RoundedRectangleBorder;
    expect(shape.borderRadius, BorderRadius.circular(12));
    expect(shape.side.color, const Color(0xFFF97316));
    expect(shape.side.width, 2);
  });

  testWidgets('AmitiaSegmentedControl applies segmented target styles', (
    tester,
  ) async {
    final document = AmitiaStyleDocument(
      revision: 1,
      rules: <AmitiaStyleRule>[
        AmitiaStyleRule(
          id: 'segmented',
          scope: const AmitiaStyleScopeData(
            platform: 'mobile',
            page: 'settings',
          ),
          target: 'segmented',
          properties: const <String, dynamic>{
            'backgroundColor': '#18191c',
            'foregroundColor': '#f8fafc',
            'borderColor': '#3b82f6',
            'borderWidth': 2,
            'borderRadius': 10,
            'fontSize': 15,
          },
        ),
      ],
    );
    await tester.pumpWidget(
      styleHost(
        document: document,
        page: 'settings',
        child: AmitiaSegmentedControl(
          segments: const <String>['概览', '设置'],
          selectedIndex: 0,
          onChanged: (_) {},
        ),
      ),
    );

    final control = tester.widget<CupertinoSlidingSegmentedControl<int>>(
      find.byType(CupertinoSlidingSegmentedControl<int>),
    );
    final selected = tester.widget<Text>(find.text('概览'));
    expect(control.backgroundColor, const Color(0xFF18191C));
    expect(selected.style?.color, const Color(0xFFF8FAFC));
    expect(selected.style?.fontSize, 15);
  });

  testWidgets('AmitiaMessageView applies user message target styles', (
    tester,
  ) async {
    final document = AmitiaStyleDocument(
      revision: 1,
      rules: <AmitiaStyleRule>[
        AmitiaStyleRule(
          id: 'message-user',
          scope: const AmitiaStyleScopeData(platform: 'mobile', page: 'chat'),
          target: 'message.user',
          properties: const <String, dynamic>{
            'backgroundColor': '#123456',
            'foregroundColor': '#ffffff',
            'borderRadius': 10,
            'padding': <String, dynamic>{'x': 18, 'y': 12},
          },
        ),
      ],
    );
    await tester.pumpWidget(
      styleHost(
        document: document,
        child: AmitiaMessageView(
          message: ChatMessage(
            id: 'user-message',
            role: MessageRole.user,
            type: MessageType.text,
            content: '你好',
            time: DateTime(2026, 10, 10),
          ),
          aiAvatarEnabled: false,
          showHeader: false,
          userMessageMaterial: UserMessageMaterial.water,
        ),
      ),
    );

    final surface = tester.widget<UserMessageSurface>(
      find.byType(UserMessageSurface),
    );
    expect(surface.decoration.color, const Color(0xFF123456));
    expect(surface.material, UserMessageMaterial.solid);
    expect(surface.decoration.borderRadius, BorderRadius.circular(10));
    expect(
      surface.padding,
      const EdgeInsets.symmetric(horizontal: 9, vertical: 6),
    );
  });

  testWidgets('AmitiaMessageBubble applies user styles in flow mode', (
    tester,
  ) async {
    final document = AmitiaStyleDocument(
      revision: 1,
      rules: <AmitiaStyleRule>[
        AmitiaStyleRule(
          id: 'message-user-flow',
          scope: const AmitiaStyleScopeData(platform: 'mobile', page: 'chat'),
          target: 'message.user',
          properties: const <String, dynamic>{
            'backgroundColor': '#FEDCBA',
            'padding': <String, dynamic>{'x': 20, 'y': 14},
          },
        ),
      ],
    );
    await tester.pumpWidget(
      styleHost(
        document: document,
        child: AmitiaMessageBubble(
          messageStyle: ChatMessageStyle.flow,
          userMessageMaterial: UserMessageMaterial.water,
          showAvatar: false,
          showHeader: false,
          message: ChatMessage(
            id: 'user-flow-message',
            role: MessageRole.user,
            type: MessageType.text,
            content: '你好',
            time: DateTime(2026, 10, 10),
          ),
        ),
      ),
    );

    final surface = tester.widget<UserMessageSurface>(
      find.byType(UserMessageSurface),
    );
    expect(surface.material, UserMessageMaterial.solid);
    expect(surface.decoration.color, const Color(0xFFFEDCBA));
    expect(
      surface.padding,
      const EdgeInsets.symmetric(horizontal: 10, vertical: 7),
    );
  });

  testWidgets('AmitiaCodeBlock applies code_block target styles', (
    tester,
  ) async {
    final document = AmitiaStyleDocument(
      revision: 1,
      rules: <AmitiaStyleRule>[
        AmitiaStyleRule(
          id: 'code-block',
          scope: const AmitiaStyleScopeData(platform: 'mobile', page: 'chat'),
          target: 'code_block',
          properties: const <String, dynamic>{
            'backgroundColor': '#111827',
            'foregroundColor': '#f9fafb',
            'borderColor': '#ef4444',
            'borderWidth': 2,
            'borderRadius': 8,
            'fontSize': 13,
          },
        ),
      ],
    );
    await tester.pumpWidget(
      styleHost(
        document: document,
        child: const AmitiaCodeBlock(code: 'void main() {}', language: 'dart'),
      ),
    );
    await tester.pumpAndSettle();

    final block = tester.widget<Container>(
      find
          .descendant(
            of: find.byType(AmitiaCodeBlock),
            matching: find.byType(Container),
          )
          .first,
    );
    final decoration = block.decoration as BoxDecoration;
    final text = tester.widget<SelectableText>(find.byType(SelectableText));
    expect(decoration.color, const Color(0xFF111827));
    expect(decoration.borderRadius, BorderRadius.circular(8));
    expect(text.style?.color, const Color(0xFFF9FAFB));
    expect(text.style?.fontSize, 13);
  });

  testWidgets('AmitiaFileBlock applies attachment target styles', (
    tester,
  ) async {
    final document = AmitiaStyleDocument(
      revision: 1,
      rules: <AmitiaStyleRule>[
        AmitiaStyleRule(
          id: 'attachment',
          scope: const AmitiaStyleScopeData(platform: 'mobile', page: 'chat'),
          target: 'attachment',
          properties: const <String, dynamic>{
            'backgroundColor': '#1f2937',
            'foregroundColor': '#f3f4f6',
            'borderColor': '#0ea5e9',
            'borderWidth': 2,
            'borderRadius': 8,
          },
        ),
      ],
    );
    await tester.pumpWidget(
      styleHost(
        document: document,
        child: const AmitiaFileBlock(
          block: AmrpFileBlock(id: 'file', name: 'report.pdf'),
        ),
      ),
    );

    final card = tester.widget<Container>(
      find
          .descendant(
            of: find.byType(AmitiaFileBlock),
            matching: find.byType(Container),
          )
          .first,
    );
    final decoration = card.decoration as BoxDecoration;
    final title = tester.widget<Text>(find.text('report.pdf'));
    expect(decoration.color, const Color(0xFF1F2937));
    expect(decoration.borderRadius, BorderRadius.circular(8));
    expect(title.style?.color, const Color(0xFFF3F4F6));
  });
}
