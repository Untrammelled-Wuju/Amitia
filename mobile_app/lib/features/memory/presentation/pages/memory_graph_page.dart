import 'package:flutter/material.dart';
import 'owned_memory_page.dart';

class MemoryGraphPage extends StatelessWidget {
  const MemoryGraphPage({super.key});

  @override
  Widget build(BuildContext context) =>
      const OwnedMemoryPage(initialKind: 'graph');
}
