import 'package:flutter/material.dart';

class QuotedMessageText extends StatefulWidget {
  const QuotedMessageText({super.key, required this.text, this.style});

  final String text;
  final TextStyle? style;

  @override
  State<QuotedMessageText> createState() => _QuotedMessageTextState();
}

class _QuotedMessageTextState extends State<QuotedMessageText> {
  bool expanded = false;

  @override
  void didUpdateWidget(covariant QuotedMessageText oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.text != widget.text) expanded = false;
  }

  @override
  Widget build(BuildContext context) {
    final text = widget.text.isEmpty ? '原消息内容不可用' : widget.text;
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (expanded)
          ConstrainedBox(
            constraints: const BoxConstraints(maxHeight: 72),
            child: SingleChildScrollView(child: Text(text, style: widget.style)),
          )
        else
          Text(text, maxLines: 2, overflow: TextOverflow.ellipsis, style: widget.style),
        if (text.length > 30 || text.contains('\n'))
          InkWell(
            onTap: () => setState(() => expanded = !expanded),
            child: Padding(
              padding: const EdgeInsets.only(top: 2),
              child: Text(
                expanded ? '收起' : '展开',
                style: widget.style?.copyWith(
                  color: Theme.of(context).colorScheme.primary,
                ),
              ),
            ),
          ),
      ],
    );
  }
}
