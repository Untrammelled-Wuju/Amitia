import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'debug_log_service.dart';

class DebugLogOverlay extends ConsumerStatefulWidget {
  const DebugLogOverlay({super.key});

  @override
  ConsumerState<DebugLogOverlay> createState() => _DebugLogOverlayState();
}

class _DebugLogOverlayState extends ConsumerState<DebugLogOverlay> {
  double _right = 16;
  double _top = 100;
  bool _expanded = true;
  String _filterSource = 'all';
  DebugLogLevel _minLevel = DebugLogLevel.debug;
  final ScrollController _scrollCtrl = ScrollController();
  bool _autoScroll = true;
  int? _expandedIndex;

  static const List<String> _sources = ['all', 'flutter', 'runtime', 'backend', 'sidecar', 'proot', 'unhandled', 'flutter.error'];

  @override
  void dispose() {
    _scrollCtrl.dispose();
    super.dispose();
  }

  List<DebugLogEntry> _filter(List<DebugLogEntry> entries) {
    return entries.where((e) {
      if (_filterSource != 'all' && e.source != _filterSource) return false;
      final levelPriority = {
        DebugLogLevel.debug: 0,
        DebugLogLevel.info: 1,
        DebugLogLevel.warn: 2,
        DebugLogLevel.error: 3,
      };
      return levelPriority[e.level]! >= levelPriority[_minLevel]!;
    }).toList();
  }

  Color _levelColor(DebugLogLevel level) {
    switch (level) {
      case DebugLogLevel.debug:
        return const Color(0xFF607D8B);
      case DebugLogLevel.info:
        return const Color(0xFF4CAF50);
      case DebugLogLevel.warn:
        return const Color(0xFFFF9800);
      case DebugLogLevel.error:
        return const Color(0xFFF44336);
    }
  }

  void _copyAllLogs(List<DebugLogEntry> entries) {
    final text = entries.map((e) => '[${e.timeStr}][${e.levelStr}][${e.source}] ${e.message}').join('\n');
    Clipboard.setData(ClipboardData(text: text));
  }

