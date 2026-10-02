import 'package:flutter/material.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_motion.dart';
import '../../runtime/automation_status.dart';

class AutomationStatusIndicator extends StatefulWidget {
  const AutomationStatusIndicator({super.key, required this.status});

  final AutomationStatus status;

  @override
  State<AutomationStatusIndicator> createState() =>
      _AutomationStatusIndicatorState();
}

class _AutomationStatusIndicatorState extends State<AutomationStatusIndicator>
    with SingleTickerProviderStateMixin {
  late final AnimationController _pulse = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1200),
  );

  void _syncAnimation() {
    final active =
        widget.status.phase == 'observing' || widget.status.phase == 'acting';
    final enabled =
        active &&
        !MediaQuery.disableAnimationsOf(context) &&
        AppMotion.standard != Duration.zero;
    if (enabled && !_pulse.isAnimating) _pulse.repeat(reverse: true);
    if (!enabled) {
      _pulse.stop();
      _pulse.value = 0;
    }
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _syncAnimation();
  }

  @override
  void didUpdateWidget(AutomationStatusIndicator oldWidget) {
    super.didUpdateWidget(oldWidget);
    _syncAnimation();
  }

  @override
  void dispose() {
    _pulse.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final status = widget.status;
    final accent = context.accentPrimary;
    return IgnorePointer(
      child: Stack(
        fit: StackFit.expand,
        children: [
          Padding(
            padding: const EdgeInsets.all(2),
            child: AnimatedBuilder(
              animation: _pulse,
              builder: (context, child) => DecoratedBox(
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(18),
                  border: Border.all(
                    color: accent.withValues(alpha: .6 + .4 * _pulse.value),
                    width: 2,
                  ),
                ),
              ),
            ),
          ),
          SafeArea(
            child: Align(
              alignment: Alignment.topCenter,
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 76, 16, 0),
                child: Semantics(
                  liveRegion: true,
                  label: '${status.label}，${status.detail}',
                  child: ExcludeSemantics(
                    child: Container(
                      constraints: const BoxConstraints(maxWidth: 340),
                      padding: const EdgeInsets.symmetric(
                        horizontal: 14,
                        vertical: 10,
                      ),
                      decoration: BoxDecoration(
                        color: context.surfacePrimary,
                        borderRadius: BorderRadius.circular(16),
                        border: Border.all(
                          color: accent.withValues(alpha: .24),
                        ),
                        boxShadow: [
                          BoxShadow(
                            color: context.textPrimary.withValues(alpha: .08),
                            blurRadius: 18,
                            offset: const Offset(0, 4),
                          ),
                        ],
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Container(
                            width: 8,
                            height: 8,
                            decoration: BoxDecoration(
                              color: accent,
                              shape: BoxShape.circle,
                            ),
                          ),
                          const SizedBox(width: 10),
                          Flexible(
                            child: Column(
                              mainAxisSize: MainAxisSize.min,
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  status.label,
                                  style: TextStyle(
                                    color: context.textPrimary,
                                    fontSize: 13,
                                    fontWeight: FontWeight.w600,
                                  ),
                                ),
                                const SizedBox(height: 3),
                                Text(
                                  '${status.detail}${status.count > 1 ? ' · ${status.count} 项操作' : ''}',
                                  style: TextStyle(
                                    color: context.textSecondary,
                                    fontSize: 11,
                                    height: 1.5,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
