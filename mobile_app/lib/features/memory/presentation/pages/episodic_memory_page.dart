import 'package:flutter/material.dart';
import 'owned_memory_page.dart';

class EpisodicMemoryPage extends StatelessWidget {
  const EpisodicMemoryPage({super.key});

  @override
  Widget build(BuildContext context) =>
      const OwnedMemoryPage(initialKind: 'episodic');
}
