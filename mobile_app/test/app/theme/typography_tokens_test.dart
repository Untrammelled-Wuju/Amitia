import 'package:amitia_app/app/theme/design_tokens.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('default mobile typography uses the compact scale', () {
    const tokens = AmitiaTypographyTokens();

    expect(tokens.pageTitleSize, 19);
    expect(tokens.pageLargeTitleSize, 22);
    expect(tokens.sectionTitleSize, 16);
    expect(tokens.cardTitleSize, 15);
    expect(tokens.bodySize, 14);
    expect(tokens.bodySmallSize, 13);
    expect(tokens.captionSize, 12);
    expect(tokens.labelSize, 11);
    expect(tokens.statusLabelSize, 10);
    expect(tokens.buttonSize, 14);
  });

  test('readable app typography uses restrained hierarchy', () {
    const tokens = AmitiaTypographyTokens.readable();

    expect(tokens.pageTitleSize, 18);
    expect(tokens.pageLargeTitleSize, 24);
    expect(tokens.sectionTitleSize, 16);
    expect(tokens.cardTitleSize, 15);
    expect(tokens.bodySize, 14);
    expect(tokens.bodySmallSize, 13);
    expect(tokens.captionSize, 12);
    expect(tokens.labelSize, 12);
    expect(tokens.statusLabelSize, 11);
    expect(tokens.buttonSize, 14);
    expect(tokens.pageTitleWeight, 500);
    expect(tokens.sectionTitleWeight, 400);
    expect(tokens.cardTitleWeight, 400);
    expect(tokens.bodyWeight, 400);
    expect(tokens.labelWeight, 400);
    expect(tokens.buttonWeight, 500);
  });
}
