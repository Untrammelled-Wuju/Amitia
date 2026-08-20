import 'package:flutter/material.dart';

import 'ui_provider.dart';

/// Resolves semantic icons from the active ui.icons provider. Providers can
/// alias to host icons or supply Material glyph code points without native code.
abstract final class UIIconRegistry {
  static IconData resolve(
    UIProviderSnapshot? snapshot,
    String semanticKey,
    IconData fallback,
  ) {
    if (snapshot == null) return fallback;
    final provider = snapshot.resolve(UICapability.icons);
    if (provider == null || provider.builtin || !provider.enabled) return fallback;
    final aliases = provider.metadata['iconAliases'];
    if (aliases is Map) {
      final raw = aliases[semanticKey] ?? aliases['default'];
      if (raw != null) return iconFromName(raw.toString(), fallback);
    }
    final glyphs = provider.metadata['iconGlyphs'];
    if (glyphs is Map) {
      final raw = glyphs[semanticKey] ?? glyphs['default'];
      if (raw is num) return IconData(raw.toInt(), fontFamily: 'MaterialIcons');
      if (raw is Map) {
        final row = raw.cast<dynamic, dynamic>();
        final codePoint = row['codePoint'];
        if (codePoint is num) {
          return IconData(
            codePoint.toInt(),
            fontFamily: row['fontFamily']?.toString() ?? 'MaterialIcons',
            fontPackage: row['fontPackage']?.toString(),
            matchTextDirection: row['matchTextDirection'] == true,
          );
        }
      }
    }
    return fallback;
  }

  static IconData iconFromName(String raw, [IconData fallback = Icons.extension_outlined]) {
    return switch (raw.toLowerCase()) {
      'chat' || 'message' => Icons.chat_bubble_outline,
      'task' || 'sparkles' => Icons.auto_awesome,
      'people' || 'character' => Icons.people_outline,
      'user' => Icons.person_outline,
      'memory' => Icons.memory,
      'devices' => Icons.devices_outlined,
      'settings' => Icons.settings_outlined,
      'game' => Icons.sports_esports_outlined,
      'dashboard' => Icons.dashboard_outlined,
      'history' => Icons.history_edu_outlined,
      'download' => Icons.file_download_outlined,
      'brush' || 'workshop' => Icons.brush_outlined,
      'pet' => Icons.pets_outlined,
      'notification' || 'reminder' => Icons.notifications_active_outlined,
      'link' || 'channel' => Icons.sync_alt,
      'extension' => Icons.extension_outlined,
      'menu' => Icons.menu,
      'back' => Icons.arrow_back_ios_new,
      _ => fallback,
    };
  }
}
