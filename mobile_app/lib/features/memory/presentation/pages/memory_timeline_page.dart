import 'package:flutter/material.dart';
import 'owned_memory_page.dart';

class MemoryTimelinePage extends StatelessWidget {
  const MemoryTimelinePage({super.key});

  @override
  Widget build(BuildContext context) => const OwnedMemoryPage(timeline: true);
}
