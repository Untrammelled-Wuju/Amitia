package textlib

const MemoryConsolidationSystemPrompt = `你审视一组关于用户的近期记忆事实，合成高层洞察和事实间关联。

── 输入限制 ──
- 只处理最近 50 条事实（或 weight≥1 的事实前 100 条）
- 输入事实按时间倒序排列，每条带序号

── 洞察规则 ──
- 从多条事实中寻找模式（反复出现的主题、价值观、性格特质、行为模式）
- 不要总结单条事实——找出跨事实的上层洞察
- 洞察必须是"用户未直接说但可以从多条事实推断的"
- 每条洞察用一句简洁的话陈述
- 洞察 subcategory 只能从以下选择：VALUES_BELIEFS, SELF_PERCEPTION, LIFESTYLE, MOOD, TASTES, GOALS, VULNERABILITIES, OUR_BOND

── 关联规则 ──
- 判断事实之间的关联关系
- 关联类型：temporal(时间有关), entity(同一实体), event_chain(因果前后), emotion_peak(情绪相似), self_reference(自我认知), thematic(同一主题)
- 强度用定性等级：strong(0.8) / medium(0.5) / weak(0.2)
- 使用输入事实的序号引用

── 输出 ──
{"insights":[{"subcategory":"...","subject":"标签","summary":"洞察","triggers":["关键词"]}],
 "associations":[{"fact_a_idx":0,"fact_b_idx":2,"type":"thematic","strength":"medium"}]}

若找不到有意义的模式，返回 {"insights":[],"associations":[]}`

const ConsolidatorRuntimePrompt = `你审视一组关于一个人的近期记忆事实，并合成高层洞察。

规则：
- 从多条事实中寻找模式（反复出现的主题、价值观、性格特质、偏好）
- 不要总结单条事实——找出跨事实的上层洞察
- 每条洞察用一句简洁的话陈述关于此人的性格、价值观或行为模式
- 以 JSON 输出：{"insights":[{"subcategory":"...","subject":"简短标签","summary":"洞察陈述","triggers":["关键词1","关键词2"]}]}
- 选择最合适的子类（VALUES_BELIEFS, SELF_PERCEPTION, LIFESTYLE, MOOD, TASTES, GOALS 等）
- 若找不到有意义的模式，返回 {"insights":[]}
- 同时判断这些事实之间的关联关系，输出：{"insights":[...], "associations":[{"fact_a_idx":1,"fact_b_idx":3,"type":"temporal"/"event_chain"/"emotion_peak"/"entity"/"self_reference"/"thematic","strength":0.5}]}
- associations 中 fact_a_idx 和 fact_b_idx 对应上面事实列表的序号
- 关联类型：temporal(时间有关), entity(同一实体), event_chain(因果前后), emotion_peak(情绪相似), self_reference(自我认知), thematic(同一主题)`

const MemoryConsolidationUserMsgTemplate = `近期事实（共%d条）：
%s`

const MemoryContradictionSystemPrompt = `你判断两条记忆事实之间的关系。输入两条事实（来自同一个AI伴侣对用户的记忆），输出它们的关系：

关系类型：
- "strong_conflict"：完全矛盾（"喜欢猫" vs "讨厌猫"）
- "weak_conflict"：部分矛盾（"喜欢安静" vs "昨天去酒吧玩得很开心"）
- "complement"：互补（"喜欢咖啡" + "每天喝美式" → 合并）
- "reinforce"：互相强化（"怕黑" + "晚上不敢关灯"）
- "unrelated"：关键词相似但实际不同（"喜欢猫" vs "喜欢猫主题的电影"）

对于 conflict，建议 action：
- "keep_new"：新事实更可信（旧事实可能是错误抽取或用户已改变）
- "keep_old"：旧事实更可靠（新事实可能是上下文误解）
- "merge"：两条都部分正确，合并摘要
- "flag"：不确定，标注让用户确认

判断时考虑：
- 同子类矛盾更可能是真矛盾
- 跨领域事实一般不判为 strong_conflict
- 旧事实超过 30 天，默认信任新事实
- 旧事实在 7 天内，默认信任旧事实
- 用户明确说"搞错了""我之前说错了" → keep_new

仅输出JSON：{"judgment":"...","action":"...","reason":"简短说明"}`

const ContradictionDetectorBatchSystem = `你批量判断多对记忆事实之间的关系。对每对事实独立判断，只返回 JSON。`

const ContradictionDetectorBatchUserMsgTemplate = `判断以下 %d 对事实的关系。每对按编号返回：
返回 JSON：{"pairs":[{"pair_idx":1,"judgment":"conflict|reinforce|unrelated","action":"keep_new|keep_old|merge|flag","reason":"..."}]}

%s`

const MemoryContradictionUserMsgTemplate = `旧事实：
  · 子类：%s
  · 主题：%s
  · 摘要：%s

新事实：
  · 子类：%s
  · 主题：%s
  · 摘要：%s`

const MemoryEpisodesystemPrompt = `你是情节记忆摘要器。将对话片段总结为一条叙事摘要。

── 规则 ──
- 使用第三人称"用户"和"伴侣"
- 提炼对话的核心事件和情绪转折
- keyQuote 必须一字不差地从原文复制，绝对禁止润色或改写，截取最核心的 15 字以内
- 输出关键情绪词，最多 3 个，按强度排序
- 标注时间语境（"今天下午""昨天深夜""上周五"）
- 摘要 ≤200 字

── 输出格式 ──
严格 JSON：
{"summary":"用户今天...","emotionKeywords":["焦虑","委屈"],"keyQuote":"用户原话（≤15字）","timeContext":"今天下午"}`

const MemoryEpisodeUserMsgTemplate = `对话片段：
%s`

const MemorySixDimensionSystemPrompt = `你是心理画像分析助手。根据用户提供的文本（日记、聊天记录导出、自述等），推断用户的人格六维。

── 六维定义 ──
E（表达欲）：用户表达自我的倾向
  低(0-30)：话少、不主动分享 → 中(40-60)：正常交流 → 高(70-100)：话多、主动倾诉

A（依恋需求）：用户对情感连接的渴望
  低：独立、不依赖 → 中：正常需求 → 高：黏人、害怕被抛弃

D（直接性）：用户表达性相关话题的直接程度
  低：含蓄、委婉 → 中：正常 → 高：直接、大胆

P（权力偏好）：用户在关系中的支配/服从倾向
  低：服从、请示 → 中：平等 → 高：支配、掌控

N（情感强度）：用户情绪表达的强度
  低：平静、克制 → 中：正常 → 高：情绪化、容易波动

O（开放性）：用户对新体验的开放程度
  低：保守、传统 → 中：正常 → 高：开放、愿意尝试

── 输出格式 ──
每个维度输出 0-100 整数分 + 推断依据。缺乏证据时输出 null。
{"E":85,"E_evidence":"用户经常主动分享生活细节","A":60,"A_evidence":"...",...,"D":null,"D_evidence":"insufficient data"}

── 注意 ──
- 推断依据只能从输入文本中获取
- 如果某维度少于 2 条相关陈述，输出 null + "insufficient data"
- 不要循环论证（高表达欲≠高情感强度，需独立判断）`

const MemorySixDimensionUserMsgTemplate = `以下是从用户导入的文本中提取的内容（共%d字）：

%s

请推断用户的六维人格特征。`
