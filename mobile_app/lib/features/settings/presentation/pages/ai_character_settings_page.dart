import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/character.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../characters/presentation/pages/character_life_rules_page.dart';

/// Legacy compatibility entry for the old AI-character settings route.
///
/// The actual role/personality/lifestyle editor is [CharacterLifeRulesPage].
/// Keeping this class as a thin router prevents two independent state/save
/// implementations from drifting apart if an older route or extension still
/// references this page.
class AiCharacterSettingsPage extends ConsumerWidget {
  const AiCharacterSettingsPage({super.key});

  CharacterDto? _preferredCharacter(List<CharacterDto> characters) {
    for (final character in characters) {
      if (character.isDefault) return character;
    }
    for (final character in characters) {
      if (character.isActive == 1) return character;
    }
    return characters.isEmpty ? null : characters.first;
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final charactersAsync = ref.watch(characterListProvider);

    return charactersAsync.when(
      loading: () => AmitiaScaffold(
        appBar: const AmitiaAppBar(
          title: '角色性格设置',
          showBackButton: true,
          fallbackRoute: AppRoutes.settings,
        ),
        body: const Center(child: CircularProgressIndicator()),
      ),
      error: (error, _) => AmitiaScaffold(
        appBar: const AmitiaAppBar(
          title: '角色性格设置',
          showBackButton: true,
          fallbackRoute: AppRoutes.settings,
        ),
        body: Center(
          child: Padding(
            padding: const EdgeInsets.all(32),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.error_outline, size: 48, color: context.textSecondary),
                const SizedBox(height: 16),
                Text(
                  '加载失败: ${error.toString().replaceFirst('Exception: ', '')}',
                  style: AppTypography.body(context).copyWith(color: context.error),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 16),
                AmitiaButton(
                  label: '重试',
                  onPressed: () => ref.invalidate(characterListProvider),
                ),
              ],
            ),
          ),
        ),
      ),
      data: (characters) {
        final character = _preferredCharacter(characters);
        if (character == null) {
          return AmitiaScaffold(
            appBar: const AmitiaAppBar(
              title: '角色性格设置',
              showBackButton: true,
              fallbackRoute: AppRoutes.settings,
            ),
            body: Center(
              child: Padding(
                padding: const EdgeInsets.all(32),
                child: Text(
                  '暂无角色，请先创建角色后再配置性格与生活规则。',
                  style: AppTypography.body(context),
                  textAlign: TextAlign.center,
                ),
              ),
            ),
          );
        }
        return CharacterLifeRulesPage(
          characterId: character.id,
          pageTitle: '角色性格设置',
          fallbackRoute: AppRoutes.settings,
        );
      },
    );
  }
}
