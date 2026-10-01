import 'package:flutter/material.dart';

class RealtimeCallLayout extends StatelessWidget {
  const RealtimeCallLayout({
    super.key,
    required this.background,
    required this.details,
    required this.controls,
    this.overlays = const [],
  });

  final Widget background;
  final Widget details;
  final Widget controls;
  final List<Widget> overlays;

  @override
  Widget build(BuildContext context) {
    return Stack(
      fit: StackFit.expand,
      children: [
        background,
        ...overlays,
        SafeArea(
          child: Column(
            children: [
              Expanded(
                child: SingleChildScrollView(
                  padding: const EdgeInsets.fromLTRB(20, 48, 20, 24),
                  child: details,
                ),
              ),
              Padding(
                padding: const EdgeInsets.fromLTRB(16, 12, 16, 28),
                child: controls,
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class RealtimeCallErrorDetails extends StatelessWidget {
  const RealtimeCallErrorDetails({super.key, required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    return Theme(
      data: Theme.of(context).copyWith(dividerColor: Colors.transparent),
      child: ExpansionTile(
        title: const Text('查看完整错误', style: TextStyle(fontSize: 14)),
        textColor: Colors.white,
        collapsedTextColor: Colors.white,
        iconColor: Colors.white,
        collapsedIconColor: Colors.white,
        childrenPadding: const EdgeInsets.fromLTRB(12, 0, 12, 16),
        children: [
          SelectableText(
            message,
            style: const TextStyle(color: Color(0xFFBCBCC0), fontSize: 14),
          ),
        ],
      ),
    );
  }
}
