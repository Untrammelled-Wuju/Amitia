import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'auth_service.dart';
import 'character_service.dart';
import '../api/api_client.dart';
import '../models/character.dart';

final apiClientProvider = Provider<ApiClient>((ref) => ApiClient());

final authServiceProvider = Provider<AuthService>((ref) => AuthService());

final characterServiceProvider = Provider<CharacterService>((ref) => CharacterService());

final authStateProvider = FutureProvider<bool>((ref) async {
  final auth = ref.read(authServiceProvider);
  return auth.isLoggedIn;
});

final currentUserProvider = FutureProvider.autoDispose<UserInfo?>((ref) async {
  final auth = ref.read(authServiceProvider);
  return auth.currentUser;
});

final characterListProvider = FutureProvider.autoDispose<List<CharacterDto>>((ref) async {
  final svc = ref.read(characterServiceProvider);
  return svc.list();
});
