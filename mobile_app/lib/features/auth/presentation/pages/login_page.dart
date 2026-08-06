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
import '../../../../core/services/auth_service.dart';
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
          content: const Text('请输入用户名和密码'),
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

      setState(() => _loginState = 2);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: const Text('登录成功，正在进入 Amitia...'),
          duration: const Duration(seconds: 1),
          backgroundColor: context.success,
        ),
      );
      await Future.delayed(const Duration(milliseconds: 800));
      if (!mounted) return;
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
      await Future.delayed(const Duration(milliseconds: 1500));
      if (!mounted) return;
      setState(() => _loginState = 0);
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const SizedBox(height: AppSpacing.xxxl + 20),
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
              const SizedBox(height: AppSpacing.xl),
              Center(
                child: Text('Amitia', style: AppTypography.pageLargeTitle(context)),
              ),
              const SizedBox(height: AppSpacing.xs),
              Center(
                child: Text(
                  '欢迎回来，请登录你的账号',
                  style: AppTypography.body(context).copyWith(color: context.textSecondary),
                ),
              ),
              const SizedBox(height: AppSpacing.sectionGap + 8),
              Text('用户名', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.sm),
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
                    hintText: '请输入用户名',
                    hintStyle: TextStyle(color: context.textTertiary),
                    prefixIcon: Icon(Icons.person_outline, size: 22, color: context.textTertiary),
                    isDense: true,
                    contentPadding: const EdgeInsets.symmetric(vertical: 14),
                    border: InputBorder.none,
                  ),
                ),
              ),
              const SizedBox(height: AppSpacing.lg),
              Text('密码', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.sm),
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
                  onSubmitted: (_) => _loginState == 0 ? _login() : null,
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
              const SizedBox(height: AppSpacing.sm),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Row(
                    children: [
                      GestureDetector(
                        onTap: () {
                          ScaffoldMessenger.of(context).showSnackBar(
                            SnackBar(
                              content: const Text('记住密码功能暂未实现'),
                              duration: const Duration(seconds: 1),
                            ),
                          );
                        },
                        child: Container(
                          width: 20,
                          height: 20,
                          decoration: BoxDecoration(
                            color: context.accentSoft,
                            borderRadius: AppRadius.brTag,
                            border: Border.all(color: context.accentPrimary, width: 1.5),
                          ),
                          child: Icon(Icons.check, size: 14, color: context.accentPrimary),
                        ),
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      Text('记住密码', style: AppTypography.caption(context)),
                    ],
                  ),
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
              const SizedBox(height: AppSpacing.xl),
              if (_loginState == 1)
                Center(
                  child: Padding(
                    padding: const EdgeInsets.all(AppSpacing.md),
                    child: Column(
                      children: [
                        CircularProgressIndicator(strokeWidth: 2.5, color: context.accentPrimary),
                        const SizedBox(height: AppSpacing.md),
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
              const SizedBox(height: AppSpacing.lg),
              if (_loginState == 3 && _errorText.isNotEmpty)
                Container(
                  padding: const EdgeInsets.all(AppSpacing.md),
                  decoration: BoxDecoration(
                    color: context.error.withValues(alpha: 0.08),
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Row(
                    children: [
                      Icon(Icons.error_outline, size: 18, color: context.error),
                      const SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: Text(
                          _errorText,
                          style: AppTypography.caption(context).copyWith(color: context.error),
                        ),
                      ),
                    ],
                  ),
                ),
              const SizedBox(height: AppSpacing.xl),
              Center(
                child: GestureDetector(
                  onTap: () => context.go(AppRoutes.onboarding),
                  child: RichText(
                    text: TextSpan(
                      style: AppTypography.caption(context),
                      children: [
                        const TextSpan(text: '首次使用？'),
                        TextSpan(
                          text: '前往引导设置',
                          style: TextStyle(color: context.accentPrimary, fontWeight: FontWeight.w500),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
