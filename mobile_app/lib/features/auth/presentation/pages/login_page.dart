import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/services/providers.dart';

class LoginPage extends ConsumerStatefulWidget {
  const LoginPage({super.key});

  @override
  ConsumerState<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends ConsumerState<LoginPage> {
  final _usernameController = TextEditingController();
  final _passwordController = TextEditingController();
  bool _obscurePassword = true;
  int _loginState = 0;
  String _errorText = '';

  @override
  void dispose() {
    _usernameController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  Future<void> _login() async {
    final username = _usernameController.text.trim();
    final password = _passwordController.text.trim();

    if (username.isEmpty || password.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: const Text('请输入账号和密码'),
          duration: const Duration(seconds: 2),
          backgroundColor: context.warning,
        ),
      );
      return;
    }

    setState(() {
      _loginState = 1;
      _errorText = '';
    });

    try {
      final auth = ref.read(authServiceProvider);
      await auth.login(username, password);

      if (!mounted) return;

      ref.invalidate(currentUserProvider);
      context.go(AppRoutes.chat);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loginState = 3;
        _errorText = e.toString().replaceFirst('Exception: ', '');
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_errorText.isNotEmpty ? _errorText : '登录失败，请重试'),
          duration: const Duration(seconds: 2),
          backgroundColor: context.error,
        ),
      );
      // Keep the real backend error visible until the user edits or retries.

    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      body: SafeArea(
        child: SingleChildScrollView(
          padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              SizedBox(height: AppSpacing.xxxl + 20),
              Center(
                child: Container(
                  width: 88,
                  height: 88,
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    shape: BoxShape.circle,
                  ),
                  child: Icon(Icons.auto_awesome, size: 44, color: context.accentPrimary),
                ),
              ),
              SizedBox(height: AppSpacing.xl),
              Center(
                child: Text('Amitia', style: AppTypography.pageLargeTitle(context)),
              ),
              SizedBox(height: AppSpacing.xs),
              Center(
                child: Text(
                  '欢迎回来，请登录你的账号',
                  style: AppTypography.body(context).copyWith(color: context.textSecondary),
                ),
              ),
              SizedBox(height: AppSpacing.sectionGap + 8),
              Text('账号', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.sm),
              Container(
                decoration: BoxDecoration(
                  color: context.surfacePrimary,
                  borderRadius: AppRadius.brMedium,
                  border: Border.all(color: context.borderPrimary, width: 0.5),
                ),
                child: TextField(
                  controller: _usernameController,
                  style: AppTypography.body(context),
                  decoration: InputDecoration(
                    hintText: '请输入账号',
                    hintStyle: TextStyle(color: context.textTertiary),
                    prefixIcon: Icon(Icons.person_outline, size: 22, color: context.textTertiary),
                    isDense: true,
                    contentPadding: const EdgeInsets.symmetric(vertical: 14),
                    border: InputBorder.none,
                  ),
                ),
              ),
              SizedBox(height: AppSpacing.lg),
              Text('密码', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.sm),
              Container(
                decoration: BoxDecoration(
                  color: context.surfacePrimary,
                  borderRadius: AppRadius.brMedium,
                  border: Border.all(color: context.borderPrimary, width: 0.5),
                ),
                child: TextField(
                  controller: _passwordController,
                  obscureText: _obscurePassword,
                  style: AppTypography.body(context),
                  onSubmitted: (_) => _loginState == 1 ? null : _login(),
                  decoration: InputDecoration(
                    hintText: '请输入密码',
                    hintStyle: TextStyle(color: context.textTertiary),
                    prefixIcon: Icon(Icons.lock_outline, size: 22, color: context.textTertiary),
                    suffixIcon: GestureDetector(
                      onTap: () => setState(() => _obscurePassword = !_obscurePassword),
                      child: Icon(
                        _obscurePassword ? Icons.visibility_off_outlined : Icons.visibility_outlined,
                        size: 22,
                        color: context.textTertiary,
                      ),
                    ),
                    isDense: true,
                    contentPadding: const EdgeInsets.symmetric(vertical: 14),
                    border: InputBorder.none,
                  ),
                ),
              ),
              SizedBox(height: AppSpacing.sm),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  const SizedBox.shrink(),
                  GestureDetector(
                    onTap: () {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(
                          content: const Text('请联系管理员重置密码'),
                          duration: const Duration(seconds: 1),
                        ),
                      );
                    },
                    child: Text(
                      '忘记密码？',
                      style: AppTypography.caption(context).copyWith(color: context.accentPrimary),
                    ),
                  ),
                ],
              ),
              SizedBox(height: AppSpacing.xl),
              if (_loginState == 1)
                Center(
                  child: Padding(
                    padding: EdgeInsets.all(AppSpacing.md),
                    child: Column(
                      children: [
                        CircularProgressIndicator(strokeWidth: 2.5, color: context.accentPrimary),
                        SizedBox(height: AppSpacing.md),
                        Text('正在登录...', style: AppTypography.caption(context)),
                      ],
                    ),
                  ),
                )
              else
                AmitiaButton(
                  label: '登录',
                  icon: Icons.login,
                  isFullWidth: true,
                  onPressed: _login,
                ),
              SizedBox(height: AppSpacing.lg),
              if (_loginState == 3 && _errorText.isNotEmpty)
                Container(
                  padding: EdgeInsets.all(AppSpacing.md),
                  decoration: BoxDecoration(
                    color: context.error.withValues(alpha: 0.08),
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Row(
                    children: [
                      Icon(Icons.error_outline, size: 18, color: context.error),
                      SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: Text(
                          _errorText,
                          style: AppTypography.caption(context).copyWith(color: context.error),
                        ),
                      ),
                    ],
                  ),
                ),
              SizedBox(height: AppSpacing.xl),
              Center(
                child: Wrap(
                  alignment: WrapAlignment.center,
                  spacing: 14,
                  children: [
                    GestureDetector(
                      onTap: () => context.push(AppRoutes.settingsUserAgreement),
                      child: Text('用户协议', style: AppTypography.caption(context).copyWith(color: context.accentPrimary)),
                    ),
                    GestureDetector(
                      onTap: () => context.push(AppRoutes.settingsPrivacyPolicy),
                      child: Text('隐私政策', style: AppTypography.caption(context).copyWith(color: context.accentPrimary)),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
