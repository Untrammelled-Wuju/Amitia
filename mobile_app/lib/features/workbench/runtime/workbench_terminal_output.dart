import 'dart:convert';

class WorkbenchTerminalOutput {
  static const maxCharacters = 220000;
  String _text = '';
  StringBuffer _pending = StringBuffer();
  List<int> _pendingUtf8 = [];
  String _escape = '';
  bool _isEscape = false;
  int _cursor = 0;

  String get text => _text;
  int get cursor => _cursor;

  void reset() {
    _text = '';
    _pending = StringBuffer();
    _pendingUtf8 = [];
    _escape = '';
    _isEscape = false;
    _cursor = 0;
  }

  void append(int nextCursor, List<int> bytes, {bool truncated = false, int? fromCursor}) {
    if (fromCursor != null && (fromCursor < 0 || fromCursor > nextCursor ||
        nextCursor - fromCursor != bytes.length)) {
      throw const FormatException('终端输出游标不一致');
    }
    if (nextCursor < _cursor) return;
    var incoming = bytes;
    if (fromCursor != null) {
      if (fromCursor > _cursor && !truncated) {
        throw const FormatException('终端输出有缺口，需要重新读取');
      }
      final overlap = _cursor > fromCursor ? _cursor - fromCursor : 0;
      incoming = bytes.sublist(overlap);
      truncated = truncated && fromCursor > _cursor;
    }
    if (nextCursor == _cursor && !truncated) return;
    if (truncated) {
      _pendingUtf8 = [];
      _escape = '';
      _isEscape = false;
      _text += '\n[较早的终端输出已被截断]\n';
    }
    final combined = [..._pendingUtf8, ...incoming];
    var fullLength = combined.length;
    if (fullLength > 0) {
      var continuation = 0;
      while (continuation < 3 && continuation < fullLength &&
          (combined[fullLength - continuation - 1] & 0xc0) == 0x80) {
        continuation++;
      }
      final marker = fullLength - continuation - 1;
      if (marker >= 0) {
        final leading = combined[marker];
        final expected = leading >= 0xf0 && leading <= 0xf4 ? 4 :
            leading >= 0xe0 && leading <= 0xef ? 3 :
            leading >= 0xc2 && leading <= 0xdf ? 2 : 1;
        if (expected > continuation + 1) fullLength = marker;
      } else if (continuation > 0) {
        fullLength = 0;
      }
    }
    _pendingUtf8 = combined.sublist(fullLength);
    final decoded = utf8.decode(combined.sublist(0, fullLength), allowMalformed: true);
    for (final code in decoded.runes) {
      _accept(String.fromCharCode(code));
    }
    _flushPending();
    if (_text.length > maxCharacters) _text = _text.substring(_text.length - maxCharacters);
    _cursor = nextCursor;
  }

  void _flushPending() {
    if (_pending.isEmpty) return;
    _text += _pending.toString();
    _pending = StringBuffer();
  }

  void _accept(String char) {
    if (_isEscape) {
      _escape += char;
      if (_escape.length > 128) {
        _isEscape = false;
        _escape = '';
      } else if (_escape.startsWith('[')) {
        final code = char.codeUnitAt(0);
        if (_escape.length > 1 && code >= 0x40 && code <= 0x7e) {
          if (char == 'K') {
            _flushPending();
            final position = _text.lastIndexOf('\n');
            _text = position < 0 ? '' : _text.substring(0, position + 1);
          } else if (char == 'J' && (_escape == '[2J' || _escape == '[3J')) {
            _flushPending();
            _text = '';
          }
          _isEscape = false;
          _escape = '';
        }
      } else if (_escape.startsWith(']')) {
        if (char == '\x07' || _escape.endsWith('\x1b\\')) {
          _isEscape = false;
          _escape = '';
        }
      } else {
        _isEscape = false;
        _escape = '';
      }
      return;
    }
    if (char == '\x1b') {
      _flushPending();
      _isEscape = true;
      _escape = '';
    } else if (char == '\r') {
      _flushPending();
      final last = _text.lastIndexOf('\n');
      _text = last < 0 ? '' : _text.substring(0, last + 1);
    } else if (char == '\b') {
      _flushPending();
      if (_text.isNotEmpty && !_text.endsWith('\n')) _text = _text.substring(0, _text.length - 1);
    } else if (char == '\n' || char == '\t' || char.codeUnitAt(0) >= 0x20) {
      _pending.write(char);
    }
  }
}
