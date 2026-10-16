import 'package:flutter_test/flutter_test.dart';
import '../../../lib/features/workbench/runtime/workbench_terminal_output.dart';

void main() {
  test('retains UTF-8 across snapshot boundaries', () {
    final output = WorkbenchTerminalOutput();
    output.append(2, [0xe4, 0xb8]);
    expect(output.text, '');
    output.append(4, [0xad, 0x0a]);
    expect(output.text, '中\n');
  });

  test('ignores duplicate and older snapshots', () {
    final output = WorkbenchTerminalOutput();
    output.append(1, [65]);
    output.append(1, [66]);
    output.append(0, [67]);
    expect(output.text, 'A');
  });

  test('filters ANSI control sequences and clears line', () {
    final output = WorkbenchTerminalOutput();
    output.append(7, [65, 66, 67, 27, 91, 50, 75]);
    expect(output.text, '');
    output.append(12, [27, 91, 51, 49, 109]);
    output.append(13, [68]);
    expect(output.text, 'D');
  });

  test('marks truncated output and stays bounded', () {
    final output = WorkbenchTerminalOutput();
    output.append(1, [65]);
    output.append(2, [66], truncated: true);
    expect(output.text, contains('已被截断'));
    for (var i = 3; i < 4000; i++) {
      output.append(i, List.filled(80, 67));
    }
    expect(output.text.length, lessThanOrEqualTo(WorkbenchTerminalOutput.maxCharacters));
  });

  test('CSI and OSC sequences remain hidden across snapshot boundaries', () {
    final output = WorkbenchTerminalOutput();
    output.append(4, [65, 27, 91, 51]);
    output.append(7, [49, 109, 66]);
    expect(output.text, 'AB');
    output.append(13, [27, 93, 48, 59, 88, 27]);
    output.append(15, [92, 67]);
    expect(output.text, 'ABC');
    output.append(19, [27, 91, 50, 74]);
    expect(output.text, '');
  });
  test('overlapping snapshots do not duplicate byte ranges', () {
    final output = WorkbenchTerminalOutput();
    output.append(5, [65, 66, 67, 68, 69], fromCursor: 0);
    output.append(8, [68, 69, 70, 71, 72], fromCursor: 3);
    expect(output.text, 'ABCDEFGH');
    output.append(5, [65, 66, 67, 68, 69], fromCursor: 0);
    expect(output.text, 'ABCDEFGH');
  });

  test('real buffer overflow reports truncation and gap refuses undetected loss', () {
    final output = WorkbenchTerminalOutput();
    output.append(2, [65, 66], fromCursor: 0);
    expect(() => output.append(12, [67, 68], fromCursor: 10), throwsFormatException);
    output.append(12, [67, 68], fromCursor: 10, truncated: true);
    expect(output.text, contains('已被截断'));
    expect(output.text, endsWith('CD'));
  });

  test('UTF-8 continuation retains bytes after overlapping frames', () {
    final output = WorkbenchTerminalOutput();
    output.append(2, [0xe4, 0xb8], fromCursor: 0);
    output.append(4, [0xb8, 0xad, 0x0a], fromCursor: 1);
    expect(output.text, '中\n');
  });

}
