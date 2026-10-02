import 'package:flutter/material.dart';

const characterPersonalityDefaults = <String, int>{
  'affection': 45,
  'conflictAvoidance': 50,
  'emotionality': 50,
  'familiarity': 78,
  'formality': 22,
  'customerServiceAvoidance': 92,
  'directness': 75,
  'verbosity': 32,
  'structureLevel': 40,
  'shortSentence': 85,
  'toneWords': 45,
  'warmth': 58,
  'emotionalExpression': 45,
  'comfortLevel': 55,
  'preachingAvoidance': 88,
  'rationality': 62,
  'humor': 35,
  'teasing': 30,
  'initiative': 50,
  'patience': 60,
  'companionship': 55,
  'boundary': 85,
  'dependencyAvoidance': 85,
  'execution': 75,
  'explanationDepth': 55,
  'judgment': 75,
  'clarification': 35,
  'intimacyExpression': 25,
  'flirtiness': 0,
  'romanticTone': 0,
  'suggestivenessAvoidance': 100,
  'intimacyBoundary': 90,
};
const _groups = <String, Map<String, String>>{
  '关系亲近感': {
    'familiarity': '熟悉感',
    'formality': '正式度',
    'customerServiceAvoidance': '反客服感',
  },
  '表达风格': {
    'directness': '直接性',
    'verbosity': '啰嗦度',
    'structureLevel': '结构化',
    'shortSentence': '短句偏好',
    'toneWords': '语气词',
  },
  '情绪与温度': {
    'warmth': '温暖感',
    'emotionalExpression': '情绪表达',
    'comfortLevel': '安慰倾向',
    'preachingAvoidance': '反说教',
    'emotionality': '感性程度',
    'rationality': '理性程度',
  },
  '互动倾向': {
    'affection': '关心程度',
    'humor': '幽默感',
    'teasing': '打趣倾向',
    'initiative': '主动性',
    'patience': '耐心',
    'companionship': '陪伴感',
    'conflictAvoidance': '避免冲突',
  },
  '边界与执行': {
    'boundary': '边界感',
    'dependencyAvoidance': '避免依赖',
    'execution': '执行力',
    'explanationDepth': '解释深度',
    'judgment': '判断力',
    'clarification': '追问澄清',
  },
  '亲密安全': {
    'intimacyExpression': '亲密表达',
    'flirtiness': '调情感',
    'romanticTone': '浪漫色调',
    'suggestivenessAvoidance': '反暧昧',
    'intimacyBoundary': '亲密边界',
  },
};

class CharacterPersonalityEditor extends StatelessWidget {
  final Map<String, dynamic> value;
  final ValueChanged<Map<String, dynamic>> onChanged;
  const CharacterPersonalityEditor({
    super.key,
    required this.value,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) => Column(
    children: [
      for (final group in _groups.entries)
        ExpansionTile(
          title: Text(group.key),
          maintainState: true,
          children: [
            for (final field in group.value.entries)
              Builder(
                builder: (context) {
                  final number =
                      ((value[field.key] as num?) ??
                              characterPersonalityDefaults[field.key]!)
                          .toDouble()
                          .clamp(0.0, 100.0);
                  return Column(
                    children: [
                      Padding(
                        padding: const EdgeInsets.symmetric(horizontal: 16),
                        child: Row(
                          children: [
                            Expanded(child: Text(field.value)),
                            Text(number.round().toString()),
                          ],
                        ),
                      ),
                      Slider(
                        value: number,
                        min: 0,
                        max: 100,
                        divisions: 100,
                        label: number.round().toString(),
                        onChanged: (number) =>
                            onChanged({...value, field.key: number.round()}),
                      ),
                    ],
                  );
                },
              ),
          ],
        ),
    ],
  );
}
