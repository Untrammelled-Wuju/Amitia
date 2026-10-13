import 'package:flutter/material.dart';
import 'owned_memory_page.dart';

class UserProfilesPage extends StatelessWidget {
  const UserProfilesPage({super.key});

  @override
  Widget build(BuildContext context) =>
      const OwnedMemoryPage(initialKind: 'profile');
}
