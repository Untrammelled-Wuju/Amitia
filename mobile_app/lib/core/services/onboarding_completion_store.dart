import 'package:shared_preferences/shared_preferences.dart';

const onboardingCompletedPreferenceKey = 'onboarding_completed';

Future<bool> readOnboardingCompleted() async {
  final preferences = await SharedPreferences.getInstance();
  return preferences.getBool(onboardingCompletedPreferenceKey) ?? false;
}

Future<void> markOnboardingCompleted() async {
  final preferences = await SharedPreferences.getInstance();
  await preferences.setBool(onboardingCompletedPreferenceKey, true);
}