  @override
  Widget build(BuildContext context) {
    final entriesAsync = ref.watch(debugLogEntriesProvider);
    final entries = entriesAsync.valueOrNull ?? [];
    final filtered = _filter(entries);

    if (_autoScroll && _scrollCtrl.hasClients && _expanded) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (_scrollCtrl.hasClients) {
          _scrollCtrl.jumpTo(_scrollCtrl.position.maxScrollExtent);
        }
      });
    }

    final screenWidth = MediaQuery.of(context).size.width;
    final screenHeight = MediaQuery.of(context).size.height;
    final isMobile = screenWidth < 600;
    final overlayWidth = _expanded ? (isMobile ? screenWidth * 0.85 : 480.0) : 200.0;
    final overlayHeight = _expanded ? (isMobile ? screenHeight * 0.5 : 360.0) : 36.0;

    return Positioned(
      right: _expanded ? _right : _right,
      top: _top,
      child: GestureDetector(
        onPanUpdate: (d) {
          setState(() {
            _right -= d.delta.dx;
            _top += d.delta.dy;
            _right = _right.clamp(-300.0, screenWidth - 50);
            _top = _top.clamp(0.0, screenHeight - 50);
          });
        },
        child: Material(
          elevation: 8,
          borderRadius: BorderRadius.circular(8),
          color: const Color(0xE61E1E1E),
          child: AnimatedContainer(
            duration: const Duration(milliseconds: 200),
            width: overlayWidth,
            height: overlayHeight,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                _buildHeader(filtered.length, entries.length),
                if (_expanded) _buildFilterBar(),
                if (_expanded)
                  Expanded(
                    child: ListView.builder(
                      controller: _scrollCtrl,
                      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                      itemCount: filtered.length,
                      itemBuilder: (context, i) => _buildLogItem(filtered[i], i),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildHeader(int filtered, int total) {
    return InkWell(
      onTap: () => setState(() => _expanded = !_expanded),
      child: Container(
        height: 36,
        padding: const EdgeInsets.symmetric(horizontal: 8),
        child: Row(
          children: [
            Container(
              width: 8,
              height: 8,
              decoration: BoxDecoration(
                color: Colors.greenAccent,
                shape: BoxShape.circle,
              ),
            ),
            const SizedBox(width: 6),
            const Text(
              'DEBUG',
              style: TextStyle(
                color: Colors.white70,
                fontSize: 11,
                fontWeight: FontWeight.bold,
                letterSpacing: 1,
              ),
            ),
            const SizedBox(width: 8),
            Text(
              '$filtered/$total',
              style: const TextStyle(color: Color(0xFFAAAAAA), fontSize: 11),
            ),
            const Spacer(),
            if (_expanded)
              GestureDetector(
                onTap: () {
                  final service = ref.read(debugLogServiceProvider);
                  service.clear();
                },
                child: const Padding(
                  padding: EdgeInsets.symmetric(horizontal: 4),
                  child: Icon(Icons.delete_sweep_outlined, size: 14, color: Colors.white54),
                ),
              ),
            if (_expanded)
              GestureDetector(
                onTap: () {
                  final entries = ref.read(debugLogServiceProvider).entries;
                  _copyAllLogs(entries);
                },
                child: const Padding(
                  padding: EdgeInsets.symmetric(horizontal: 4),
                  child: Icon(Icons.copy_all_outlined, size: 14, color: Colors.white54),
                ),
              ),
            if (_expanded)
              GestureDetector(
                onTap: () => setState(() => _autoScroll = !_autoScroll),
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 4),
                  child: Icon(
                    _autoScroll ? Icons.vertical_align_bottom : Icons.pause_circle_outline,
                    size: 14,
                    color: _autoScroll ? Colors.greenAccent : Colors.white54,
                  ),
                ),
              ),
            Icon(
              _expanded ? Icons.unfold_less : Icons.unfold_more,
              size: 16,
              color: Colors.white54,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildFilterBar() {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: const BoxDecoration(
        border: Border(bottom: BorderSide(color: Color(0xFF333333))),
      ),
      child: Row(
        children: [
          Expanded(
            child: SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: Row(
                children: _sources.map((s) {
                  final selected = _filterSource == s;
                  return GestureDetector(
                    onTap: () => setState(() => _filterSource = s),
                    child: Container(
                      margin: const EdgeInsets.only(right: 4),
                      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                      decoration: BoxDecoration(
                        color: selected ? const Color(0xFF3D5AFE) : const Color(0xFF333333),
                        borderRadius: BorderRadius.circular(3),
                      ),
                      child: Text(
                        s == 'all' ? '全部' : s,
                        style: TextStyle(
                          color: selected ? Colors.white : const Color(0xFFAAAAAA),
                          fontSize: 10,
                        ),
                      ),
                    ),
                  );
                }).toList(),
              ),
            ),
          ),
          const SizedBox(width: 8),
          DropdownButton<DebugLogLevel>(
            value: _minLevel,
            isDense: true,
            style: const TextStyle(fontSize: 10, color: Colors.white),
            dropdownColor: const Color(0xFF2A2A2A),
            underline: const SizedBox(),
            items: const [
              DropdownMenuItem(value: DebugLogLevel.debug, child: Text('DBG', style: TextStyle(color: Color(0xFF607D8B), fontSize: 11))),
              DropdownMenuItem(value: DebugLogLevel.info, child: Text('INF', style: TextStyle(color: Color(0xFF4CAF50), fontSize: 11))),
              DropdownMenuItem(value: DebugLogLevel.warn, child: Text('WRN', style: TextStyle(color: Color(0xFFFF9800), fontSize: 11))),
              DropdownMenuItem(value: DebugLogLevel.error, child: Text('ERR', style: TextStyle(color: Color(0xFFF44336), fontSize: 11))),
            ],
            onChanged: (v) {
              if (v != null) setState(() => _minLevel = v);
            },
          ),
        ],
      ),
    );
  }

  Widget _buildLogItem(DebugLogEntry entry, int index) {
    final hasStack = entry.stackTrace != null && entry.stackTrace!.isNotEmpty;
    final isExpanded = _expandedIndex == index;

    return GestureDetector(
      onTap: hasStack
          ? () => setState(() => _expandedIndex = isExpanded ? null : index)
          : null,
      child: Padding(
        padding: const EdgeInsets.only(bottom: 2),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                SizedBox(
                  width: 56,
                  child: Text(
                    entry.timeStr,
                    style: const TextStyle(color: Color(0xFF777777), fontSize: 9, fontFamily: 'monospace'),
                  ),
                ),
                Container(
                  width: 28,
                  alignment: Alignment.center,
                  child: Text(
                    entry.levelStr,
                    style: TextStyle(
                      color: _levelColor(entry.level),
                      fontSize: 8,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
                Container(
                  width: 50,
                  child: Text(
                    entry.source,
                    style: const TextStyle(color: Color(0xFF9C27B0), fontSize: 9),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                const SizedBox(width: 4),
                if (hasStack)
                  Icon(
                    isExpanded ? Icons.expand_less : Icons.expand_more,
                    size: 12,
                    color: const Color(0xFF777777),
                  ),
                Expanded(
                  child: Text(
                    entry.message,
                    style: const TextStyle(color: Color(0xFFDDDDDD), fontSize: 9, fontFamily: 'monospace'),
                    maxLines: isExpanded ? 10 : 3,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
            if (isExpanded && hasStack)
              Container(
                margin: const EdgeInsets.only(top: 4, left: 8, right: 8),
                padding: const EdgeInsets.all(6),
                decoration: BoxDecoration(
                  color: const Color(0xFF0D0D0D),
                  borderRadius: BorderRadius.circular(4),
                ),
                child: SelectableText(
                  entry.stackTrace!,
                  style: const TextStyle(
                    color: Color(0xFFAAAAAA),
                    fontSize: 8,
                    fontFamily: 'monospace',
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
