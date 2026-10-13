import 'package:flutter/material.dart';
import '../../../memory/presentation/pages/owned_memory_page.dart';

class CharacterTimelinePage extends StatelessWidget {
  const CharacterTimelinePage({super.key, required this.characterId});
  final String characterId;

  @override
  Widget build(BuildContext context) =>
      OwnedMemoryPage(characterId: characterId, timeline: true);
}
