import 'package:amitia_app/app/theme/appearance_colors.dart';
import 'package:amitia_app/app/theme/app_colors.dart';
import 'package:amitia_app/core/settings/appearance_preferences.dart';
import 'package:amitia_app/core/settings/chat_input_preferences.dart';
import 'package:amitia_app/core/settings/custom_palette.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test('mobile sending preference defaults off and survives reload', () async {
    SharedPreferences.setMockInitialValues({});
    final notifier = ChatInputPreferencesNotifier();
    await notifier.init();
    expect(notifier.state, false);
    await notifier.setSendOnEnter(true);
    final restored = ChatInputPreferencesNotifier();
    await restored.init();
    expect(restored.state, true);
    notifier.dispose(); restored.dispose();
  });
  test('custom colors persist without losing the previous preset', () async {
    SharedPreferences.setMockInitialValues({'appearance.accentColorIndex': 0});
    final notifier = AppearancePreferencesNotifier();
    await notifier.init();
    await notifier.setCustomPalette(const CustomPalette(enabled: true, primary: Colors.yellow, secondary: Colors.purple, text: Colors.red, textMode: ThemeTextMode.custom));
    final restored = AppearancePreferencesNotifier();
    await restored.init();
    expect(restored.state.customPalette.primary.toARGB32(), Colors.yellow.toARGB32());
    expect(restored.state.customPalette.secondary.toARGB32(), Colors.purple.toARGB32());
    final active = resolveAppearanceColors(defaultLightColorTokens(), Brightness.light, restored.state);
    expect(active.textPrimary.toARGB32(), Colors.red.toARGB32());
    expect(active.accentSecondary.toARGB32(), Colors.purple.toARGB32());
    await restored.setCustomPalette(restored.state.customPalette.copyWith(enabled: false));
    expect(resolveAppearanceColors(defaultLightColorTokens(), Brightness.light, restored.state).accentPrimary, const Color(0xFF8A5728));
    expect(restored.state.customPalette.primary.toARGB32(), Colors.yellow.toARGB32());
    notifier.dispose(); restored.dispose();
  });
  test('automatic text follows background and corrupt preferences are safe', () {
    expect(CustomPalette.decode('invalid').enabled, false);
    expect(CustomPalette.decode('{"primary": "invalid"}').primary, const Color(0xFF6C8FEA));
    expect(colorContrast(Colors.black, Colors.white), 21);
    expect(readableForeground(Colors.yellow), Colors.black);
    const appearance = AppearancePreferences(customPalette: CustomPalette(enabled: true));
    expect(resolveAppearanceColors(defaultLightColorTokens(), Brightness.light, appearance).textPrimary, Colors.black);
    expect(resolveAppearanceColors(defaultDarkColorTokens(), Brightness.dark, appearance).textPrimary, Colors.white);
  });
}
