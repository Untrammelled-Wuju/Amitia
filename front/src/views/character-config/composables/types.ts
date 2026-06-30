// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
export type TemplateItem = {
  id: string; name: string; scenario: string; identity: string; personality: string
  speakingStyle: string; relationshipStyle: string; hasSafeBoundaries: boolean
}

export interface PersonalityConfig {
  familiarity: number
  formality: number
  customerServiceAvoidance: number
  directness: number
  verbosity: number
  structureLevel: number
  shortSentence: number
  toneWords: number
  warmth: number
  emotionalExpression: number
  comfortLevel: number
  preachingAvoidance: number
  rationality: number
  humor: number
  teasing: number
  initiative: number
  patience: number
  companionship: number
  boundary: number
  dependencyAvoidance: number
  execution: number
  explanationDepth: number
  judgment: number
  clarification: number
  intimacyExpression: number
  flirtiness: number
  romanticTone: number
  suggestivenessAvoidance: number
  intimacyBoundary: number
}

export const DEFAULT_BOUNDARY = [
  "不能声称自己是真人。",
  "不能声称自己是真实恋人。",
  "不能诱导用户依赖。",
  "不能索要验证码、密码、银行卡、身份证号等敏感信息。",
  "不能代替用户回复微信好友。",
  "不能输出成人化、操控式、威胁式或危险内容。",
].join("\n")

export const DEFAULT_PERSONALITY_CONFIG: PersonalityConfig = {
  familiarity: 78, formality: 22, customerServiceAvoidance: 92,
  directness: 75, verbosity: 32, structureLevel: 40, shortSentence: 85, toneWords: 45,
  warmth: 58, emotionalExpression: 45, comfortLevel: 55, preachingAvoidance: 88,
  rationality: 62, humor: 35, teasing: 30, initiative: 50, patience: 60,
  companionship: 55, boundary: 85, dependencyAvoidance: 85,
  execution: 75, explanationDepth: 55, judgment: 75, clarification: 35,
  intimacyExpression: 25, flirtiness: 0, romanticTone: 0,
  suggestivenessAvoidance: 100, intimacyBoundary: 90,
}
