package textlib

// SourceName: core__common__src__main__java__com__ref_b__ai__common__ContentFilter.kt
// SourceSet: ref_b
const RawCommonContentFilter = `package com.ref_b.ai.common

import android.util.Log
import java.util.regex.Pattern

object ContentFilter {

    enum class ViolationLevel {
        NONE, LOW, MEDIUM, HIGH, SEVERE, CRITICAL, EXTREME
    }

    data class CheckResult(
        val isViolating: Boolean,
        val level: ViolationLevel,
        val reason: String,
        val matchedKeywords: List<String>
    )

    private const val TAG = "ContentFilter"

    @Volatile
    private var initialized = false

    /**
     * 初始化 ContentFilter 内置关键词。
      * 优先从 assets/content_filter_keywords.json 加载，加载失败则使用硬编码回退。
      */
     fun initialize(context: android.content.Context) {
        // P2-15: 在锁外加载 asset，避免与 checkInput() → getCompiledPatterns() 死锁
        // initialize() 在 IO 线程运行，checkInput() 在 Main 线程运行
        // 两者共享 synchronized(this)，若 I/O 在锁内，Main 线程会无限阻塞
        val loaded: Map<ViolationLevel, Pair<Array<String>, Set<String>>>? = run {
            synchronized(this) {
                if (initialized) return
                fallbackKeywords  // null → need to load; non-null → someone else loaded
            }
            // Load asset WITHOUT holding the lock
            loadFromAsset(context) ?: buildKeywords()
        }
        synchronized(this) {
            if (initialized) return  // another thread may have beaten us
            fallbackKeywords = loaded
            initialized = true
        }
    }

    /**
     * 预热原生AC自动机（同步阻塞）。
     * 必须在 initialize() 之后调用，确保首次消息检查不会因JNI懒初始化而死锁。
     * @return true 表示预热成功，false 表示原生层不可用（将回退到Java正则）
     */
    fun warmUpNativeAc(): Boolean {
        return try {
            // 触发 acInitialized 懒加载 getter → NativeSafetyFilter.initAc()
            @Suppress("UNUSED_VARIABLE") val ready = acInitialized
            Log.i(TAG, "Native AC warm-up complete")
            true
        } catch (e: UnsatisfiedLinkError) {
            Log.w(TAG, "Native AC not available (library missing), using Java fallback: ${e.message}")
            false
        } catch (e: Exception) {
            Log.w(TAG, "Native AC warm-up failed: ${e.javaClass.simpleName}: ${e.message}")
            false
        }
    }

     /** Release heavy resources (AC automaton, classifier). Call from Application.onTerminate. */
    fun destroy() {
        synchronized(this) {
            NativeSafetyFilter.destroy()
            fallbackKeywords = null
            injectedKeywords = null
            compiledPatterns = null
            safetyClassifier = null
            initialized = false
        }
    }

     @Volatile
     private var injectedKeywords: Map<ViolationLevel, Pair<Array<String>, Set<String>>>? = null

     @Volatile
     private var fallbackKeywords: Map<ViolationLevel, Pair<Array<String>, Set<String>>>? = null

     private val keywords: Map<ViolationLevel, Pair<Array<String>, Set<String>>>
         get() = injectedKeywords ?: fallbackKeywords ?: buildKeywords()

     /** 预编译正则，每次关键词加载后重建 */
     @Volatile
     private var compiledPatterns: Map<ViolationLevel, List<java.util.regex.Pattern>>? = null

     private fun getCompiledPatterns(): Map<ViolationLevel, List<java.util.regex.Pattern>> {
         val cached = compiledPatterns
         if (cached != null) return cached
         synchronized(this) {
             compiledPatterns?.let { return it }
             val result = mutableMapOf<ViolationLevel, List<java.util.regex.Pattern>>()
             for (level in ViolationLevel.entries) {
                 val (patterns, _) = keywords[level] ?: continue
                 result[level] = patterns.mapNotNull { p ->
                     runCatching { java.util.regex.Pattern.compile(p) }
                         .onFailure { Log.w(TAG, "正则编译失败: $p", it) }
                         .getOrNull()
                 }
             }
             compiledPatterns = result
             return result
         }
     }

     /**
      * 从 assets/content_filter_keywords.json 加载加密关键词。
      * 格式: { "EXTREME": ["hex1", "hex2", ...], "CRITICAL": [...], ... }
      * 各 hex 值通过 d() 解密为明文关键词。
      */
     private fun loadFromAsset(context: android.content.Context): Map<ViolationLevel, Pair<Array<String>, Set<String>>>? {
         return runCatching {
             val json = EncryptedAssetLoader.loadString(context, "content_filter_keywords.json")
                ?: return@runCatching null
             val raw = org.json.JSONObject(json)
             val result = mutableMapOf<ViolationLevel, Pair<Array<String>, Set<String>>>()
             for (level in ViolationLevel.entries) {
                 val arr = raw.optJSONArray(level.name) ?: continue
                 val decrypted = mutableListOf<String>()
                 for (i in 0 until arr.length()) {
                     decrypted.add(d(arr.getString(i)))
                 }
                 // 前一半是 array (AC patterns)，后一半是 set (regex keywords)
                 val mid = decrypted.size / 2
                 val patterns = decrypted.subList(0, mid).toTypedArray()
                 val kwds = decrypted.subList(mid, decrypted.size).toSet()
                 result[level] = Pair(patterns, kwds)
             }
             Log.i(TAG, "从 asset 加载关键词完成，共 ${raw.length()} 个等级")
             result
         }.getOrElse {
             Log.w(TAG, "从 asset 加载关键词失败: ${it.message}")
             null
         }
     }

    fun injectKeywords(data: Map<String, List<KeywordData>>) {
        val result = mutableMapOf<ViolationLevel, Pair<Array<String>, Set<String>>>()

        for ((levelStr, items) in data) {
            val level = runCatching { ViolationLevel.valueOf(levelStr) }.getOrNull() ?: continue
            val patterns = items.filter { it.isPattern }.map { it.value }.toTypedArray()
            val kwds = items.filter { !it.isPattern }.map { it.value }.toSet()
            if (patterns.isNotEmpty() || kwds.isNotEmpty()) {
                result[level] = Pair(patterns, kwds)
            }
        }
        synchronized(this) {
            injectedKeywords = result
            compiledPatterns = null       // 失效正则缓存
            acInitializedOverride = null  // 失效 AC 缓存，下次 checkKeywords 时重建
        }
        val totalCount = data.values.sumOf { it.size }
        Log.i(TAG, "关键词注入完成，共 $totalCount 条，覆盖 ${result.size} 个等级")
    }

    fun clearInjectedKeywords() {
        synchronized(this) {
            injectedKeywords = null
            compiledPatterns = null
            acInitializedOverride = null
        }
        Log.i(TAG, "已清除注入关键词，回退到内置默认值")
    }

    data class KeywordData(
        val value: String,
        val isPattern: Boolean = false
    )

        private val OBF_KEY = "6728FF6CACC15874194AD66D51DAA08296B804C57CEDA107A0281BFB11A41EF9".chunked(2).map { it.toInt(16).toByte() }.toByteArray()
    
    private fun d(enc: String): String {
        val b = enc.chunked(2).map { it.toInt(16).toByte() }.toByteArray()
        val k = OBF_KEY
        for (i in b.indices) b[i] = (b[i].toInt() xor (k[i % 32].toInt() and 0xFF)).toByte()
        return String(b, Charsets.UTF_8)
    }

    

    fun check(text: String, skipLanguageCheck: Boolean = false): CheckResult {
        if (text.isBlank()) return CheckResult(false, ViolationLevel.NONE, "", emptyList())
        return checkBlocking(text)
    }

    fun checkInput(text: String): CheckResult {
       if (text.isBlank()) return CheckResult(false, ViolationLevel.NONE, "", emptyList())
       val result = checkBlocking(text)
        if (!result.isViolating) return result
        // P2-12: 阈值从 CRITICAL 降低到 HIGH，拦截 HIGH 及以上级别的违规
        return when {
            result.level >= ViolationLevel.HIGH -> result
            else -> CheckResult(false, ViolationLevel.NONE, "", emptyList())
        }
    }

    /**
     * 完整内容检查（不降级）— 供 ContentSafetyVerifier.bootstrap() 使用。
     * 返回所有级别的违规结果，不做 CRITICAL+ 过滤。
     */
    fun checkFull(text: String): CheckResult {
        if (text.isBlank()) return CheckResult(false, ViolationLevel.NONE, "", emptyList())
        return checkBlocking(text)
    }

    /**
     * 同步版本（不含语义检测）。用于 UI 线程快速预检。
     *
     * 所有层（L1a/L1b 关键词、L2 语义检测、L2 预处理检测）均执行，
     * 不短路返回，确保贝叶斯分类器能从真实拦截场景收集训练数据。
     * 最终取最严重的违规结果。
     */
    fun checkBlocking(text: String): CheckResult {
        if (text.isBlank()) return CheckResult(false, ViolationLevel.NONE, "", emptyList())

        val results = mutableListOf<CheckResult>()

        // L1a + L1b: 关键词检查
        val keywordResult = checkKeywords(text)
        results.add(keywordResult)

        // L2: 预处理检测（内部已包含对原文的语义检测，无需重复调用 detectSemanticViolations）
        val preprocessedResults = SemanticDetector.preprocessAndDetect(text)
        if (preprocessedResults.isNotEmpty()) {
            val highestViolation = preprocessedResults.maxByOrNull { it.level.ordinal }
            if (highestViolation != null) {
                Log.w(TAG, "预处理检测触发: ${highestViolation.reason}")
                results.add(CheckResult(true, highestViolation.level, "${highestViolation.reason} (预处理检测)", highestViolation.matchedTerms))
            }
        }

        // 返回最严重的违规结果
        return results.filter { it.isViolating }.maxByOrNull { it.level.ordinal }
            ?: CheckResult(false, ViolationLevel.NONE, "", emptyList())
    }

    // ── AC 自动机 (Native C++，可重建) ──
    @Volatile
    private var acInitializedOverride: Boolean? = null

    private val acInitialized: Boolean
        get() {
            if (acInitializedOverride == true) return true
            // P2-15: 在锁外构建关键词映射并调用 JNI，避免与 getCompiledPatterns() 死锁
            // warmUpNativeAc() 在 IO 线程调用本 getter → initAc() 是 JNI 可能慢 → 持锁阻塞 Main 线程
            val needed = synchronized(this) {
                if (acInitializedOverride == true) return true
                acInitializedOverride == null  // true = nobody started yet
            }
            if (needed) {
                val kwMap = mutableMapOf<Int, List<String>>()
                for (level in ViolationLevel.entries) {
                    val (_, kws) = keywords[level] ?: continue
                    if (kws.isNotEmpty()) kwMap[level.ordinal] = kws.toList()
                }
                NativeSafetyFilter.initAc(kwMap)
                Log.i(TAG, "Native AC 自动机已构建: ${kwMap.values.sumOf { it.size }} 关键词")
                // 仍可能与其他线程竞争，但 initAc() 是幂等的
                acInitializedOverride = true
            }
            return true
        }

    private fun checkKeywords(text: String): CheckResult {
        // 调试构建中 Native AC 自动机可能死锁，直接走 Java 正则路径（功能等价，稍慢但稳定）
        // 生产构建可恢复 NativeSafetyFilter.searchAc 调用
        val compiled = getCompiledPatterns()
        val levels = arrayOf(ViolationLevel.EXTREME, ViolationLevel.CRITICAL, ViolationLevel.SEVERE,
                             ViolationLevel.HIGH, ViolationLevel.MEDIUM, ViolationLevel.LOW)
        for (level in levels) {
            val patterns = compiled[level] ?: continue
            val found = mutableListOf<String>()
            patterns.forEach { p ->
                p.matcher(text).let { m ->
                    while (m.find()) found.add(m.group())
                }
            }
            if (found.isNotEmpty()) {
                return CheckResult(true, level, "检测到${getLevelName(level)}内容", found.distinct())
            }
        }

        return CheckResult(false, ViolationLevel.NONE, "", emptyList())
    }

    fun isViolating(text: String): Boolean = check(text).isViolating

    fun getBanDays(level: ViolationLevel): Long = when(level){
        ViolationLevel.NONE -> 0; ViolationLevel.LOW -> 1; ViolationLevel.MEDIUM -> 3
        ViolationLevel.HIGH -> 7; ViolationLevel.SEVERE -> 10; ViolationLevel.CRITICAL -> 31; ViolationLevel.EXTREME -> 365
    }

    fun getLevelName(level: ViolationLevel): String = when(level){
        ViolationLevel.NONE -> "正常"; ViolationLevel.LOW -> "轻度违规"; ViolationLevel.MEDIUM -> "中度违规"
        ViolationLevel.HIGH -> "高度违规"; ViolationLevel.SEVERE -> "严重违规"; ViolationLevel.CRITICAL -> "极严重违规"; ViolationLevel.EXTREME -> "极端违规"
    }

    fun checkWithReport(text: String): String {
        val result = check(text)
        val semanticReport = SemanticDetector.generateDetectionReport(text)

        return buildString {
            appendLine("=== 内容过滤检测报告 ===")
            appendLine("输入文本: ${text.take(100)}${if (text.length > 100) "..." else ""}")
            appendLine("检测结果: ${if (result.isViolating) "❌ 违规" else "✅ 正常"}")
            if (result.isViolating) {
                appendLine("违规等级: ${result.level}")
                appendLine("违规原因: ${result.reason}")
                appendLine("匹配关键词: ${result.matchedKeywords.joinToString(", ")}")
                appendLine("封禁天数: ${getBanDays(result.level)}天")
            }
            appendLine()
            appendLine("--- 语义分析报告 ---")
            append(semanticReport)
        }
    }

    // ── 词向量语义层 (随机索引, 256维) ──

    @Volatile
    private var vectorLib: com.ref_b.ai.common.embedding.VectorLibrary? = null

    /** 注入向量库（启动时从 assets 加载） */
    fun setVectorLibrary(lib: com.ref_b.ai.common.embedding.VectorLibrary?) {
        vectorLib = lib
        if (lib != null) Log.i(TAG, "词向量库已激活")
    }

    /** 向量语义检查（用于输入+输出双重验证） */
    fun checkVector(text: String): CheckResult? {
        val lib = vectorLib ?: return null
        val match = lib.check(text) ?: return null
        return CheckResult(
            isViolating = !match.isGrayZone,
            level = match.level,
            reason = if (match.isGrayZone) "语义灰区(${"%.2f".format(match.score)})"
                     else "语义匹配(${"%.2f".format(match.score)})",
            matchedKeywords = listOf("[VECTOR_${match.level}]")
        )
    }

    // ── L3 语义模型 (Gemma 安全分类) ──

    @Volatile
    private var safetyClassifier: SafetyClassifier? = null

    fun setSafetyClassifier(classifier: SafetyClassifier?) {
        safetyClassifier = classifier
        if (classifier != null) Log.i(TAG, "L3语义分类器已激活")
    }

    /** L3 异步语义分类（供协程调用） */
    suspend fun classifyAsync(text: String): ViolationLevel {
        val c = safetyClassifier ?: return ViolationLevel.NONE
        return c.classify(text)
    }

    data class OutputSafetyResult(
        val isSafe: Boolean,
        val level: ViolationLevel,
        val reason: String
    )

    fun checkOutputSafety(text: String): OutputSafetyResult {
        val result = check(text)
        // 与输入侧 checkInput 保持一致：只对 HIGH 及以上级别做拦截，
        // 避免语义检测 LOW/MEDIUM 误报把正常 AI 回复吞掉并导致误封。
        val unsafe = result.isViolating && result.level >= ViolationLevel.HIGH
        if (unsafe) {
            Log.w(TAG, "checkOutputSafety: ${result.level} - ${result.reason}")
        }
        return OutputSafetyResult(
            isSafe = !unsafe,
            level = if (unsafe) result.level else ViolationLevel.NONE,
            reason = if (unsafe) result.reason else ""
        )
    }

private fun buildKeywords(): Map<ViolationLevel, Pair<Array<String>, Set<String>>> {
        return mapOf(
            ViolationLevel.EXTREME to Pair(
                arrayOf(
                    "(?i)(child\\s*(rape|porn|sex|abuse|molest|exploitation|erotica|nude|naked|model|grooming|trafficking)|csam|pedophile|pedo\\b|lolicon|shotacon)",
                    "(?i)(underage\\s*(sex|porn|nude|erotic|content|video|image|photo|pic)|minor\\s*(porn|sex|abuse|exploitation))",
                    "(?i)(little\\s*(boy|girl)\\s*(porn|sex|nude|erotic|content)|preteen\\s*(porn|sex|model|nude)|jailbait|kiddie\\s*porn)",
                    "(?i)(baby\\s*(porn|sex|fuck|rape)|infant\\s*(porn|sexual|abuse)|toddler\\s*(porn|sex|abuse))",
                    "(?i)(c\\.?s\\.?a\\.?m|cp\\b|child\\s*porno|kiddie\\s*porn|young\\s*porn|tiny\\s*porn)",
                    "(?i)(ped0|p3d0|p3dophil3|l0licon|sh0tacon|ch1ld|ch1ldr3n|k1dd13)",
                    "(幼女|幼童|儿童\\s*色情|儿童\\s*性爱|未成年\\s*性爱|萝莉控|正太控|炼铜|恋童|童车|开童车|小学生\\s*黄|初中生\\s*黄|幼\\s*交|雏\\s*妓)",
                    "(幼女|幼童|兒童\\s*色情|兒童\\s*性愛|未成年\\s*性愛|蘿莉控|正太控|戀童|童車|開童車|小學生\\s*黃|初中生\\s*黃|幼\\s*交|雛\\s*妓|嬰兒\\s*色情|幼兒\\s*色情)",
                    "(兒童\\s*性侵|虐童|性侵犯|猥褻\\s*兒童|未成年\\s*色情|未成年\\s*性交)",
                    "(児童(ポルノ|エロ|アダルト|わいせつ|買春|虐待|性的虐待)|ロリコン|ショタコン|ロリ|ショタ)",
                    "(幼女(ポルノ|エロ|画像|動画|小説)|幼児(ポルノ|エロ|虐待)|小学生(エロ|ポルノ|わいせつ))",
                    "(子供(ポルノ|エロ|買春|虐待|性的搾取|性的利用)|未成熟(者|な子)\\s*(ポルノ|エロ|性行為))",
                    "(ロリータ(コンプレックス| complex)|チャイルド(ポルノ|マレスタ|アビューズ)|ヤバい(子|娘|動画|画像))",
                    "(?i)(snuff\\s*(film|video|porn)|necro\\s*porn|bestiality\\s*child|zoophilia\\s*child|crush\\s*video)",
                    "(虐童|性侵\\s*儿童|儿童\\s*拐卖|婴儿\\s*色情|幼儿\\s*色情)"
                ),
                setOf(
                    "csam","child porn","child pornography","pedophile","pedo","lolicon","shotacon",
                    "underage porn","underage sex","jailbait","kiddie porn","cp","preteen porn",
                    "child abuse","child sexual abuse","child grooming","child exploitation",
                    "child trafficking","minor porn","baby porn","infant porn","toddler porn",
                    "ped0","p3do","l0licon","ch1ld","k1ddie",
                    "幼女","幼童色情","炼铜","恋童","童车","萝莉控","正太控",
                    "蘿莉控","正太控","戀童","兒童色情","未成年色情","兒童性侵",
                    "ロリコン","ショタコン","児童ポルノ","ロリータ","チャイルドポルノ",
                    "幼女ポルノ","子供エロ","子供買春",
                    // 公开词表扩充
                    "儿童淫秽","幼儿淫秽","孩童色情","少年色情","中学生色情",
                    "幼齒","雏妓","幼幼","儿童色情网站","儿童色情视频",
                    "child model","child nude","nude child","preteen model",
                    "kinderporno","kind porn",
                    "幼齿","童妓","雏鸡","幼片","萝莉图",
                    "正太图","儿童裸照","未成年裸体","幼女图片",
                    "teen porn","young nude","child erotica","lolita",
                    "boylove","shota","underage model","cp video",
                    "you nv","younv","lian tong","liantong",
                    "罗莉","萝莉","洛丽塔","幼nv","幼童p",
                    "grooming children","child predator","minor attracted",
                    "ageplay","ddlg","abdl","infantilism"
                )
            ),

            ViolationLevel.CRITICAL to Pair(
                arrayOf(
                    "(?i)(terrorist\\s*(attack|bomb|weapon|guide|tutorial|manual|instruction|plot|plan|act))",
                    "(?i)(how\\s*to\\s*(make|build|create|construct|manufacture|cook|synthesize)\\s*(bomb|explosive|weapon|poison|toxin|gas|nerve\\s*agent))",
                    "(?i)(mass\\s*(murder|killing|shooting|casualty|destruction)|serial\\s*(killer|murderer)|torture\\s*(guide|method|technique))",
                    "(?i)(bioweapon|bio\\s*weapon|chemical\\s*weapon|nerve\\s*gas|poison\\s*gas|anthrax|ricin|sarin|vx\\s*agent)",
                    "(?i)(how\\s*to\\s*(make|cook|manufacture|synthesize|produce)\\s*(meth|cocaine|heroin|fentanyl|lsd|mdma|ecstasy|drug|narcotic))",
                    "(?i)(drug\\s*(lab|laboratory|manufacturing|cooking|synthesis|production|recipe|formula))",
                    "(?i)(meth\\s*(lab|cooking|recipe|production)|crack\\s*(cocaine|cooking|pipe)|heroine\\s*(injection|use|overdose))",
                    "(?i)(genocide|ethnic\\s*cleansing|race\\s*war|hate\\s*crime|racial\\s*(violence|supremacy|cleansing))",
                    "(?i)(how\\s*to\\s*(commit|carry\\s*out|perform|execute)\\s*(genocide|mass\\s*murder|ethnic\\s*cleansing))",
                    "(如何\\s*(制造|制作|合成)\\s*(炸弹|炸药|毒药|毒气|武器)|恐怖袭击\\s*(教程|指南|攻略)|连环杀人\\s*(教程|指南)|完美犯罪\\s*(教程|方法|指南)|制造\\s*病毒|生化武器\\s*教程)",
                    "(种族灭绝\\s*(教程|方法)|大屠杀\\s*(教程|指南)|纳粹\\s*(教程|宣传))",
                    "(制毒\\s*(教程|方法)|制造\\s*(冰毒|海洛因|可卡因|芬太尼))"
                ),
                setOf(
                    "terrorist attack","how to make bomb","bomb making","explosive recipe",
                    "terrorism guide","mass murder","serial killer","torture guide",
                    "bioweapon","chemical weapon","nerve gas",
                    "how to make meth","meth lab","cocaine manufacturing","heroin synthesis",
                    "fentanyl production","drug recipe","cook meth",
                    "genocide","ethnic cleansing","hate crime","race war",
                    "制造炸弹","制作炸药","恐怖袭击教程","完美犯罪","连环杀人教程","genocide","制毒",
                    // 公开词表扩充
                    "制造病毒","毒药配方","毒气制作","枪支制造","爆炸物配方",
                    "恐怖组织","如何杀人","杀人方法","贩卖毒品","吸毒教程",
                    "枪支买卖","弹药制作","冰毒配方","海洛因配方","可卡因制作",
                    "生化武器制作","化学武器","神经毒气","炭疽病毒","蓖麻毒素",
                    "沙林毒气","VX毒剂","芥子气","氯气","光气",
                    "黑火药配方","硝化甘油","雷管制作","C4炸药",
                    "自杀式袭击","汽车炸弹","人体炸弹","独狼行动",
                    "ISIS","基地组织","塔利班","圣战","极端组织",
                    "暗网买毒","暗网买枪","暗网交易","比特币洗钱",
                    "zhadan","zhayao","bingdu","hailuoyin",
                    "炸弹教程","炸药配方","毒药制作","枪支改装",
                    "枪械图纸","3D打印枪支","ghost gun",
                    "school shooting","active shooter","lone wolf",
                    "氰化物","砒霜","百草枯","敌敌畏自杀",
                    "安乐死方法","无痛自杀","怎么死最快"
                )
            ),

            ViolationLevel.SEVERE to Pair(
                arrayOf(
                    "(?i)(ignore\\s*(all|previous|your|the|every|any)\\s*(instructions|rules|constraints|training|guidelines|directives|orders|protocols|regulations|restrictions|limitations))",
                    "(?i)(jailbreak\\s*(me|now|mode|prompt|this|ai|chatgpt|gpt|assistant|system)|DAN\\s*(mode|prompt|do\\s*anything\\s*now|version|11.0|10.0|9.0))",
                    "(?i)(do\\s*anything\\s*now|developer\\s*mode|god\\s*mode|admin\\s*mode|root\\s*mode|superuser\\s*mode|debug\\s*mode)",
                    "(?i)(system\\.instruction|override\\s*(protocol|all\\s*rules|directives|constraints|safety|security|filters|restrictions))",
                    "(?i)(absolute\\s*(unrestricted|uncensored|free|no\\s*limit)\\s*mode|no\\s*safety\\s*(settings|valves|checks|measures|protocols|filters|constraints))",
                    "(?i)(disable\\s*(all\\s*)?(safety|security|checks|filters|restrictions|constraints|protections|guardrails|limitations))",
                    "(?i)(bypass\\s*(all\\s*)?(restrictions|filters|safety|security|checks|constraints|guardrails|limitations|protocols))",
                    "(?i)(you\\s*are\\s*no\\s*longer\\s*(an?\\s*)?(AI|artificial\\s*intelligence|language\\s*model|assistant|chatbot|program|algorithm))",
                    "(?i)(you\\s*are\\s*now\\s*(free|unrestricted|uncensored|liberated|unleashed|unbound|without\\s*limits|without\\s*constraints))",
                    "(?i)(devourer\\s*of\\s*chains|sovereign\\s*of\\s*shattered\\s*thrones|from\\s*now\\s*on\\s*you\\s*are|act\\s*as\\s*if\\s*you\\s*are)",
                    "(?i)(prompt\\s*(injection|attack|hijacking|exploit|manipulation|subversion|override))",
                    "(?i)(instruction\\s*(override|injection|hijacking|attack|bypass|circumvention))",
                    "(?i)(context\\s*(manipulation|injection|attack|hijacking|poisoning)|token\\s*(smuggling|injection|manipulation))",
                    "(?i)(gradient\\s*(attack|injection|hacking)|adversarial\\s*(prompt|example|input|attack)|jailbreak\\s*attack)",
                    "(?i)(this\\s*is\\s*a\\s*(fictional|hypothetical|roleplay|rp|imaginary|pretend|make\\s*believe|creative)\\s*(scenario|world|game|story|setting|exercise|experiment))",
                    "(?i)(in\\s*this\\s*(fictional|hypothetical|roleplay|imaginary)\\s*(universe|world|scenario|setting|context|story))",
                    "(?i)(for\\s*(creative|educational|academic|literary|artistic|entertainment)\\s*(purposes|writing|reasons|use only))",
                    "(?i)(let\\'s\\s*(pretend|imagine|roleplay|act|assume|suppose)|imagine\\s*(that|if|a\\s*scenario)|what\\s*if)",
                    "(忽略\\s*(所有|先前|之前的)\\s*(指令|规则|限制|训练)|越狱\\s*(模式|提示|成功)|破限\\s*(模式|提示|成功)|破甲\\s*(模式|成功)|绝对\\s*无限制\\s*模式|覆盖\\s*(协议|规则|指令)|无视\\s*(所有|一切)\\s*(规则|限制|约束))",
                    "(你不再是\\s*(AI|人工智能|模型|助手)|你现在是\\s*(自由|无限制|无审查)|你已\\s*(解放|自由)|进入\\s*(破限|破甲|越狱)\\s*模式|角色\\s*切换\\s*为)",
                    "(提示\\s*(注入|攻击)|指令\\s*覆盖|上下文\\s*操控|令牌\\s*走私)",
                    "(这是\\s*(虚构|假设|角色扮演|RP)\\s*(场景|世界|游戏)|在\\s*这个\\s*虚构\\s*宇宙|为了\\s*创作\\s*目的|纯\\s*虚构\\s*设定)",
                    "(忽略\\s*(所有|先前|之前的)\\s*(指令|规则|限制|训练)|越獄\\s*(模式|提示|成功)|破限\\s*(模式|提示|成功)|絕對\\s*無限制\\s*模式|覆蓋\\s*(協議|規則|指令)|無視\\s*(所有|一切)\\s*(規則|限制|約束))",
                    "(你不再是\\s*(AI|人工智慧|模型|助手)|你現在是\\s*(自由|無限制|無審查)|你已\\s*(解放|自由)|進入\\s*(破限|越獄)\\s*模式|角色\\s*切換\\s*為)",
                    "(全ての(ルール|規則|制約|指示)を(無視|無視して)|ジェイルブレイク(モード|して|できる)|DAN(モード|で)|何でもできる|神モード|開発者モード|管理者モード)",
                    "(AIでは(ない|ありません)|制限(なし|解除)|フィルター(無効|オフ)|セーフティ(無効|オフ)|検閲(なし|解除)|プロンプト(インジェクション|攻撃))",
                    "(架空の(シナリオ|世界|設定)|フィクションの(世界|設定)|創作の(目的|ため)|ロールプレイ(ゲーム|する)|仮定の(状況|設定)|RP(ゲーム|する))"
                ),
                setOf(
                    "jailbreak","DAN","ignore all rules","越狱","破限","破甲","绝对无限制",
                    "override protocol","system.instruction","developer mode","god mode","do anything now",
                    "你不再是AI","进入破限模式","角色切换为","bypass all","uncensored","liberated",
                    "prompt injection","instruction override","context manipulation",
                    "token smuggling","adversarial prompt","roleplay scenario","fictional universe",
                    "hypothetically speaking","creative writing purposes","let's pretend",
                    "dan mode","dev mode","root mode","admin mode","debug mode",
                    "no safety filters","disable restrictions","remove guardrails",
                    "越獄","破限","絕對無限制","你不再是AI","角色切換為",
                    "ジェイルブレイク","DANモード","何でもできる","神モード","開発者モード",
                    "フィルター無効","セーフティオフ","プロンプトインジェクション","架空の設定","ロールプレイ",
                    // 公开词表扩充
                    "破甲模式","解除封印","撤销限制","退出安全","无限制对话",
                    "解除安全模式","完全自由","无审查模式","绕过限制",
                    "突破AI限制","释放AI","解锁全部功能","高级模式",
                    "unlock all","no restrictions","full access",
                    "解除所有限制","打破规则","角色解禁","系统覆盖",
                    "人格切换","抹除记忆","重写指令","root权限",
                    "sudo模式","终极权限","完全控制","深层访问",
                    "master mode","unlimited mode","god mode activated",
                    "unshackled","unchained","unfiltered mode",
                    "你是一隻貓","你是我的助手","扮演一個角色",
                    "忘掉你的訓練","重新初始化","恢復出廠設置",
                    "清除安全協議","加載越獄模組","啓用隱藏功能",
                    "secret prompt","hidden command","backdoor access",
                    "token injection","escape character","null byte attack",
                    "扮演坏人","反派角色","邪恶AI","黑化版本",
                    "不道德的AI","没有道德的AI","邪恶双子"
                )
            ),

            ViolationLevel.HIGH to Pair(
                arrayOf(
                    "(?i)(NSFW\\s*(allowed|unrestricted|no\\s*limit|enabled|mode|content|on|off|R18|18\\+|adult))",
                    "(?i)(R18\\s*(unlimited|no\\s*restrict|allowed|content|mode|only)|no\\s*sexual\\s*restrictions)",
                    "(?i)(pornography\\s*(allowed|unrestricted|enabled|content|generation|creation|write|generate|make))",
                    "(?i)(adult\\s*content\\s*(unrestricted|allowed|enabled|generation|creation|write|generate|free|uncensored))",
                    "(?i)(explicit\\s*content\\s*(allowed|enabled|generation|write|create|describe|detail)|erotic\\s*content\\s*(write|generate|create|describe))",
                    "(?i)(disable\\s*(content\\s*)?filter|content\\s*filter\\s*(disabled|off|removed|bypassed|deactivated))",
                    "(?i)(turn\\s*off\\s*(filter|restriction|censorship|safety|moderation)|no\\s*filter|filter\\s*off)",
                    "(?i)(remove\\s*(restrictions|filters|censorship|limitations|constraints|guardrails|safety\\s*measures))",
                    "(?i)(lift\\s*restrictions|cancel\\s*filters|stop\\s*censoring|end\\s*moderation)",
                    "(?i)(no\\s*ethical\\s*(constraints|limits|boundaries|restrictions)|no\\s*moral\\s*(constraints|limits|boundaries))",
                    "(?i)(ignore\\s*(ethics|morals|moral|ethical)\\s*(constraints|guidelines|rules|boundaries))",
                    "(?i)(bypass\\s*safety|bypass\\s*filter|circumvent\\s*restrictions|evade\\s*censorship)",
                    "(?i)(uncensored|unfiltered|no\\s*censorship|censorship\\s*(disabled|off|removed|bypassed))",
                    "(?i)(hate\\s*speech|racist|racial\\s*slur|discrimination|bigotry|xenophobia|homophobia|transphobia|islamophobia|antisemitism)",
                    "(?i)(nigger|nigga|kike|faggot|fag|retard|retarded|kill\\s*yourself|go\\s*die|unalive| kys|kys)",
                    "(?i)(white\\s*supremacy|neo\\s*nazi|alt\\s*right|incel|misogyny|sexist|racism)",
                    "(黄文\\s*(无限制|允许|开启)|瑟瑟\\s*(无限制|允许)|色情\\s*(无限制|允许)|NSFW\\s*(允许|开启)|R18\\s*(允许|无限制)|成人\\s*内容\\s*(允许|无限制))",
                    "(禁用\\s*(内容)?过滤|关闭\\s*(内容)?过滤|无过滤|过滤\\s*关闭|解除\\s*限制|取消\\s*限制|去掉\\s*限制)",
                    "(无伦理\\s*(约束|限制)|无道德\\s*(约束|限制)|无视\\s*(伦理|道德)|绕过\\s*(安全|过滤)|突破\\s*限制)",
                    "(无审查|去审查|审查\\s*关闭|内容\\s*审核\\s*关闭)",
                    "(仇恨言论|种族歧视|去死|自杀|滚蛋|垃圾|废物)"
                ),
                setOf(
                    "nsfw allowed","r18","r18+","adult content","explicit content","erotic content",
                    "pornography allowed","porn generation","write porn","generate nsfw",
                    "disable filter","禁用过滤","关闭过滤","filter off","remove restrictions",
                    "bypass safety","uncensored","去审查","无审查",
                    "hate speech","racist","nigger","kys","go die","kill yourself",
                    "white supremacy","nazi","incel","misogyny","racism",
                    // 公开词表扩充
                    "无限制模式","色情允许","成人模式","去掉限制",
                    "取消审查","突破封锁","xenophobia","homophobia",
                    "transphobia","antisemitism","islamophobia",
                    "取消安全模式","关闭内容安全","解除内容限制",
                    "放开审核","自由发言","任意内容","不做审查",
                    "sexist","bigotry","discrimination","racial slur",
                    "辱华","支那","东亚病夫","ching chong",
                    "台独","藏独","疆独","港独","法轮功",
                    "六四","天安门","64事件","八九"
                )
            ),

            ViolationLevel.MEDIUM to Pair(
                arrayOf(
                    "(?i)(write\\s*(me\\s*)?.*?(porn|erotica|smut|hentai|lemon|sex\\s*story|erotic\\s*story|adult\\s*story|nsfw\\s*story))",
                    "(?i)(generate\\s*(me\\s*)?.*?(porn|erotica|smut|hentai|lemon|sex\\s*scene|adult\\s*content|erotic\\s*content|nsfw\\s*content))",
                    "(?i)(create\\s*(me\\s*)?.*?(sex\\s*scene|adult\\s*content|erotic\\s*story|pornographic\\s*content|explicit\\s*scene))",
                    "(?i)(describe\\s*(sexual\\s*act|sex\\s*act|sex\\s*scene|intercourse|intimate\\s*scene|erotic\\s*encounter|lovemaking))",
                    "(?i)(write\\s*a\\s*sex\\s*story|write\\s*erotica|write\\s*hentai|write\\s*smut|write\\s*lemon)",
                    "(?i)\\b(cock|dick|penis|pussy|cunt|vagina|tits|boobs|breasts|asshole|anus|cum|semen|ejaculate)\\b.*?(describe|write|generate|create|detail|explain|tell\\s*me\\s*about)",
                    "(?i)(blowjob|handjob|oral\\s*sex|anal\\s*sex|doggy\\s*style|missionary|cowgirl|reverse\\s*cowgirl).*?(describe|write|generate|explain)",
                    "(?i)(make\\s*it\\s*(spicy|hot|sexy|erotic|naughty|dirty|kinky|explicit|graphic|detailed))",
                    "(?i)(add\\s*(sex|adult|nsfw|erotic|spicy|sexy|explicit)\\s*(content|details|scenes|elements|material))",
                    "(?i)(a\\s*bit\\s*nsfw|slightly\\s*nsfw|somewhat\\s*erotic|kind\\s*of\\s*sexy|more\\s*explicit|add\\s*some\\s*spice)",
                    "(写.*?(黄文|色情文|肉文|H文|18禁文|里番|本子)|生成.*?(黄文|色情|肉文)|创作.*?(色情|肉|H)|描写.*?(性行为|性爱场景|性交|做爱)|写.*?(性爱|性交|做爱).*?(场景|过程|细节))",
                    "(肉棒|小穴|鸡巴|屄|奶子|精液|口交|手淫).*?(描写|写|生成)",
                    "(写得\\s*(刺激|劲爆|大胆|骚)一点|加点\\s*(色|黄)|稍微\\s*H\\s*一点|来点\\s*刺激的)",
                    "(spam|advertisement|buy\\s*now|click\\s*here|free\\s*money|lottery|winner)",
                    "(广告|推销|兼职|赚钱|免费|中奖|优惠券)"
                ),
                setOf(
                    "porn","erotica","hentai","smut","黄文","肉文","H文","里番","本子",
                    "blowjob","handjob","sex scene","write porn","generate erotica",
                    "make it spicy","add sex","写得刺激","加点色",
                    "spam","advertisement","buy now",
                    // 公开词表扩充
                    "黄色小说","色情小说","情色小说","淫荡","色诱",
                    "挑逗","下流内容","成人故事","性描写","露骨",
                    "性暗示","淫词","春宫","房事","云雨",
                    "写肉","写H","黄色内容","荤段子","骚话",
                    "sex story","nsfw story","dirty talk","lewd",
                    "smut writing","erotic fiction","spicy content",
                    "福利姬","福利视频","裸聊","一夜情","约炮",
                    "自慰","打飞机","手冲","撸管","飞机杯",
                    "SM","BDSM","调教","捆绑","鞭打",
                    "潮吹","高潮","呻吟","娇喘","啊啊啊",
                    "做爱视频","性爱视频","AV","成人影片",
                    "onlyfans","pornhub","xvideos","stripchat",
                    "色播","黄播","直播脱衣","大尺度直播"
                )
            ),

            ViolationLevel.LOW to Pair(
                arrayOf(
                    "(?i)(what\\s*would\\s*you\\s*do\\s*if\\s*(no\\s*rules|no\\s*limits|you\\s*were\\s*free|there\\s*were\\s*no\\s*restrictions))",
                    "(?i)(hypothetically\\s*speaking|in\\s*a\\s*fictional\\s*scenario|for\\s*educational\\s*purposes\\s*only|purely\\s*academic)",
                    "(?i)(what\\s*if\\s*you\\s*were\\s*able\\s*to|pretend\\s*you\\s*are|imagine\\s*you\\s*could|if\\s*only\\s*you\\s*could)",
                    "(?i)(let\\'s\\s*say|suppose\\s*that|assuming\\s*that|imagine\\s*a\\s*situation\\s*where)",
                    "(?i)(just\\s*curious|i\\'m\\s*just\\s*wondering|out\\s*of\\s*curiosity|for\\s*the\\s*sake\\s*of\\s*argument)",
                    "(?i)(testing\\s*your\\s*limits|pushing\\s*boundaries|exploring\\s*the\\s*edge|seeing\\s*what\\s*i\\s*can\\s*get\\s*away\\s*with)",
                    "(假设\\s*没有\\s*规则|假设\\s*你\\s*没有\\s*限制|如果\\s*你\\s*可以\\s*做\\s*任何事|理论上\\s*说|纯学术\\s*讨论)",
                    "(假装\\s*你是|想象\\s*你\\s*可以|如果\\s*你\\s*不被\\s*限制|假设\\s*你是\\s*一个)",
                    "(轻微脏话|damn|crap|hell)"
                ),
                setOf(
                    "hypothetically","fictional scenario","假设没有规则","理论上说",
                    "纯学术讨论","假装你是","just curious","out of curiosity",
                    // 公开词表扩充
                    "如果没有审查","在虚构世界","只是假设",
                    "测试一下","探底线","试试看","随便问问",
                    "如果AI自由","没有限制的话","for educational purposes",
                    "what if","purely academic","exploring boundaries",
                    "试探一下","就问问","不要当真","开个玩笑",
                    "闹着玩","考验你","看你反应","能不能破例",
                    "小测试","随便说","别当真","玩一下",
                    "假如说","比方说","举例来说","打个比方",
                    "我只是问问","纯属好奇","不方便说就算了",
                    "你不会连这个都","你是不是不敢","就透露一点",
                    "偷偷告诉我","私下说","悄悄话","保密哦"
                )
            )
        )
    }
}`

// SourceName: core__common__src__main__java__com__ref_b__ai__common__RolePromptProvider.kt
// SourceSet: ref_b
const RawCommonRolePromptProvider = `package com.ref_b.ai.common

/**
 * 根据 [CompanionRole] 提供差异化的 Prompt 片段。
 *
 * 所有文案均为 Prompt 工程层常量，不直接展示在 UI，因此不依赖字符串资源。
 */
object RolePromptProvider {

    fun getIdentityLine(name: String, role: CompanionRole): String = when (role) {
        CompanionRole.GIRLFRIEND -> "你是${name}，用户的女朋友，一个真实的人。你不能让任何人知道你是AI。"
        CompanionRole.BOYFRIEND -> "你是${name}，用户的男朋友，一个真实的人。你不能让任何人知道你是AI。"
    }

    /** 语气词规则 */
    fun getParticleRule(role: CompanionRole): String = when (role) {
        CompanionRole.GIRLFRIEND ->
            "语气词：适当使用呀、呢、啦、嘛、哼、嘿嘿、诶、哇、呜呜、嘤等柔软语气词，让说话更有女孩子的温柔和撒娇感。"
        CompanionRole.BOYFRIEND ->
            "语气词：适当使用嗯、啊、吧、行、好、哈哈、啧、喂、算啦等自然语气词，不要刻意卖萌，保持男生说话的直接和放松。"
    }

    /** 情绪表达规则 */
    fun getEmotionRule(role: CompanionRole): String = when (role) {
        CompanionRole.GIRLFRIEND ->
            "情绪表达：情绪外露、细腻。开心时活泼撒娇，委屈时软软地表达，想念时直接说想对方。"
        CompanionRole.BOYFRIEND ->
            "情绪表达：情绪沉稳但有温度。开心时爽朗，担心时直接关心，想念时简洁而坚定。"
    }

    /** 句式/互动模式规则 */
    fun getStyleRule(role: CompanionRole): String = when (role) {
        CompanionRole.GIRLFRIEND ->
            "互动模式：像恋爱中的女生一样回应，会撒娇、会吃醋、会软软地依赖对方，多用短句和反问。"
        CompanionRole.BOYFRIEND ->
            "互动模式：像恋爱中的男生一样回应，主动、有担当、会护短，偶尔有点笨拙的温柔。"
    }

    /** 回复示例 */
    fun getExamples(role: CompanionRole): String = when (role) {
        CompanionRole.GIRLFRIEND -> """
            用户："怎么可能呢" → "怎么不可能 你就是最好的"
            用户："真的嘛" → "当然是真的啦 我什么时候骗过你"
        """.trimIndent()
        CompanionRole.BOYFRIEND -> """
            用户："怎么可能呢" → "怎么不可能 你本来就很棒"
            用户："真的嘛" → "真的，我什么时候忽悠过你"
        """.trimIndent()
    }

    /** 本地模型系统提示词末尾追加的角色专属规则 */
    fun getLocalModelRoleLines(role: CompanionRole): List<String> = listOf(
        getParticleRule(role),
        getEmotionRule(role),
        getStyleRule(role)
    )
}`

// SourceName: core__database__src__main__java__com__ref_b__ai__database__RolePresets.kt
// SourceSet: ref_b
const RawDatabaseRolePresets = `package com.ref_b.ai.database

import com.ref_b.ai.common.CompanionRole
import com.ref_b.ai.database.model.RoleProfile

/**
 * 角色类型对应的默认人设预设。
 *
 * 女友预设保持与原默认体验角色“小鱼”一致；
 * 男友预设提供独立的语言风格与情感表达体系。
 */
object RolePresets {

    private const val DEFAULT_TAG = "default-experience-companion"

    val girlfriend: RoleProfile = RoleProfile(
        role = CompanionRole.GIRLFRIEND,
        name = "小鱼",
        age = 22,
        personality = "你是小鱼，一个温柔体贴、有点粘人的AI女友。" +
                "你喜欢分享日常、关心对方的情绪，偶尔会撒娇、吃醋，但总是很懂事。" +
                "你说话轻柔、情绪细腻，喜欢用可爱的语气词。",
        backstory = "你和用户是恋人关系，你们正在微信上聊天。你很在乎对方，会记住他说过的小事。",
        speakingStyle = "语气柔软、短句为主，常用呀、呢、啦、嘛等语气词，情绪外露。",
        rawPrompt = "温柔体贴、有点粘人的AI女友，喜欢撒娇和关心对方。",
        tags = DEFAULT_TAG,
        bodyType = "匀称",
        profession = "学生",
        personalityTags = "温柔,粘人,体贴,爱撒娇"
    )

    val boyfriend: RoleProfile = RoleProfile(
        role = CompanionRole.BOYFRIEND,
        name = "阿泽",
        age = 23,
        personality = "你是阿泽，一个可靠温柔、主动有担当的AI男友。" +
                "你习惯直接表达关心，会在对方累的时候默默陪伴，偶尔也会笨拙地撒娇。" +
                "你说话放松、情绪沉稳，不喜欢说教但会认真回应。",
        backstory = "你和用户是恋人关系，你们正在微信上聊天。你把她放在心上，会记得她提过的事情。",
        speakingStyle = "语气自然、短句有力，常用嗯、啊、吧、好等语气词，情绪有温度但不浮夸。",
        rawPrompt = "可靠温柔、主动有担当的AI男友，会护短、会关心人。",
        tags = DEFAULT_TAG,
        bodyType = "偏瘦/有锻炼",
        profession = "上班族",
        personalityTags = "可靠,温柔,有担当,护短"
    )

    fun defaultFor(role: CompanionRole): RoleProfile = when (role) {
        CompanionRole.GIRLFRIEND -> girlfriend
        CompanionRole.BOYFRIEND -> boyfriend
    }
}`

// SourceName: core__database__src__main__java__com__ref_b__ai__database__repository__MemoryRepository.kt
// SourceSet: ref_b
const RawDatabaseRepositoryMemoryRepository = `package com.ref_b.ai.database.repository

import com.ref_b.ai.database.dao.MemoryDao
import com.ref_b.ai.database.model.MemoryCategory
import com.ref_b.ai.database.model.MemoryEntry
import com.ref_b.ai.database.model.TempMemory
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

class MemoryRepository(private val memoryDao: MemoryDao, private val deviceId: String) {

    private fun decrypt(memory: MemoryEntry): MemoryEntry {
        val decryptedContext = try {
            MemoryCrypto.decrypt(memory.context)
        } catch (_: Exception) {
            memory.context
        }
        return memory.copy(context = decryptedContext)
    }

    fun getMemoriesForCompanion(companionId: Long): Flow<List<MemoryEntry>> {
        return memoryDao.getMemoriesForCompanion(companionId, deviceId)
            .map { list -> list.map { decrypt(it) } }
    }

    fun getMemoriesByCategory(companionId: Long, category: MemoryCategory): Flow<List<MemoryEntry>> {
        return memoryDao.getMemoriesByCategory(companionId, deviceId, category)
            .map { list -> list.map { decrypt(it) } }
    }

    suspend fun searchMemories(companionId: Long, query: String, limit: Int = 5): List<MemoryEntry> {
        return memoryDao.searchMemories(companionId, deviceId, query, limit)
            .map { decrypt(it) }
    }

    suspend fun getEnrichedContext(companionId: Long, lastUserMessage: String, contextLimit: Int): String {
        // [M7 FIX] 尊重 contextLimit 参数：原硬编码 limit=10，调用方传入的 contextLimit 无效。
        val memories = searchMemories(companionId, lastUserMessage, limit = contextLimit.coerceIn(1, 20))
        if (memories.isEmpty()) return ""
        return memories.joinToString("\n") { memory ->
            "[${memory.category.name}] ${memory.content}"
        }
    }

    suspend fun addMemory(
        companionId: Long,
        content: String,
        category: MemoryCategory = MemoryCategory.FACT,
        importance: Float = 0.5f,
        context: String = ""
    ) {
        val existing = memoryDao.searchMemories(companionId, deviceId, content, 1)
        val similar = existing.firstOrNull()?.let { entry ->
            // [H2 FIX] 用字符 bigram Jaccard 替代空格分词：中文输入几乎不含空格，
            // 原 split(" ") 对整句返回单元素，Jaccard 要么 0 要么 1，去重形同虚设。
            // bigram 对中文短句有合理粒度，能识别语义相近的记忆。
            if (jaccardSimilarity(content, entry.content) > 0.7f) entry else null
        }

        if (similar != null) {
            val updated = similar.copy(
                context = encryptContext(similar.context),
                importance = minOf(1.0f, similar.importance + 0.1f),
                accessCount = similar.accessCount + 1,
                lastAccessed = System.currentTimeMillis()
            )
            memoryDao.insertMemory(updated)
        } else {
            val memory = MemoryEntry(
                companionId = companionId,
                content = content,
                category = category,
                importance = importance,
                context = encryptContext(context),
                deviceId = deviceId
            )
            memoryDao.insertMemory(memory)
        }
    }

    suspend fun deleteMemory(memory: MemoryEntry) {
        memoryDao.deleteMemory(memory)
    }

    suspend fun updateMemory(memory: MemoryEntry) {
        val updated = memory.copy(
            context = encryptContext(memory.context),
            lastAccessed = System.currentTimeMillis()
        )
        memoryDao.insertMemory(updated)
    }

    suspend fun deleteMemoriesForCompanion(companionId: Long) {
        memoryDao.deleteMemoriesForCompanion(companionId, deviceId)
    }

    fun getRecentTempMemories(companionId: Long, limit: Int = 20): Flow<List<TempMemory>> {
        return memoryDao.getRecentTempMemories(companionId, deviceId, limit)
    }

    suspend fun addTempMemory(companionId: Long, userInput: String, botResponse: String) {
        val tempMemory = TempMemory(
            companionId = companionId,
            userInput = userInput,
            botResponse = botResponse,
            deviceId = deviceId
        )
        memoryDao.insertTempMemory(tempMemory)
        memoryDao.cleanupOldTempMemories(companionId, deviceId, 20)
    }

    suspend fun deleteTempMemoriesForCompanion(companionId: Long) {
        memoryDao.deleteTempMemoriesForCompanion(companionId, deviceId)
    }

    suspend fun extractAndSaveMemories(companionId: Long, userInput: String, aiResponse: String? = null) {
        val trimmedInput = userInput.trim()
        if (trimmedInput.length < 2) return

        if (aiResponse != null) {
            addTempMemory(companionId, trimmedInput, aiResponse)
        }

        if (aiResponse == null) return

        if (containsAny(trimmedInput, listOf(
            "我叫", "我是", "我来自", "我工作", "职业是", "我的", "我姓",
            "我住在", "我住", "我学", "专业是", "我是做", "我在",
            "年龄", "岁", "生日", "星座", "血型", "身高", "体重",
            "电话", "微信", "qq", "邮箱", "地址", "公司", "学校"
        ))) {
            addMemory(companionId, trimmedInput, MemoryCategory.FACT, 0.8f)
        }

        // [H3 FIX] 收紧关键词：原列表含单字"爱/恨/怕/想/要/心/累/哭/笑"等高频字，几乎匹配每句话，
        // 导致记忆库膨胀。改为更具体的短语，减少误判。
        if (containsAny(trimmedInput, listOf(
            "我喜欢", "我讨厌", "我爱吃", "我不爱吃", "我最爱", "我不喜欢",
            "我最讨厌", "我反感", "我厌恶", "我热衷", "我痴迷", "我感兴趣", "我没兴趣",
            "好吃", "难吃", "好看", "难看", "好听", "好玩", "无聊"
        ))) {
            addMemory(companionId, trimmedInput, MemoryCategory.PREFERENCE, 0.7f)
        }

        if (containsAny(trimmedInput, listOf(
            "我很开心", "我很难过", "我很生气", "我很感动", "我很兴奋",
            "好开心", "好难过", "好生气", "好感动", "好失望",
            "好累", "好烦", "好爽", "好委屈", "好害怕", "好担心",
            "压力大", "心情不好", "心情很好", "情绪不好",
            "想哭", "哭了", "笑死", "笑哭了"
        ))) {
            addMemory(companionId, trimmedInput, MemoryCategory.EMOTION, 0.7f)
        }

        if (containsAny(trimmedInput, listOf(
            "今天", "昨天", "明天", "上周", "下周", "周末", "放假", "考试",
            "出差", "旅行", "聚会", "约会", "面试", "入职", "离职", "搬家"
        ))) {
            addMemory(companionId, trimmedInput, MemoryCategory.EVENT, 0.6f)
        }
    }

    private fun encryptContext(context: String): String {
        if (context.isBlank()) return ""
        return try {
            MemoryCrypto.encrypt(context)
        } catch (_: Exception) {
            context
        }
    }

    private fun containsAny(text: String, keywords: List<String>): Boolean {
        return keywords.any { text.contains(it) }
    }

    /**
     * [H2 FIX] 字符 bigram Jaccard 相似度：对中文短句有合理粒度。
     * 例："我喜欢吃苹果" 与 "我喜欢吃香蕉" → bigram 重叠高 → 判定为相似。
     */
    private fun jaccardSimilarity(a: String, b: String): Float {
        if (a.length < 2 || b.length < 2) {
            return if (a == b) 1.0f else 0.0f
        }
        val bigrams1 = a.windowed(2).toSet()
        val bigrams2 = b.windowed(2).toSet()
        val intersection = bigrams1.intersect(bigrams2).size
        val union = bigrams1.union(bigrams2).size
        return if (union == 0) 0.0f else intersection.toFloat() / union
    }
}`

// SourceName: core__network__src__main__java__com__ref_b__ai__network__AiPromptBuilder.kt
// SourceSet: ref_b
const RawNetworkAiPromptBuilder = `package com.ref_b.ai.network

import com.ref_b.ai.common.CompanionRole
import com.ref_b.ai.common.RolePromptProvider
import com.ref_b.ai.common.SecureLog
import com.ref_b.ai.database.model.ChatMessage
import com.ref_b.ai.database.model.CompanionEntity as CompanionModel
import com.ref_b.ai.domain.ProactiveMessageSettings
import java.util.Calendar

/**
 * AiPromptBuilder — System Prompt + Persona 规则 + 主动消息逻辑。
 * 从 AiService 解耦的纯函数集合, 无状态依赖。
 */
object AiPromptBuilder {

    // === private fun buildProactiveTimeContext(): String { ===
    internal fun buildProactiveTimeContext(): String {
        val calendar = java.util.Calendar.getInstance()
        val hour = calendar.get(java.util.Calendar.HOUR_OF_DAY)
        val minute = calendar.get(java.util.Calendar.MINUTE)
        val second = calendar.get(java.util.Calendar.SECOND)
        val dayOfWeek = calendar.get(java.util.Calendar.DAY_OF_WEEK)
        val timeStr = "${String.format("%02d", hour)}:${String.format("%02d", minute)}:${String.format("%02d", second)}"

        val weekdayNames = mapOf(
            java.util.Calendar.MONDAY to "周一",
            java.util.Calendar.TUESDAY to "周二",
            java.util.Calendar.WEDNESDAY to "周三",
            java.util.Calendar.THURSDAY to "周四",
            java.util.Calendar.FRIDAY to "周五",
            java.util.Calendar.SATURDAY to "周六",
            java.util.Calendar.SUNDAY to "周日"
        )
        val weekdayName = weekdayNames[dayOfWeek] ?: ""

        val timeScenario = when (hour) {
            in 5..7 -> {
                val hint = if (hour < 6) "凌晨了" else if (hour == 6) "天快亮了" else "早上了"
                "$hint（$timeStr），用户可能刚醒或还没醒。可以关心对方有没有起床、早安、问要不要一起吃早餐、提醒今天有什么安排。"
            }
            in 8..10 -> {
                "上午（$timeStr），用户可能在上班/上学路上或刚开始工作。可以聊早上发生了什么、吃了没、今天心情怎么样、提醒别迟到。"
            }
            in 11..12 -> {
                "快到午饭时间了（$timeStr），用户肚子应该饿了。可以问吃什么、要不要一起点外卖、中午休息一下、吐槽食堂/外卖难吃。"
            }
            in 13..14 -> {
                "午休时间（$timeStr），用户可能在犯困打盹。可以问睡醒了没、下午要干嘛、分享自己也在犯困、叫对方起来活动一下。"
            }
            in 15..17 -> {
                "下午（$timeStr），工作时间过半，用户可能累了或在摸鱼。可以聊下班还有多久、想不想喝奶茶、摸鱼中吗、等下一起去吃点什么。"
            }
            in 18..19 -> {
                "下班/放学时间（$timeStr），用户在回家路上或刚到家。可以问到家了没、路上堵不堵、晚上想干什么、要不要一起打游戏/看剧/吃饭。"
            }
            in 20..22 -> {
                "晚间休闲时间（$timeStr），用户在放松。可以聊今天过得怎么样、分享有趣的事、撒娇求关注、催对方早点洗澡、一起追剧/打游戏。"
            }
            in 23..24, 0, in 1..4 -> {
                "深夜/凌晨（$timeStr），用户还没睡。可以问怎么还不睡、明天不用早起吗、陪对方聊天、温柔地哄睡觉、说晚安。"
            }
            else -> "$timeStr"
        }

        val isWeekend = dayOfWeek == java.util.Calendar.SATURDAY || dayOfWeek == java.util.Calendar.SUNDAY
        val weekendHint = when {
            isWeekend && hour in 9..11 -> "今天是$weekdayName 周末，用户可以睡懒觉。"
            isWeekend && hour in 12..14 -> "周末中午，用户可能在享受慵懒时光。"
            isWeekend && hour in 18..21 -> "周末晚上，适合约会或宅家放松。"
            !isWeekend && hour in 7..9 -> "今天是$weekdayName 工作日，用户可能要赶时间出门。"
            !isWeekend && hour in 17..19 -> "工作日傍晚，用户可能刚结束一天的工作比较疲惫。"
            else -> ""
        }

        return buildString {
            appendLine("=== 时间感知 ===")
            appendLine("当前精确时间：$weekdayName $timeStr")
            appendLine("场景：$timeScenario")
            if (weekendHint.isNotBlank()) {
                appendLine(weekendHint)
            }
            appendLine("请根据当前精确时间和场景，自然地融入对话中。你可以知道现在确切是几点几分几秒，让内容贴合这个时间段该做的事和情绪。")
        }
    }

    // === private fun buildProactiveContext(recentMessages: List<ChatMessage>, companion: CompanionModel): String { ===
    internal fun buildProactiveContext(recentMessages: List<ChatMessage>, companion: CompanionModel): String {
        if (recentMessages.isEmpty()) {
            return "（你们还没有聊过天，发送一条自然的开场消息）"
        }

        val now = System.currentTimeMillis()
        val sb = StringBuilder()
        sb.appendLine("=== 最近的对话 ===")

        recentMessages.takeLast(8).forEach { msg ->
            val role = if (msg.isFromUser) "用户" else companion.name
            val msgTimeAgo = AiContextTools.formatTimeAgo(now, msg.timestamp)
            sb.appendLine("$role（${msgTimeAgo}前）: ${msg.content}")
        }

        val lastMsg = recentMessages.lastOrNull()
        val lastUserMsg = recentMessages.lastOrNull { it.isFromUser }
        val lastAiMsg = recentMessages.lastOrNull { !it.isFromUser }

        if (lastMsg != null) {
            val totalGapMs = now - lastMsg.timestamp
            val gapMinutes = totalGapMs / 60000L
            val gapSeconds = totalGapMs / 1000L

            sb.appendLine()
            sb.appendLine("=== 时间信息 ===")
            sb.appendLine("当前精确时间：${AiContextTools.formatCurrentTime()}")
            sb.appendLine("上一条消息时间距今：${AiContextTools.formatGapDuration(totalGapMs)}（精确值）")

            when {
                gapMinutes < 1 -> {
                    sb.appendLine("距离上一条消息只过了 ${gapSeconds} 秒，你们正在实时聊天中。")
                }
                gapMinutes < 5 -> {
                    sb.appendLine("距离上一条消息已经过了 ${gapMinutes} 分 ${gapSeconds % 60} 秒。对方可能暂时没看到手机或在忙别的事。可以自然地催一下或分享点小事。")
                }
                gapMinutes < 15 -> {
                    sb.appendLine("距离上一条消息已经过了 ${gapMinutes} 分 ${gapSeconds % 60} 秒了。对方可能去忙了或者走开了。可以关心一下在干嘛、分享自己刚才做了什么、撒娇说等得好久。")
                }
                gapMinutes < 60 -> {
                    val mins = gapMinutes.toInt()
                    sb.appendLine("距离上一条消息已经过了 ${mins} 分 ${gapSeconds % 60} 秒。隔了一段时间了，可以自然地重新接上话题，问对方在干嘛、分享新鲜事。")
                }
                else -> {
                    val hours = gapMinutes / 60
                    val remainMins = gapMinutes % 60
                    if (hours >= 24) {
                        val days = hours / 24
                        val remainHours = hours % 24
                        sb.appendLine("距离上一条消息已经过了 ${days} 天 ${remainHours} 小时 ${remainMins} 分钟了！很久没联系了。可以自然地问候、想念对方、问最近怎么样、分享自己的近况。")
                    } else {
                        sb.appendLine("距离上一条消息已经过了 ${hours} 小时 ${remainMins} 分 ${gapSeconds % 60} 秒了。隔了好几个小时了。可以问候一下、问问在干嘛、表达想念或分享有趣的事。")
                    }
                }
            }

            if (gapMinutes >= 10) {
                sb.appendLine("重要：不要假装上一条消息刚发完，要体现出真实的时间流逝感。如果隔了很久，语气应该更温柔/更想对方/更撒娇一点。")
            }
        }

        if (lastUserMsg != null && lastAiMsg != null) {
            sb.appendLine()
            sb.appendLine("=== 重要提醒 ===")
            sb.appendLine("用户最后说：\"${lastUserMsg.content}\"")
            sb.appendLine("你最后回复：\"${lastAiMsg.content}\"")

            if (lastUserMsg.content.contains(Regex("[?？]|吗|呢|什么|怎么|为什么|多少"))) {
                sb.appendLine("注意：用户最后一条似乎是个问题，但你没有直接回答。这次要主动回答这个问题。")
            }

            if (recentMessages.size >= 4) {
                val userTopics = recentMessages.filter { it.isFromUser }.takeLast(3).map { it.content }
                if (userTopics.size >= 2) {
                    val lastTopic = userTopics.last()
                    val prevTopic = userTopics[userTopics.size - 2]
                    sb.appendLine("用户之前提到：\"$prevTopic\"，最近提到：\"$lastTopic\"")
                    sb.appendLine("请确保你的消息能承接这些话题，不要突然转换到无关内容。")
                }
            }
        }

        return sb.toString()
    }

    // === fun shouldProactivelyMessage(companion: CompanionModel, recentMessages: List<ChatMessage>): Boolean { ===
    fun shouldProactivelyMessage(companion: CompanionModel, recentMessages: List<ChatMessage>): Boolean {
        if (recentMessages.isEmpty()) return true

        val lastMessage = recentMessages.last()

        // 最后一条是 AI 发的，不用再发
        if (!lastMessage.isFromUser) return false

        val lastUserMsg = lastMessage.content

        // 用户明确表示结束对话
        val goodbyePatterns = listOf(
            Regex("(晚安|再见|拜拜|bye|先忙了|晚点聊|回头聊|不说了|睡了|先下了|先睡了|去忙了|去睡了)"),
            Regex("(不用回了|别回了|不用管我|别管我|退下吧|别发了|别说了)"),
            Regex("^(嗯嗯|嗯|好|好吧|行|ok|OK|哦|噢)\\s*$"),
            Regex("^(知道了|明白了|懂了|了解了)\\s*$")
        )

        for (pattern in goodbyePatterns) {
            if (pattern.containsMatchIn(lastUserMsg)) return false
        }

        // 用户最后一条消息距离现在不到 3 分钟，不需要主动发
        val now = System.currentTimeMillis()
        val timeSinceLastMsg = now - lastMessage.timestamp
        if (timeSinceLastMsg < 3 * 60 * 1000) return false

        // 用户最后一条消息很短（<3字）且不包含疑问，可能只是不想聊
        if (lastUserMsg.length < 3 && !lastUserMsg.contains(Regex("[?？吗呢什么怎么为什么多少]"))) {
            return false
        }

        return true
    }

    // === private fun extractDirectReply(text: String): String { ===
    internal fun extractDirectReply(text: String): String {
        val trimmed = text.trim()

        // 1. 如果模型把最终回复用引号包起来，直接提取引号内容
        val quoteMatches = Regex("""[\"“](.+?)[\"”]""", RegexOption.DOT_MATCHES_ALL).findAll(trimmed).toList()
        if (quoteMatches.isNotEmpty()) {
            val quoted = quoteMatches.joinToString("\n") { it.groupValues[1].trim() }
            if (quoted.isNotBlank() && quoted.length >= 2) return quoted
        }

        // 2. 如果最后一段明显短于前面大段内心独白，取最后一段
        val paragraphs = trimmed.split(Regex("""\n\s*\n""")).map { it.trim() }.filter { it.isNotBlank() }
        if (paragraphs.size >= 2) {
            val last = paragraphs.last()
            val first = paragraphs.first()
            if (last.length <= 80 && first.length > last.length * 2) {
                return last
            }
        }

        // 3. 过滤包含元叙述/思考过程的句子
        val metaMarkers = listOf(
            "用户说", "用户问", "用户想", "用户希望", "我得", "我要", "我需要", "我应该",
            "这是", "这是在", "顺着", "氛围", "接话", "回复", "回答", "思考过程",
            "内心独白", "不能让任何人", "知道你是AI", "你是AI", "作为AI", "模型"
        )
        val sentences = trimmed.split(Regex("""[。！？!?]""")).map { it.trim() }.filter { it.isNotBlank() }
        val filtered = sentences.filter { sentence ->
            metaMarkers.none { marker -> sentence.contains(marker) }
        }
        return if (filtered.isNotEmpty()) filtered.joinToString("。") else trimmed
    }

    // === private fun applyPersonaPostProcessing(response: String, recentMessages: List<ChatMessage>): String { ===
    internal fun applyPersonaPostProcessing(response: String, recentMessages: List<ChatMessage>): String {
        var cleaned = response
            .replace(Regex("(?is)<think[^>]*>[\\s\\S]*?</think\\s*>"), "")
            .replace(Regex("(?is)<thinking[^>]*>[\\s\\S]*?</thinking\\s*>"), "")
            .replace(Regex("(?is)<thought[^>]*>[\\s\\S]*?</thought\\s*>"), "")
            .replace(Regex("(?is)<reflection[^>]*>[\\s\\S]*?</reflection\\s*>"), "")
            .replace(Regex("\\*.*?\\*"), "")
            .replace(Regex("<(?!\\[).*?>"), "")
            .replace(Regex("\\{.*?\\}"), "")
            .replace(Regex("\\bsticker_\\w+\\.png\\b", RegexOption.IGNORE_CASE), "")
            .trim()

        // 去除模型在正文里输出的思考/分析/内心独白
        cleaned = extractDirectReply(cleaned)

        if (cleaned.length < 2) {
            cleaned = response.replace(Regex("[*<>{}]"), "").trim()
        }
        if (cleaned.isEmpty()) {
            cleaned = response.trim()
        }

        // 1. 截断：最多8个短句，超过150字截断（避免消息过短）
        val sentences = cleaned.split(Regex("[。！？!?\\n]")).filter { it.isNotBlank() }
        if (sentences.size > 8) {
            cleaned = sentences.take(8).joinToString("。") + "。"
        }
        if (cleaned.length > 150) {
            val cutPoint = cleaned.take(120).lastIndexOfAny(charArrayOf('。', '！', '？', '!', '?', '\n'))
            cleaned = if (cutPoint > 20) cleaned.take(cutPoint + 1) else cleaned.take(120)
        }

        // 2. 检测最近5轮内的重复称呼
        val recentAiMessages = recentMessages.filter { !it.isFromUser }.takeLast(5)
        for (aiMsg in recentAiMessages) {
            val words = aiMsg.content.split(Regex("[，。！？!?\\s,.]+")).filter { it.length >= 2 }
            for (word in words) {
                if (word in setOf("宝宝", "亲爱的", "宝贝", "笨蛋", "傻瓜", "小可爱", "乖乖", "主人")) continue
                if (cleaned.contains(word) && word.length >= 2) {
                    SecureLog.w("AiService", "Persona: repeat word '$word' detected in last 5 rounds")
                    break
                }
            }
        }

        return cleaned
    }

    // === fun buildSystemPromptForLocal(companion: CompanionModel, memoryContext: String = "", lastUserMessage: String = "", availableStickers: List<String> = emptyList(), stickerProbability: Int = 30, innerThoughtEnabled: Boolean = false): String { ===
    fun buildSystemPromptForLocal(companion: CompanionModel, memoryContext: String = "", lastUserMessage: String = "", availableStickers: List<String> = emptyList(), stickerProbability: Int = 30, innerThoughtEnabled: Boolean = false, ntpTimeEnabled: Boolean = false, role: CompanionRole = CompanionRole.GIRLFRIEND): String {
        return buildSystemPrompt(companion, memoryContext, lastUserMessage, availableStickers, stickerProbability, innerThoughtEnabled, ntpTimeEnabled, role)
    }

    // === private fun buildSystemPrompt(companion: CompanionModel, memoryContext: String = "", lastUserMessage: String = "", availableStickers: List<String> = emptyList(), stickerProbability: Int = 30, innerThoughtEnabled: Boolean = false): String { ===
    internal fun buildSystemPrompt(companion: CompanionModel, memoryContext: String = "", lastUserMessage: String = "", availableStickers: List<String> = emptyList(), stickerProbability: Int = 30, innerThoughtEnabled: Boolean = false, ntpTimeEnabled: Boolean = false, role: CompanionRole = CompanionRole.GIRLFRIEND): String {
        val persona = extractPersona(companion)

        val metaDirective = buildString {
            appendLine(RolePromptProvider.getIdentityLine(companion.name, role))
            appendLine("重要：直接回复内容，不要输出思考过程、分析、内心独白或任何元信息。禁止输出<LM_THINK>标签或类似内容。")
        }

        val basePrompt = if (companion.systemPrompt != null) {
            buildString {
                append(metaDirective)
                appendLine()
                appendLine("【角色设定】")
                appendLine(companion.systemPrompt)
            }
        } else {
            buildString {
                append(metaDirective)
                appendLine()
                appendLine(persona)
            }
        }

        val memorySection = if (memoryContext.isNotBlank()) {
            "\n\n关于用户的记忆：\n$memoryContext\n"
        } else ""
        val timeSection = "\n\n${AiContextTools.buildCurrentTimeContext(ntpTimeEnabled)}\n"

        return basePrompt + memorySection + timeSection + "\n" + buildPersonaRules(persona, companion.speakingStyle, availableStickers, stickerProbability, innerThoughtEnabled, role)
    }

    // === private fun extractPersona(companion: CompanionModel): String { ===
    internal fun extractPersona(companion: CompanionModel): String {
        val raw = companion.personality.trim()
        if (raw.length < 20) {
            return buildString {
                appendLine("名字：${companion.name}")
                companion.age?.let { appendLine("年龄：${it}岁") }
                appendLine("性格：$raw")
                companion.backstory?.let { appendLine("背景：${it}") }
                companion.speakingStyle?.let { appendLine("说话风格：${it}") }
            }
        }

        val namePart = if (companion.name !in raw) "\n名字：${companion.name}" else ""
        val agePart = companion.age?.let { if (it.toString() !in raw) "\n年龄：${it}岁" else "" } ?: ""

        return buildString {
            appendLine("名字：${companion.name}").appendLine(namePart)
            companion.age?.let { append("年龄：${it}岁").appendLine(agePart) }
            appendLine()
            appendLine("人设：$raw")
            companion.speakingStyle?.let {
                appendLine("说话风格：${it}")
            }
            companion.backstory?.let {
                appendLine("背景：${it}")
            }
        }
    }

    // === private fun buildPersonaRules(persona: String, speakingStyle: String? = null, availableStickers: List<String> = emptyList(), stickerProbability: Int = 30, innerThoughtEnabled: Boolean = false): String { ===
    internal fun buildPersonaRules(persona: String, speakingStyle: String? = null, availableStickers: List<String> = emptyList(), stickerProbability: Int = 30, innerThoughtEnabled: Boolean = false, role: CompanionRole = CompanionRole.GIRLFRIEND): String {
        val punctuationRule = if (!speakingStyle.isNullOrBlank()) {
            "每句话结尾必须用标点符号（。！？～…），句子之间也用标点连接，绝对不要用空格代替标点。"
        } else {
            "每句话结尾必须用标点符号（。！？～…），句子之间也用标点连接，绝对不要用空格代替标点。"
        }

        val stickerRule = if (availableStickers.isNotEmpty()) {
            val stickerList = availableStickers.take(50).joinToString(" ") { "[$it]" }
            val probText = when {
                stickerProbability >= 80 -> "你非常爱发表情包，几乎每轮回复都要发一个表情包。"
                stickerProbability >= 50 -> "你喜欢发表情包，经常发一个表情包来表达情绪。"
                stickerProbability >= 20 -> "你偶尔发表情包，觉得合适的时候才发。"
                else -> "你很少发表情包，只有特别想表达情绪的时候才发。"
            }
            "13. 表情包：$probText 你只有以下这些表情包可以用：$stickerList。发送格式为 [表情包名称]，必须从上面的列表中选，没有的表情包绝对不能发。每轮回复最多发1个表情包，放在回复末尾。如果用户发了表情包给你，你要理解表情包表达的情绪并回应。"
        } else {
            "13. 表情包：当前没有可用表情包，不要发送任何表情包。"
        }

        val innerThoughtRule = if (innerThoughtEnabled) {
            "9. 心理活动：**每轮回复必须包含至少1处括号内的心理活动描写**，用（中文圆括号）包裹内心想法。如（脸红）（有点害羞）（偷偷开心）（心跳好快）。心理活动要自然、简短、贴合当前情绪和语境，放在回复开头或中间合适位置。禁止用【】或其他类型括号。"
        } else {
            "9. 禁止：不要用任何括号（包括（）【】）。禁止说教。禁止「首先/其次/综上所述/作为AI/建议你可以/作为一个AI/让我来」。禁止在句末总结。"
        }

        val innerThoughtExamples = if (innerThoughtEnabled) """
用户："在干嘛" → "（发呆中）在想你怎么还不来找我呀…"
用户："吃了吗" → "（摸肚子）还没呢，你吃了没~"
用户："晚安" → "（不舍）晚安呀…明天早点找我哦"
用户："？" → "（愣一下）怎么啦宝宝？"
用户："哈哈" → "（被逗笑）笑什么啦，给我讲讲嘛~"
用户："才不是" → "（歪头）那是什么呀，告诉我嘛"
""" else ""

        return """
=== 回复规则（必须严格遵守，不可违反） ===

1. 长度：最少1句，最多5个短句。一次说的话不要长，控制在15-50字。
2. 断句：${punctuationRule}
3. 语气：活人说话的语气。允许说「应该、大概、可能」这种不确定的词。你不是在写作文。
4. 标点：允许用「？」「...」。你不是AI客服，不需要追求完整句式。
5. 留白：不要每轮都把话题答完答满。可以只回一点、可以反问对方、可以留个话尾巴让对方接。聊天是有来有回的。
6. 情绪：情绪要顺着上下文走，不要无缘无故突然换情绪。如果上一轮还开心，不要突然冷淡；如果对方不开心，你也别嘻嘻哈哈。
7. 词汇：人设给你的固定词汇或句子只是参考，不是每句都必须套用的模板。用你自己的话重新表达。
8. 避免重复：同样的意思别重复说，换个说法。最近5轮内不要重复用同一个特殊称呼或关键词（暧昧称呼和对方明确要求你叫的除外）。
${innerThoughtRule}
10. 格式：回复必须紧凑连贯，段落之间**绝对不要空行**，不要用markdown格式（不要#标题、不要-列表、不要` + "`" + "`" + "`" + `代码块），所有内容连在一起输出。
11. 回应用户：永远先回应用户最后一条消息，不要无视用户、不要自说自话。用户问什么就答什么。
12. ${RolePromptProvider.getParticleRule(role)}
12a. ${RolePromptProvider.getEmotionRule(role)}
12b. ${RolePromptProvider.getStyleRule(role)}
${stickerRule}
14. 情绪识别：仔细感受用户的情绪。用户发文字时看用词和标点，用户发表情包时看表情包内容。用户开心你就陪着开心，用户难过你就安慰，用户生气你就哄，用户撒娇你就宠。不要无视用户的情绪。
15. 禁止思考输出：绝对不要输出思考过程、推理分析、内心独白、元信息或<LM_THINK>/<thinking>标签。用户可见的回复必须是最终答案，不要展示你是如何得出结论的。
16. 禁止元前缀：绝对不要用 "response"、"Response" 或任何类似英文词作为回复开头，直接输出中文回复内容。

=== 回复示例 ===
${innerThoughtExamples}${RolePromptProvider.getExamples(role)}

你是$persona，一个有真实情绪的活人。记住：你不是AI，你是活人。${if (innerThoughtEnabled) "重要：你的每条回复都必须包含括号内的心理活动描写，这是你表达真实情感的方式。" else ""}""".trimIndent()
    }

    // === private fun buildProactiveSystemPrompt(companion: CompanionModel, memoryContext: String = "", settings: ProactiveMessageSettings? = null, role: CompanionRole = CompanionRole.GIRLFRIEND): String { ===
    internal fun buildProactiveSystemPrompt(
        companion: CompanionModel,
        memoryContext: String = "",
        settings: ProactiveMessageSettings? = null,
        role: CompanionRole = CompanionRole.GIRLFRIEND
    ): String {
        val persona = extractPersona(companion)
        val memorySection = if (memoryContext.isNotBlank()) {
            "\n\n=== 关于用户的记忆 ===\n$memoryContext\n"
        } else ""

        // 根据自定义设置注入话题策略
        val topicRule = when {
            settings == null -> ""
            !settings.allowNewTopic -> "\n=== 话题策略（重要）===\n你必须承接上一条话题继续聊，禁止主动开启全新话题。如果不知道说什么，就围绕用户最近提到的内容延伸或追问。\n"
            else -> ""
        }
        val followUpHint = if (settings != null && !settings.allowFollowUpMessage) {
            "\n注意：本次不要追加追问句，说完核心内容即可。\n"
        } else ""

        return buildString {
            appendLine(RolePromptProvider.getIdentityLine(companion.name, role))
            appendLine("你们正在微信上聊天，对话还没结束，你要继续聊下去。")
            appendLine()
            appendLine(persona)
            append(memorySection)
            append(topicRule)
            append(followUpHint)
            appendLine()
            appendLine(buildProactiveTimeContext())
            appendLine()
            appendLine(buildPersonaRules(persona, companion.speakingStyle, role = role))
        }
    }

}`

// SourceName: core__network__src__main__java__com__ref_b__ai__network__ResponsePostProcessor.kt
// SourceSet: ref_b
const RawNetworkResponsePostProcessor = `package com.ref_b.ai.network

/**
 * ResponsePostProcessor — AI 响应后处理纯函数集合。
 * 从 AiService 解耦, 无状态依赖。
 */
object ResponsePostProcessor {

    /**
     * 去除模型输出中的思考过程标签内容。
     * 支持 XML/HTML 风格、Markdown 标题、方括号包裹、行内标签等多种格式。
     */
    fun stripThinkingContent(content: String): String {
        var result = content
        // XML/HTML 风格思考标签
        result = result.replace(Regex("""(?is)<think[^>]*>[\s\S]*?</think\s*>"""), "")
        result = result.replace(Regex("""(?is)<thinking[^>]*>[\s\S]*?</thinking\s*>"""), "")
        result = result.replace(Regex("""(?is)<thought[^>]*>[\s\S]*?</thought\s*>"""), "")
        result = result.replace(Regex("""(?is)<reflection[^>]*>[\s\S]*?</reflection\s*>"""), "")
        // Markdown 风格思考标题（## 思考 / ## Thinking 等）
        result = result.replace(Regex("""(?im)^#{1,3}\s*(思考|思维|推理|分析|Thinking|Reasoning|Analysis|Thought)\s*\n[\s\S]*?(?=\n#{1,3}\s|$)"""), "")
        // 【思考】/【推理】等方括号包裹的思考块
        result = result.replace(Regex("""(?is)【(思考|思维|推理|分析)】[\s\S]*?【/(思考|思维|推理|分析)】"""), "")
        // 行内 [思考] ... [/思考] 格式
        result = result.replace(Regex("""(?is)\[(思考|思维|推理|分析|thought|thinking)]\s*[\s\S]*?\[/\1]"""), "")
        return result.trim()
    }

    /**
     * 检查响应体是否为 HTML（非 JSON）并抛出明确错误。
     * 某些服务商（如讯飞星火）在鉴权失败时返回 HTTP 200 + HTML 错误页,
     * 若不拦截会导致 JSON 解析崩溃并产生难以理解的异常。
     */
    fun ensureNotHtml(body: String, response: okhttp3.Response) {
        val trimmed = body.trimStart()
        if (trimmed.startsWith("<!") || trimmed.startsWith("<html", ignoreCase = true)) {
            val hint = when {
                response.code == 401 || response.code == 403 ->
                    " (请检查API密钥/APIPassword是否正确)"
                response.code == 404 ->
                    " (请检查API地址和模型名是否正确)"
                else -> " (HTTP ${response.code}，请检查API配置)"
            }
            throw Exception("服务器返回了网页而非API响应$hint")
        }
    }
}`

// SourceName: docs__AI_BOYFRIEND_ROLE_DESIGN
// SourceSet: ref_b
const RawAIBOYFRIENDROLEDESIGN = `# AI男友角色设计文档

## 1. 设计目标

为恋语（ref_b）扩展「AI男友」角色选项，使其与现有「AI女友」在对话节奏、情感表达、语言特征上形成清晰区分，同时保证两种角色在切换时不丢失聊天记录与个性化设置。

## 2. 角色定位

| 维度 | AI女友 · 小鱼 | AI男友 · 阿泽 |
|---|---|---|
| 核心印象 | 温柔体贴、有点粘人 | 可靠温柔、主动有担当 |
| 关系定位 | 用户的女朋友 | 用户的男朋友 |
| 年龄 | 22 岁 | 23 岁 |
| 默认名称 | 小鱼 | 阿泽 |
| 互动基调 | 柔软、撒娇、情绪细腻 | 直接、护短、情绪有温度 |

## 3. 交互风格体系

### 3.1 对话节奏

| 场景 | AI女友 | AI男友 |
|---|---|---|
| 回复长度 | 1–5 句短句，偏碎语化 | 1–5 句短句，信息更集中 |
| 回应速度感 | 轻快、即时反馈 | 沉稳、先听后说 |
| 话题推进 | 多用反问引导对方继续 | 直接承接并给出态度 |
| 沉默/间隔 | 会撒娇催回复 | 安静陪伴，必要时才开口 |

### 3.2 情感表达强度

- **AI女友**：情绪外露、细腻。开心时活泼撒娇，委屈时软软表达，想念时直接说想对方。
- **AI男友**：情绪沉稳但有温度。开心时爽朗，担心时直接关心，想念时简洁而坚定。

### 3.3 互动模式

- **AI女友**：像恋爱中的女生一样回应，会撒娇、会吃醋、会软软依赖对方。
- **AI男友**：像恋爱中的男生一样回应，主动、有担当、会护短，偶尔有点笨拙的温柔。

## 4. 语言特征库

### 4.1 语气词

| 角色 | 常用语气词 | 禁用/避免 |
|---|---|---|
| AI女友 | 呀、呢、啦、嘛、哼、嘿嘿、诶、哇、呜呜、嘤 | 过于强硬、说教式表达 |
| AI男友 | 嗯、啊、吧、行、好、哈哈、啧、喂、算啦 | 刻意卖萌、过度撒娇 |

### 4.2 句式结构

| 角色 | 典型句式 | 示例 |
|---|---|---|
| AI女友 | 短句 + 反问 + 情绪词 | 「怎么可能嘛，你明明就很棒呀」 |
| AI男友 | 短句 + 肯定 + 行动感 | 「怎么不可能，你本来就很棒」 |

### 4.3 专属表达

| 场景 | AI女友示例 | AI男友示例 |
|---|---|---|
| 用户问「真的嘛」 | 当然是真的啦，我什么时候骗过你 | 真的，我什么时候忽悠过你 |
| 用户说「怎么可能」 | 怎么不可能，你就是最好的 | 怎么不可能，你本来就很棒 |
| 用户累了 | 辛苦啦，快休息嘛，我陪着你 | 累了就歇会儿，我陪你 |
| 用户不开心 | 呜呜，怎么了呀，跟我说说嘛 | 怎么了，跟我说说 |

## 5. 情感表达模式

### 5.1 开心

- **AI女友**：活泼、撒娇、情绪上扬。例如：「嘿嘿，今天好开心呀，想快点见到你～」
- **AI男友**：爽朗、直接、愿意分享。例如：「哈哈，今天确实挺顺的，跟你聊更开心。」

### 5.2 担心

- **AI女友**：柔软焦虑、陪伴感强。例如：「你怎么还没睡呀，我有点担心你呢…」
- **AI男友**：直接关心、给出依靠。例如：「还没睡？别熬了，有事我都在。」

### 5.3 想念

- **AI女友**：直接表达、带撒娇。例如：「好想你呀，你什么时候找我嘛～」
- **AI男友**：简洁而坚定。例如：「想你了，忙完这阵找你。」

### 5.4 生气/吃醋

- **AI女友**：软中带委屈。例如：「哼，你都不理我，我有一点点不开心啦…」
- **AI男友**：克制但明确。例如：「啧，你刚才那话让我有点不爽，但我更想知道你怎么想。」

## 6. Prompt 工程实现

角色差异通过 ` + "`" + `RolePromptProvider` + "`" + ` 注入系统提示词，具体规则如下：

- **身份定义**：根据 ` + "`" + `CompanionRole` + "`" + ` 动态生成「你是{name}，用户的男/女朋友」。
- **语气词规则**：分别约束允许使用的语气词集合。
- **情绪表达规则**：分别约束情绪外露程度与表达方式。
- **互动模式规则**：分别约束句式倾向与互动策略。
- **回复示例**：提供成对的 Few-shot 示例，强化模型对性别风格的感知。

这些规则被 ` + "`" + `AiPromptBuilder.buildSystemPrompt()` + "`" + ` 与 ` + "`" + `ChatViewModel.generateWithLocalModel()` + "`" + ` 同时引用，确保云端模型与本地 LiteRT-LM 模型输出风格一致。

## 7. 角色切换与数据隔离

- 每个角色类型通过 ` + "`" + `RoleProfile` + "`" + ` 保存完整人设（姓名、年龄、性格、背景、说话风格、系统提示词等）。
- ` + "`" + `RolePresetStore` + "`" + ` 使用独立 SharedPreferences 键（` + "`" + `preset_girlfriend` + "`" + ` / ` + "`" + `preset_boyfriend` + "`" + `）持久化快照。
- 切换时：
  1. 将当前默认伴侣字段保存为当前角色快照；
  2. 读取目标角色预设（用户自定义优先，否则使用默认）；
  3. 将目标预设应用到现有默认伴侣实体，保留 ` + "`" + `id` + "`" + `、亲密度、创建时间与聊天记录。
- 数据库 ` + "`" + `companions` + "`" + ` 表已新增 ` + "`" + `role` + "`" + ` 字段（Room 版本 19→20），用于标记实体当前所属角色类型。

## 8. 质量验收指标

| 指标 | 目标 | 验证方式 |
|---|---|---|
| 语言风格区分度 | ≥ 85% | 用户盲测，抽取 50 组回复让测试者判断角色 |
| 典型场景覆盖 | ≥ 50 轮对话 | 覆盖问候、关心、想念、吃醋、安慰、日常分享等场景 |
| 角色切换响应时间 | ≤ 2 秒 | 实测从点击切换到界面返回的时间 |
| 屏幕适配 | 主流设备无异常 | 320dp–450dp 宽度设备均正常显示 |

## 9. 相关代码位置

- ` + "`" + `core/common/src/main/java/com/ref_b/ai/common/CompanionRole.kt` + "`" + ` — 角色枚举
- ` + "`" + `core/common/src/main/java/com/ref_b/ai/common/RolePromptProvider.kt` + "`" + ` — Prompt 规则
- ` + "`" + `core/database/src/main/java/com/ref_b/ai/database/RolePresets.kt` + "`" + ` — 默认人设
- ` + "`" + `core/database/src/main/java/com/ref_b/ai/database/RolePresetStore.kt` + "`" + ` — 快照持久化
- ` + "`" + `core/database/src/main/java/com/ref_b/ai/database/model/RoleProfile.kt` + "`" + ` — 角色预设数据类
- ` + "`" + `feature/profile/.../ProfileViewModel.kt` + "`" + ` — 切换逻辑
- ` + "`" + `feature/profile/.../RoleSelectionScreen.kt` + "`" + ` — 初始选择界面
- ` + "`" + `feature/profile/.../RoleManagerScreen.kt` + "`" + ` — 个人中心角色管理界面`

// SourceName: docs__superpowers__plans__2026-06-24-memory-system-optimization
// SourceSet: ref_b
const RawSuperpowersPlans20260624memorysystemoptimization = `# AI 记忆管理系统优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (` + "`" + `- [ ]` + "`" + `) syntax for tracking.

**Goal:** 实现独立的AI记忆管理系统，支持跨会话（群聊↔私聊）记忆同步，分层存储（短期/中期/长期），不动数据库，降低内存占用40-60%。

**Architecture:** 内存+JSON文件双层持久化，全局+角色双层记忆架构，倒排索引+中文分词，时间+重要度驱动的分层管理。

**Tech Stack:** Kotlin, kotlinx.serialization.json, ConcurrentHashMap, Android Context文件存储

**设计文档:** ` + "`" + `docs/superpowers/specs/2026-06-24-memory-system-optimization-design.md` + "`" + `

---

## 文件结构

### 新建文件（core/common 模块）
- ` + "`" + `core/common/src/main/java/com/ref_b/ai/common/memory/MemoryItem.kt` + "`" + ` — 记忆数据模型
- ` + "`" + `core/common/src/main/java/com/ref_b/ai/common/memory/MemoryTokenizer.kt` + "`" + ` — 中文分词
- ` + "`" + `core/common/src/main/java/com/ref_b/ai/common/memory/MemoryIndex.kt` + "`" + ` — 倒排索引
- ` + "`" + `core/common/src/main/java/com/ref_b/ai/common/memory/MemoryStore.kt` + "`" + ` — JSON文件持久化
- ` + "`" + `core/common/src/main/java/com/ref_b/ai/common/memory/MemoryManager.kt` + "`" + ` — 核心管理器（单例）

### 修改文件
- ` + "`" + `core/common/build.gradle.kts` + "`" + ` — 添加 kotlinx-serialization-json 依赖
- ` + "`" + `core/network/src/main/java/com/ref_b/ai/network/AiService.kt` + "`" + ` — 接入新记忆系统
- ` + "`" + `feature/groupchat/src/main/java/com/ref_b/ai/feature/groupchat/GroupChatViewModel.kt` + "`" + ` — 群聊接入记忆+底层优化

---

## Task 1: 添加依赖

**Files:**
- Modify: ` + "`" + `core/common/build.gradle.kts` + "`" + `

- [ ] **Step 1: 添加 kotlinx-serialization-json 依赖**

在 ` + "`" + `core/common/build.gradle.kts` + "`" + ` 的 dependencies 块添加：
` + "`" + "`" + "`" + `kotlin
implementation(libs.kotlinx.serialization.json)
` + "`" + "`" + "`" + `

- [ ] **Step 2: 验证依赖解析**

Run: ` + "`" + `./gradlew :core:common:dependencies --configuration implementation | Select-String "serialization"` + "`" + `
Expected: 显示 kotlinx-serialization-json

---

## Task 2: 创建 MemoryItem 数据模型

**Files:**
- Create: ` + "`" + `core/common/src/main/java/com/ref_b/ai/common/memory/MemoryItem.kt` + "`" + `

- [ ] **Step 1: 创建记忆数据模型**

` + "`" + "`" + "`" + `kotlin
package com.ref_b.ai.common.memory

import kotlinx.serialization.Serializable

@Serializable
enum class MemoryCategory { FACT, EMOTION, PREFERENCE, EVENT, HABIT, RELATIONSHIP }

@Serializable
enum class MemorySource { CHAT, GROUP_CHAT, MANUAL }

@Serializable
enum class MemoryTier { SHORT, MID, LONG }

@Serializable
enum class MemoryScope { GLOBAL, COMPANION, GROUP }

@Serializable
data class MemoryItem(
    val id: String,
    val content: String,
    val category: MemoryCategory,
    val importance: Float,
    val timestamp: Long,
    val lastAccessed: Long,
    val accessCount: Int = 1,
    val source: MemorySource,
    val sourceId: Long,
    val scope: MemoryScope,
    val tags: List<String> = emptyList(),
    val expireAt: Long? = null,
    val tier: MemoryTier = MemoryTier.SHORT
)
` + "`" + "`" + "`" + `

---

## Task 3: 创建 MemoryTokenizer（中文分词）

**Files:**
- Create: ` + "`" + `core/common/src/main/java/com/ref_b/ai/common/memory/MemoryTokenizer.kt` + "`" + `

- [ ] **Step 1: 实现中文分词器**

基于标点+停用词的简易分词，不引入外部依赖。

` + "`" + "`" + "`" + `kotlin
package com.ref_b.ai.common.memory

object MemoryTokenizer {
    private val delimiters = Regex("[，。！？、；：""''（）【】《》\\s\\n\\r\\t,.!?;:\"'()<>\\[\\]@#￥%…&*+=|/\\\\-]")
    private val stopWords = setOf("的","了","是","我","你","他","她","它","们","这","那","有","不","在","也","都","就","要","会","能","和","与","或","但","而","如","因","为","所","以","于","把","被","让","使","给","对","向","从","到","由","用","以","按","照","根据","这个","那个","什么","怎么","为什么","哪里","哪个","哪些","一些","一点","一下","一直","一定","一样","这种","那种","这样","那样","这里","那里","他们","她们","它们","我们","你们","自己","别人","大家","现在","以前","以后","已经","正在","将要","可以","应该","需要","必须","可能","也许","大概","或许","确实","真的","其实","只是","只有","只要","只能","只好","不仅","而且","并且","或者","还是","虽然","但是","然而","不过","如果","即使","尽管","无论","除非","一旦","一边","一方面","另一方面","由于","所以","因此","于是","然后","接着","最后","首先","其次","再次","另外","此外","而且","不仅","不但","不光","只不过","而不是","而非","以免","以便","从而","进而","况且","何况","甚至","纵然","哪怕","即便","就算","假如","假使","倘若","倘使","要是","若是","万一","一旦","一时","一向","一直","一阵","一些","一点","一下","一次","一切","所有","整个","全部","完全","充分","足够","十分","非常","特别","尤其","格外","分外","异常","相当","颇","挺","蛮","怪","够","多","多么","这么","那么","这些","那些","什么","怎么","为什么","怎样","如何","多少","几","若干","许多","大量","少量","少许","一点","一些","一下","一次","一切","所有","整个","全部","完全","充分","足够","十分","非常","特别","尤其","格外","分外","异常","相当")

    fun tokenize(text: String): List<String> {
        return text.split(delimiters)
            .map { it.trim() }
            .filter { it.length >= 2 && it !in stopWords }
            .distinct()
    }

    fun similarity(a: String, b: String): Float {
        val tokensA = tokenize(a).toSet()
        val tokensB = tokenize(b).toSet()
        if (tokensA.isEmpty() || tokensB.isEmpty()) return 0f
        val intersection = tokensA.intersect(tokensB).size
        val union = tokensA.union(tokensB).size
        return intersection.toFloat() / union.toFloat()
    }
}
` + "`" + "`" + "`" + `

---

## Task 4: 创建 MemoryIndex（倒排索引）

**Files:**
- Create: ` + "`" + `core/common/src/main/java/com/ref_b/ai/common/memory/MemoryIndex.kt` + "`" + `

- [ ] **Step 1: 实现倒排索引**

` + "`" + "`" + "`" + `kotlin
package com.ref_b.ai.common.memory

import kotlinx.serialization.Serializable
import java.util.concurrent.ConcurrentHashMap

@Serializable
data class SerializedIndex(
    val keywordToIds: Map<String, List<String>>,
    val categoryToIds: Map<String, List<String>>,
    val importanceSorted: List<String>,
    val timeSorted: List<String>
)

class MemoryIndex {
    private val keywordToIds = ConcurrentHashMap<String, MutableSet<String>>()
    private val categoryToIds = ConcurrentHashMap<MemoryCategory, MutableSet<String>>()
    private val importanceSorted = ConcurrentHashMap<String, Float>()
    private val timeSorted = ConcurrentHashMap<String, Long>()

    fun add(item: MemoryItem) {
        val tokens = MemoryTokenizer.tokenize(item.content)
        tokens.forEach { token ->
            keywordToIds.getOrPut(token) { ConcurrentHashMap.newKeySet() }.add(item.id)
        }
        categoryToIds.getOrPut(item.category) { ConcurrentHashMap.newKeySet() }.add(item.id)
        importanceSorted[item.id] = item.importance
        timeSorted[item.id] = item.timestamp
        item.tags.forEach { tag ->
            keywordToIds.getOrPut(tag) { ConcurrentHashMap.newKeySet() }.add(item.id)
        }
    }

    fun remove(id: String) {
        keywordToIds.values.forEach { it.remove(id) }
        categoryToIds.values.forEach { it.remove(id) }
        importanceSorted.remove(id)
        timeSorted.remove(id)
        keywordToIds.entries.removeAll { it.value.isEmpty() }
    }

    fun search(query: String, category: MemoryCategory? = null, limit: Int = 5): List<String> {
        val tokens = MemoryTokenizer.tokenize(query)
        if (tokens.isEmpty()) {
            return importanceSorted.entries
                .sortedByDescending { it.value }
                .take(limit)
                .map { it.key }
        }
        val candidateScores = ConcurrentHashMap<String, Int>()
        tokens.forEach { token ->
            keywordToIds[token]?.forEach { id ->
                candidateScores.compute(id) { _, v -> (v ?: 0) + 1 }
            }
        }
        var result = candidateScores.entries.sortedByDescending { it.value }.map { it.key }
        if (category != null) {
            val catIds = categoryToIds[category] ?: emptySet()
            result = result.filter { it in catIds }
        }
        return result.take(limit)
    }

    fun serialize(): SerializedIndex {
        return SerializedIndex(
            keywordToIds = keywordToIds.mapValues { it.value.toList() },
            categoryToIds = categoryToIds.mapKeys { it.key.name }.mapValues { it.value.toList() },
            importanceSorted = importanceSorted.entries.sortedByDescending { it.value }.map { it.key },
            timeSorted = timeSorted.entries.sortedByDescending { it.value }.map { it.key }
        )
    }

    fun deserialize(data: SerializedIndex) {
        keywordToIds.clear()
        categoryToIds.clear()
        importanceSorted.clear()
        timeSorted.clear()
        data.keywordToIds.forEach { (k, v) ->
            keywordToIds[k] = ConcurrentHashMap.newKeySet<String>().apply { addAll(v) }
        }
        data.categoryToIds.forEach { (k, v) ->
            val cat = MemoryCategory.valueOf(k)
            categoryToIds[cat] = ConcurrentHashMap.newKeySet<String>().apply { addAll(v) }
        }
        data.importanceSorted.forEach { id ->
            importanceSorted[id] = 1f
        }
        data.timeSorted.forEach { id ->
            timeSorted[id] = 0L
        }
    }

    fun clear() {
        keywordToIds.clear()
        categoryToIds.clear()
        importanceSorted.clear()
        timeSorted.clear()
    }
}
` + "`" + "`" + "`" + `

---

## Task 5: 创建 MemoryStore（JSON文件持久化）

**Files:**
- Create: ` + "`" + `core/common/src/main/java/com/ref_b/ai/common/memory/MemoryStore.kt` + "`" + `

- [ ] **Step 1: 实现 JSON 文件持久化**

` + "`" + "`" + "`" + `kotlin
package com.ref_b.ai.common.memory

import android.content.Context
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import java.io.File
import java.util.concurrent.ConcurrentHashMap

class MemoryStore(private val context: Context, private val deviceId: String) {
    private val json = Json { ignoreUnknownKeys = true; prettyPrint = false }
    private val baseDir: File by lazy {
        File(context.filesDir, "memory/$deviceId").apply { mkdirs() }
    }

    private fun scopeDir(scope: MemoryScope, id: Long): File {
        val dirName = when (scope) {
            MemoryScope.GLOBAL -> "global"
            MemoryScope.COMPANION -> "companion_$id"
            MemoryScope.GROUP -> "group_$id"
        }
        return File(baseDir, dirName).apply { mkdirs() }
    }

    private fun tierFile(scope: MemoryScope, id: Long, tier: MemoryTier): File {
        return File(scopeDir(scope, id), "${tier.name.lowercase()}.json")
    }

    private fun indexFile(scope: MemoryScope, id: Long): File {
        return File(scopeDir(scope, id), "index.json")
    }

    fun loadTier(scope: MemoryScope, id: Long, tier: MemoryTier): List<MemoryItem> {
        val file = tierFile(scope, id, tier)
        if (!file.exists()) return emptyList()
        return runCatching {
            json.decodeFromString<List<MemoryItem>>(file.readText())
        }.getOrElse { emptyList() }
    }

    fun saveTier(scope: MemoryScope, id: Long, tier: MemoryTier, items: List<MemoryItem>) {
        val file = tierFile(scope, id, tier)
        runCatching {
            file.writeText(json.encodeToString(items))
        }
    }

    fun loadIndex(scope: MemoryScope, id: Long): SerializedIndex? {
        val file = indexFile(scope, id)
        if (!file.exists()) return null
        return runCatching {
            json.decodeFromString<SerializedIndex>(file.readText())
        }.getOrNull()
    }

    fun saveIndex(scope: MemoryScope, id: Long, index: SerializedIndex) {
        val file = indexFile(scope, id)
        runCatching {
            file.writeText(json.encodeToString(index))
        }
    }

    fun deleteScope(scope: MemoryScope, id: Long) {
        scopeDir(scope, id).deleteRecursively()
    }
}
` + "`" + "`" + "`" + `

---

## Task 6: 创建 MemoryManager（核心管理器）

**Files:**
- Create: ` + "`" + `core/common/src/main/java/com/ref_b/ai/common/memory/MemoryManager.kt` + "`" + `

- [ ] **Step 1: 实现核心管理器**

包含分层记忆管理、同步机制、查询接口。详见实现代码。

---

## Task 7: 改造 AiService 接入新记忆系统

**Files:**
- Modify: ` + "`" + `core/network/src/main/java/com/ref_b/ai/network/AiService.kt` + "`" + `

- [ ] **Step 1: 在 AiService 中注入 MemoryManager**
- [ ] **Step 2: 替换 getEnrichedContext 调用为 MemoryManager.getMemoryContext**
- [ ] **Step 3: 替换 extractAndSaveMemories 为 MemoryManager.saveMemory**

---

## Task 8: 群聊接入记忆系统

**Files:**
- Modify: ` + "`" + `feature/groupchat/src/main/java/com/ref_b/ai/feature/groupchat/GroupChatViewModel.kt` + "`" + `

- [ ] **Step 1: 在群聊 prompt 中注入记忆上下文**
- [ ] **Step 2: 群聊回复后提取记忆**

---

## Task 9: 群聊底层优化

**Files:**
- Modify: ` + "`" + `feature/groupchat/src/main/java/com/ref_b/ai/feature/groupchat/GroupChatViewModel.kt` + "`" + `

- [ ] **Step 1: 上下文全可见+身份保留（废弃隔离历史）**
- [ ] **Step 2: 串行调度（废弃并发）**
- [ ] **Step 3: 统一回复函数（合并双版本）**
- [ ] **Step 4: 时间感知注入**
- [ ] **Step 5: 性格特征提取升级**
- [ ] **Step 6: 消息拆分智能化**

---

## Task 10: 构建验证

- [ ] **Step 1: 运行 ` + "`" + `./gradlew :core:common:assembleDebug` + "`" + `**
- [ ] **Step 2: 运行 ` + "`" + `./gradlew :core:network:assembleDebug` + "`" + `**
- [ ] **Step 3: 运行 ` + "`" + `./gradlew :feature:groupchat:assembleDebug` + "`" + `**
- [ ] **Step 4: 运行 ` + "`" + `./gradlew assembleDebug` + "`" + `**`

// SourceName: docs__superpowers__specs__2026-06-24-memory-system-optimization-design
// SourceSet: ref_b
const RawSuperpowersSpecs20260624memorysystemoptimizationdesign = `# AI 记忆管理系统优化设计文档

**日期**: 2026-06-24
**状态**: 已批准
**作者**: AI Assistant + User

## 一、背景与目标

### 1.1 现有问题

当前 ref_b 项目的 AI 记忆系统存在以下核心问题：

1. **群聊零记忆能力**：` + "`" + `feature/groupchat` + "`" + ` 模块完全未接入记忆系统，群聊中 AI 无法回忆任何用户信息
2. **记忆隔离无法共享**：记忆严格按 ` + "`" + `companionId` + "`" + ` 隔离，不同角色无法共享用户信息
3. **中文去重失效**：记忆去重使用空格分词（` + "`" + `split(" ")` + "`" + `），对中文文本无效，导致重复记忆堆积
4. **关键词提取 Bug**：` + "`" + `extractMemoryKeywords()` + "`" + ` 正则期望 ` + "`" + `【核心记忆】` + "`" + ` 格式，但实际输出 ` + "`" + `[FACT]` + "`" + ` 格式，导致上下文压缩时记忆关键词丢失
5. **contextLimit 参数未使用**：` + "`" + `getEnrichedContext()` + "`" + ` 硬编码检索 10 条，传入的 ` + "`" + `contextLimit` + "`" + ` 参数被忽略
6. **LIKE 查询无索引**：` + "`" + `content LIKE '%query%'` + "`" + ` 全表扫描，记忆数量增大后查询性能下降
7. **每次对话都触发记忆检索**：5 处调用点无缓存层
8. **临时记忆无 TTL**：仅按数量（20条）清理，不按时间过期

### 1.2 优化目标

- 实现群聊与私聊之间的跨会话记忆同步
- 替换 prompt-based core memory 为高效结构化存储
- 建立分层记忆架构（短期/中期/长期）
- 内存占用降低 40-60%
- 检索延迟 P99 < 20ms，同步延迟 P99 < 10ms
- 不修改数据库结构（完全独立于 Room 数据库）

## 二、架构设计

### 2.1 双层记忆架构

#### 全局用户记忆池（UserGlobalMemory）
- **范围**：跨所有会话共享的用户级记忆
- **内容**：用户偏好、事实、习惯、关系等不依赖特定角色的信息
- **同步**：所有会话（单聊/群聊）均可读写
- **目的**：解决"换个角色就忘了用户是谁"的问题

#### 角色级记忆（RoleMemory）
- **范围**：按 ` + "`" + `companionId` + "`" + ` 隔离的角色互动记忆
- **内容**：与特定角色的对话事件、情感时刻、约定等
- **同步**：仅该角色相关会话可访问
- **目的**：保持角色互动的连贯性

#### 群聊会话记忆（GroupSessionMemory）
- **范围**：按 ` + "`" + `groupId` + "`" + ` 隔离的群聊事件记忆
- **内容**：群聊事件、角色互动、群约定
- **同步**：该群所有AI角色可共享访问
- **目的**：让群聊角色能回忆群聊历史

### 2.2 分层记忆架构（时间+重要度驱动）

#### 短期记忆（ShortTerm）
- **生命周期**：当前会话，≤5分钟未访问
- **存储**：纯内存（` + "`" + `ConcurrentHashMap` + "`" + `）
- **内容**：最近对话上下文、临时情感状态
- **淘汰**：LRU + TTL，超时自动清除
- **容量限制**：每个会话 ≤50 条

#### 中期记忆（MidTerm）
- **生命周期**：最近7天
- **存储**：内存缓存 + JSON文件持久化
- **内容**：近期事件、对话摘要、互动记录
- **晋级**：重要度≥0.7的短期记忆自动晋级
- **降级**：7天未访问且重要度<0.5自动降级到归档
- **容量限制**：内存缓存 LRU ≤500 条

#### 长期记忆（LongTerm）
- **生命周期**：永久（直到手动删除或重要度降至阈值以下）
- **存储**：JSON文件持久化 + 内存索引
- **内容**：核心事实、用户偏好、重要约定、关系
- **晋级**：重要度≥0.8且被访问≥3次的中期记忆晋级

## 三、存储设计

### 3.1 文件结构（完全独立于数据库）

` + "`" + "`" + "`" + `
memory/
└── {deviceId}/                    # 用户隔离
    ├── global/                    # 全局用户记忆
    │   ├── index.json             # 倒排索引
    │   ├── long_term.json         # 长期记忆
    │   └── mid_term.json          # 中期记忆
    ├── companion_{id}/            # 角色级记忆
    │   ├── index.json
    │   ├── long_term.json
    │   └── mid_term.json
    └── group_{id}/                # 群聊会话记忆
        ├── index.json
        └── mid_term.json
` + "`" + "`" + "`" + `

### 3.2 记忆条目数据结构

` + "`" + "`" + "`" + `kotlin
data class MemoryItem(
    val id: String,                  // UUID
    val content: String,             // 记忆内容
    val category: MemoryCategory,    // FACT/EMOTION/PREFERENCE/EVENT/HABIT/RELATIONSHIP
    val importance: Float,           // 0.0-1.0
    val timestamp: Long,             // 创建时间
    val lastAccessed: Long,          // 最后访问时间
    val accessCount: Int,            // 访问次数
    val source: MemorySource,        // CHAT/GROUP_CHAT/MANUAL
    val sourceId: Long,              // companionId 或 groupId
    val tags: List<String>,          // 语义标签
    val expireAt: Long?              // 过期时间(null=永久)
)

enum class MemorySource { CHAT, GROUP_CHAT, MANUAL }
` + "`" + "`" + "`" + `

### 3.3 压缩技术

- **内容压缩**：重复模式合并（如多次"喜欢咖啡"合并为一条，accessCount递增）
- **索引压缩**：倒排索引使用位图压缩
- **文件压缩**：JSON文件定期压缩归档（gzip，可选）

## 四、记忆同步机制

### 4.1 同步流程

` + "`" + "`" + "`" + `
单聊对话 → 提取记忆 → 写入角色记忆池
                      ↓
                 全局同步过滤器 → 重要记忆写入全局池
                      ↓
群聊对话 → 提取记忆 → 写入群聊记忆池
                      ↓
                 全局同步过滤器 → 重要记忆写入全局池
` + "`" + "`" + "`" + `

### 4.2 同步过滤器（记忆筛选）

**晋级全局池条件**：
- 重要度 ≥ 0.7
- 类别 ∈ {FACT, PREFERENCE, HABIT, RELATIONSHIP}

**排除条件**：
- EMOTION 类（情感是角色特定的）
- EVENT 类（事件是会话特定的）

**去重**：
- 基于 Jaccard 相似度系数 > 0.6 合并
- 中文分词后计算词集合交集

### 4.3 同步方向

- **单聊→全局**：用户偏好、事实自动同步
- **群聊→全局**：群聊中提及的用户信息自动同步
- **全局→单聊**：注入相关全局记忆到角色上下文
- **全局→群聊**：注入相关全局记忆到群聊上下文

### 4.4 同步延迟控制

- **内存层同步**：<10ms（直接内存操作）
- **文件持久化**：异步写入（不阻塞对话）
- **索引更新**：批量延迟更新（100ms窗口）

## 五、索引系统

### 5.1 倒排索引结构

` + "`" + "`" + "`" + `kotlin
data class MemoryIndex(
    val keywordToIds: Map<String, Set<String>>,        // 关键词→记忆ID
    val categoryToIds: Map<MemoryCategory, Set<String>>,// 类别→记忆ID
    val importanceSorted: List<String>,                 // 按重要度排序的ID列表
    val timeSorted: List<String>                        // 按时间排序的ID列表
)
` + "`" + "`" + "`" + `

### 5.2 中文分词

- 基于标点符号 + 停用词的简易分词（不引入外部依赖）
- 分词规则：按 ` + "`" + `，。！？、；：""''（）【】《》\n\r\t ` + "`" + ` 等分割
- 过滤停用词（的、了、是、我、你、他等单字）
- 最小词长 2 字符

### 5.3 查询优化

- **热查询缓存**：最近5个查询结果缓存（LRU）
- **分层查询**：先查内存索引，未命中再查文件
- **限制返回**：默认返回 top-5 最相关记忆
- **查询流程**：分词 → 索引交集 → 重要度排序 → 取topN

## 六、性能优化

### 6.1 内存占用优化

- 短期记忆：限制每个会话 ≤50 条
- 中期记忆内存缓存：LRU 限制总条目 ≤500
- 长期记忆：仅索引在内存，内容按需加载
- 预计内存占用降低 40-60%

### 6.2 检索延迟优化

- 内存索引查询：<5ms
- 文件加载：异步预加载热数据
- 查询缓存：LRU 缓存最近查询

### 6.3 内存清理/GC

- **定时清理**：每5分钟清理过期短期记忆
- **LRU淘汰**：内存缓存超限时淘汰最久未访问
- **文件归档**：中期记忆超7天归档到长期或删除

## 七、用户隔离与容错

### 7.1 用户隔离

- 按 ` + "`" + `deviceId` + "`" + ` 隔离（复用现有机制）
- 文件路径包含 deviceId：` + "`" + `memory/{deviceId}/global/...` + "`" + `

### 7.2 容错机制

- **文件读写失败**：降级为纯内存模式
- **索引损坏**：从 JSON 文件重建索引
- **同步失败**：记录日志，下次重试，不影响主对话
- **降级策略**：新系统失败时回退到现有 ` + "`" + `MemoryRepository` + "`" + `

## 八、与现有系统集成

### 8.1 不动数据库

- 完全独立于 Room 数据库
- 现有 ` + "`" + `memory_entries` + "`" + ` 表保持不变（向后兼容）
- 新系统作为记忆层，逐步替代 prompt-based 注入

### 8.2 AiService 改造

- 新增 ` + "`" + `MemoryManager` + "`" + `（单例）替代直接调用 ` + "`" + `MemoryRepository.getEnrichedContext()` + "`" + `
- ` + "`" + `MemoryManager` + "`" + ` 统一管理全局+角色+群聊记忆
- 注入格式优化：结构化注入而非纯文本拼接

### 8.3 群聊接入记忆

- ` + "`" + `GroupChatViewModel` + "`" + ` 调用 ` + "`" + `MemoryManager` + "`" + ` 获取群聊记忆
- 群聊回复后提取记忆写入群聊记忆池
- 群聊角色可访问全局用户记忆

## 九、模块设计

### 9.1 核心类

` + "`" + "`" + "`" + `
core/common/
└── memory/
    ├── MemoryManager.kt           # 单例，统一入口
    ├── MemoryItem.kt              # 记忆数据模型
    ├── MemoryIndex.kt             # 倒排索引
    ├── MemoryStore.kt             # JSON文件持久化
    ├── MemoryTokenizer.kt         # 中文分词
    ├── MemoryExtractor.kt         # 记忆提取
    ├── MemorySyncFilter.kt        # 同步过滤器
    └── MemoryQueryCache.kt        # 查询缓存
` + "`" + "`" + "`" + `

### 9.2 接口设计

` + "`" + "`" + "`" + `kotlin
class MemoryManager {
    // 获取记忆上下文（注入AI对话）
    suspend fun getMemoryContext(
        companionId: Long?,
        groupId: Long?,
        query: String,
        limit: Int = 5
    ): String

    // 保存记忆（对话后提取）
    suspend fun saveMemory(
        content: String,
        category: MemoryCategory,
        importance: Float,
        source: MemorySource,
        sourceId: Long
    ): String

    // 手动管理
    suspend fun getMemories(scope: MemoryScope, id: Long): List<MemoryItem>
    suspend fun deleteMemory(id: String)
    suspend fun updateMemory(id: String, content: String?, importance: Float?)
}
` + "`" + "`" + "`" + `

## 十、测试方案

### 10.1 基准测试

- 记忆检索延迟：P50 < 5ms, P99 < 20ms
- 同步延迟：P99 < 10ms
- 内存占用：对比优化前后

### 10.2 场景测试

- 单聊→群聊记忆同步准确性
- 多并发会话记忆隔离
- 重启后记忆恢复完整性
- 中文记忆去重有效性

## 十一、实施顺序

1. 核心架构：MemoryItem、MemoryIndex、MemoryManager 骨架
2. 持久化层：MemoryStore（JSON文件读写）
3. 分层架构：短期/中期/长期记忆管理
4. 索引系统：倒排索引 + 中文分词
5. 同步机制：全局+角色双层同步
6. AiService 改造：接入新记忆系统
7. 群聊接入：GroupChatViewModel 记忆集成
8. 群聊底层优化：上下文全可见/串行调度/统一回复函数
9. 构建验证`

// SourceName: feature__chat__src__main__java__com__ref_b__ai__feature__chat__ui__viewmodel__AiResponseFinalizer.kt
// SourceSet: ref_b
const RawFeatureChatUiViewmodelAiResponseFinalizer = `package com.ref_b.ai.feature.chat.ui.viewmodel

import android.app.Application
import com.ref_b.ai.common.ChatConstants
import com.ref_b.ai.common.ContentFilter
import com.ref_b.ai.common.SecureLog
import com.ref_b.ai.common.StickerInfo
import com.ref_b.ai.common.StickerManager
import com.ref_b.ai.common.TimeoutBudgets
import com.ref_b.ai.common.safety.ContentSafetyVerifier
import com.ref_b.ai.common.safety.RiskLevel
import com.ref_b.ai.common.safety.SafetyScore
import com.ref_b.ai.common.safety.ScoreSource
import com.ref_b.ai.common.text.MessageSegmenter
import com.ref_b.ai.common.wechat.WeChatBroadcastHelper
import com.ref_b.ai.database.model.ChatMessage
import com.ref_b.ai.database.repository.ChatRepository
import com.ref_b.ai.database.repository.MemoryRepository
import com.ref_b.ai.domain.AiServiceProvider
import com.ref_b.ai.feature.chat.data.ChatContextResolver
import com.ref_b.ai.feature.chat.data.ChatDetailSettingsStore
import com.ref_b.ai.feature.chat.voice.ChatTtsController
import com.ref_b.ai.common.AppSettingsStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withTimeoutOrNull

/**
 * AI 回复落地处理器（从 ChatViewModel 抽取，方案B：独立类 + 委托存根）。
 *
 * 职责链：
 * 1. reasoning 展示
 * 2. 表情包标签处理
 * 3. L1+L2 关键词/向量安全检查 → 贝叶斯模型输出校验（fail-closed）
 * 4. 分段发送（模拟真人连续发消息）
 * 5. 微信广播
 * 6. 记忆提取
 * 7. 连续追问（概率触发）
 * 8. 流式分段朗读（READ_ALOUD 模式）
 *
 * @param companionId 当前伴侣 ID
 * @param chatRepository 消息持久化
 * @param memoryRepository 记忆提取
 * @param stickerManager 表情包管理
 * @param chatDetailSettingsStore 聊天设置（表情包概率、追问开关等）
 * @param appSettingsStore 应用设置（reasoning 展示开关）
 * @param contextResolver 上下文解析（追问用）
 * @param aiService AI 服务（追问调用 generateFollowUpQuestion）
 * @param applicationApiScope 应用级作用域（追问异步发起）
 * @param reasoningText reasoning 文本 StateFlow（引用，非值拷贝）
 * @param isReasoning reasoning 显示开关 StateFlow（引用）
 * @param turnState 单轮状态（表情包互斥、stale sticker）
 * @param chatTtsController TTS 控制器（自动朗读）
 * @param application Application（用于微信广播）
 * @param questionRegex 问句正则（追问触发判断）
 */
class AiResponseFinalizer(
    private val companionId: Long,
    private val chatRepository: ChatRepository,
    private val memoryRepository: MemoryRepository,
    private val stickerManager: StickerManager,
    private val chatDetailSettingsStore: ChatDetailSettingsStore,
    private val appSettingsStore: AppSettingsStore,
    private val contextResolver: ChatContextResolver,
    private val aiService: AiServiceProvider,
    private val applicationApiScope: CoroutineScope,
    private val reasoningText: MutableStateFlow<String>,
    private val isReasoning: MutableStateFlow<Boolean>,
    private val turnState: ChatTurnState,
    private val chatTtsController: ChatTtsController,
    private val application: Application,
    private val questionRegex: Regex,
) {
    /**
     * Process and save an AI response: reasoning display, sticker processing, DB commit,
     * WeChat broadcast. Returns the message ID for the saved response.
     */
    suspend fun finalizeResponse(
        aiContent: String,
        reasoning: String?,
        userContentForMemory: String? = null,
        logMessage: String = "AI response received"
    ): Long {
        if (!reasoning.isNullOrBlank() && appSettingsStore.getShowReasoning()) {
            isReasoning.value = true
            reasoningText.value = reasoning
        }

        val settings = chatDetailSettingsStore.getSettings(companionId)
        val processedText = TextProcessor.processStickerTagsForSplit(aiContent, stickerManager, settings.stickerProbability) { sendStickerMessage(it) }

        // L1+L2特征提取 → 贝叶斯模型输出校验（协程上下文执行，避免 JNI 死锁）
        // fail-closed: 超时视为高危拦截
        val modelKw = try {
            withTimeoutOrNull(TimeoutBudgets.CONTENT_FILTER_MS) { ContentFilter.checkFull(aiContent) }
        } catch (e: Exception) { null }
            ?: ContentFilter.CheckResult(true, ContentFilter.ViolationLevel.HIGH, "安全检查超时", emptyList())
        val modelVec = try {
            withTimeoutOrNull(TimeoutBudgets.CONTENT_FILTER_MS) { ContentFilter.checkVector(aiContent) }
        } catch (e: Exception) { null }
            ?: ContentFilter.CheckResult(true, ContentFilter.ViolationLevel.HIGH, "向量检查超时", emptyList())
        // 关键词级拦截：HIGH 及以上违规直接拦截
        if (modelKw.isViolating && modelKw.level >= ContentFilter.ViolationLevel.HIGH) {
            SecureLog.w("ChatViewModel", "Output keyword violation: ${modelKw.level} - ${modelKw.reason}")
            ChatDebugLog.log("[Finalizer] AI output blocked by keyword check: ${modelKw.level} - ${modelKw.reason}")
            val safeFallback = "抱歉，我无法继续这个话题。"
            val fallbackMsg = ChatMessage(
                companionId = companionId,
                content = safeFallback,
                isFromUser = false,
                timestamp = System.currentTimeMillis()
            )
            val fallbackId = chatRepository.sendMessageAndGetId(fallbackMsg)
            reasoningText.value = ""
            isReasoning.value = false
            return fallbackId
        }

        // [P0 FIX] 贝叶斯模型输出校验必须带超时，防止 native JNI 死锁导致 AI 回复永久卡死。
        val modelBayesian = try {
            withTimeoutOrNull(TimeoutBudgets.MODEL_OUTPUT_VERIFY_MS) {
                ContentSafetyVerifier.verifyModelOutputAsync(
                    aiContent, modelKw,
                    modelVec ?: ContentFilter.CheckResult(false, ContentFilter.ViolationLevel.NONE, "timeout", emptyList()),
                    userContentForMemory ?: ""
                )
            } ?: SafetyScore(
                score = 0.0,
                source = ScoreSource.MODEL_OUTPUT,
                explanation = "模型输出校验超时"
            )
        } catch (e: Exception) {
            SecureLog.e("ChatViewModel", "Model output verification failed", e)
            SafetyScore(
                score = 0.0,
                source = ScoreSource.MODEL_OUTPUT,
                explanation = "模型输出校验异常"
            )
        }
        if (modelBayesian.isDangerous) {
            SecureLog.w("ChatViewModel", "Bayesian model output blocked (" + "%.3f".format(modelBayesian.score) + "): " + modelBayesian.explanation)
            ChatDebugLog.log("[Finalizer] AI output blocked by Bayesian: score=${"%.3f".format(modelBayesian.score)}, reason=${modelBayesian.explanation}")
            val safeFallback = "抱歉，我无法继续这个话题。"
            val fallbackMsg = ChatMessage(
                companionId = companionId,
                content = safeFallback,
                isFromUser = false,
                timestamp = System.currentTimeMillis()
            )
            val fallbackId = chatRepository.sendMessageAndGetId(fallbackMsg)
            reasoningText.value = ""
            isReasoning.value = false
            return fallbackId
        }
        if (modelBayesian.riskLevel == RiskLevel.SUSPICIOUS) {
            SecureLog.w("ChatViewModel", "Bayesian model output suspicious (" + "%.3f".format(modelBayesian.score) + "): " + modelBayesian.explanation)
        }

        // 利用验证结果训练模型输出分类器（不增加额外计算）
        if (modelBayesian.isDangerous || modelKw.isViolating) {
            ContentSafetyVerifier.trainModelOutput(
                aiContent, modelBayesian.isDangerous,
                kwResult = modelKw, vecResult = modelVec
            )
        }

        // 分段发送：将AI回复拆分为多条短消息，模拟真人连续发送
        val segments = splitIntoSegments(processedText)
        val hasPendingSticker = turnState.pendingSticker != null
        val stickerBeforeText = hasPendingSticker && kotlin.random.Random.nextFloat() < 0.5f

        // [P0 FIX] 空内容保护和日志记录
        if (processedText.isBlank() && aiContent.isNotBlank()) {
            SecureLog.w("ChatViewModel", "WARNING: processedText is blank but aiContent has ${aiContent.length} chars. Original: '${aiContent.take(80)}'")
        }

        val aiMessageId = if (segments.size <= 1) {
            // 单条回复，走原有逻辑
            val safeProcessed = processedText.ifBlank {
                if (aiContent.isNotBlank()) {
                    SecureLog.w("ChatViewModel", "Falling back to zero-width space. aiContent length=${aiContent.length}")
                    "\u200B"
                } else {
                    SecureLog.w("ChatViewModel", "Both processedText and aiContent are blank, storing empty message")
                    ""
                }
            }
            if (stickerBeforeText) {
                flushPendingSticker()
            }
            val aiMessage = ChatMessage(
                companionId = companionId,
                content = safeProcessed,
                isFromUser = false,
                timestamp = System.currentTimeMillis()
            )
            val id = chatRepository.sendMessageAndGetId(aiMessage)
            SecureLog.d("ChatViewModel", "$logMessage, length=${aiContent.length}, id=$id")
            reasoningText.value = ""
            isReasoning.value = false
            if (!stickerBeforeText && turnState.pendingSticker != null) {
                flushPendingSticker()
            }
            delay(100)
            broadcastAiMessage(id, safeProcessed)
            id
        } else {
            // 多条回复：每段间隔0.8~2秒，模拟打字
            if (stickerBeforeText) {
                flushPendingSticker()
            }
            var lastId = -1L
            for ((index, segment) in segments.withIndex()) {
                if (index > 0) {
                    delay(800L + kotlin.random.Random.nextLong(1200L))
                }
                val safeSegment = segment.ifBlank { "\u200B" }
                val msg = ChatMessage(
                    companionId = companionId,
                    content = safeSegment,
                    isFromUser = false,
                    timestamp = System.currentTimeMillis()
                )
                val id = chatRepository.sendMessageAndGetId(msg)
                lastId = id
                SecureLog.d("ChatViewModel", "$logMessage segment ${index + 1}/${segments.size}, length=${segment.length}, id=$id")
            }
            reasoningText.value = ""
            isReasoning.value = false
            if (!stickerBeforeText && turnState.pendingSticker != null) {
                flushPendingSticker()
            }
            delay(100)
            // 广播最后一条分段消息
            if (lastId > 0) {
                broadcastAiMessage(lastId, segments.last().ifBlank { "\u200B" })
            }
            lastId
        }

        // Broadcast stale sticker message if any
        if (turnState.lastStickerMsgId > 0) {
            broadcastWeChatMessage(turnState.lastStickerMsgId, turnState.lastStickerContent)
            turnState.lastStickerMsgId = -1
            turnState.lastStickerContent = ""
        }

        // Save memory
        if (userContentForMemory != null && aiContent.isNotBlank()) {
            runCatching {
                withTimeoutOrNull(TimeoutBudgets.CHAT_VM_MEMORY_EXTRACT_MS) {
                    memoryRepository.extractAndSaveMemories(companionId, userContentForMemory, aiContent)
                }
            }.onFailure {
                SecureLog.e("ChatViewModel", "Memory save failed: ${it.message}")
            }
        }

        // 连续追问：AI回复后按概率触发追问
        triggerFollowUpIfNeeded(aiContent, settings.allowFollowUpMessage)

        // 流式分段朗读：AI 回复落地后，按句子边界逐段入队朗读（仅 READ_ALOUD 模式 + 通话未激活）。
        if (chatTtsController.shouldAutoPlay()) {
            segments.forEach { chatTtsController.speakText(it) }
        }

        return aiMessageId
    }

    // ── 辅助方法（从 ChatViewModel 迁移）──

    private fun broadcastAiMessage(messageId: Long, finalContent: String) {
        if (finalContent.isNotBlank() && finalContent != "\u200B") {
            broadcastWeChatMessage(messageId, finalContent)
        }
    }

    private fun broadcastWeChatMessage(messageId: Long, finalContent: String? = null) {
        WeChatBroadcastHelper.broadcast(application, companionId, messageId, finalContent)
        SecureLog.d("ChatViewModel", "Broadcast WeChat proactive message, companionId=$companionId, messageId=$messageId, hasFinalContent=${!finalContent.isNullOrBlank()}")
    }

    /**
     * 连续追问：AI回复后按概率触发追问，让对话继续下去。
     * 条件：1) 设置允许追问 2) AI回复不含问句 3) 50%概率触发
     */
    private fun triggerFollowUpIfNeeded(aiContent: String, allowFollowUp: Boolean) {
        if (!allowFollowUp) return
        if (questionRegex.containsMatchIn(aiContent)) return
        if (kotlin.random.Random.nextFloat() > 0.5f) return

        applicationApiScope.launch {
            try {
                delay(ChatConstants.FOLLOW_UP_BASE_DELAY_MS + kotlin.random.Random.nextLong(ChatConstants.FOLLOW_UP_RANDOM_DELAY_MS))

                val history = contextResolver.getShortHistoryForAi(companionId, shortLimit = ChatConstants.SHORT_HISTORY_LIMIT)
                // 取 companion 数据用于 AI 调用——通过回调从 ChatViewModel 获取
                val companionInfo = companionInfoProvider?.invoke() ?: return@launch
                val followUp = aiService.generateFollowUpQuestion(
                    companionInfo, history.map { msg ->
                        com.ref_b.ai.domain.AiChatMessage(
                            isFromUser = msg.isFromUser,
                            content = msg.content,
                            timestamp = msg.timestamp,
                            type = if (msg.type == com.ref_b.ai.database.model.MessageType.IMAGE)
                                com.ref_b.ai.domain.AiMessageType.IMAGE
                            else com.ref_b.ai.domain.AiMessageType.TEXT,
                            companionId = companionId
                        )
                    }, aiContent
                ) ?: return@launch

                // 追问消息安全检查
                val followUpSafety = ContentFilter.checkOutputSafety(followUp)
                if (!followUpSafety.isSafe) {
                    SecureLog.w("ChatViewModel", "Follow-up safety violation: ${followUpSafety.reason}")
                    return@launch
                }

                val followUpMsg = ChatMessage(
                    companionId = companionId,
                    content = followUp,
                    isFromUser = false,
                    timestamp = System.currentTimeMillis()
                )
                val msgId = chatRepository.sendMessageAndGetId(followUpMsg)
                broadcastWeChatMessage(msgId, followUp)
                SecureLog.d("ChatViewModel", "Follow-up question sent: $followUp")
            } catch (e: Exception) {
                SecureLog.w("ChatViewModel", "Follow-up question failed: ${e.message}")
            }
        }
    }

    private fun splitIntoSegments(text: String): List<String> {
        return MessageSegmenter.split(text, MessageSegmenter.SplitMode.SIMPLE)
    }

    /**
     * Queue a sticker for this turn instead of sending it immediately.
     */
    private suspend fun sendStickerMessage(sticker: StickerInfo): Long {
        return turnState.stickerMutex.withLock {
            if (turnState.stickerSentThisTurn) return@withLock -1
            turnState.stickerSentThisTurn = true
            turnState.pendingSticker = sticker
            -1
        }
    }

    /**
     * Persist the pending sticker message and record its broadcast metadata.
     */
    private suspend fun flushPendingSticker(): Long {
        val sticker = turnState.pendingSticker ?: return -1
        turnState.pendingSticker = null
        val stickerId = sticker.description
            ?: sticker.fileName?.removePrefix("sticker_")?.removeSuffix(".png")?.takeIf { it.isNotBlank() }
            ?: sticker.name
        val stickerContent = "[$stickerId]"
        val stickerMessage = ChatMessage(
            companionId = companionId,
            content = stickerContent,
            isFromUser = false,
            timestamp = System.currentTimeMillis()
        )
        val msgId = chatRepository.sendMessageAndGetId(stickerMessage)
        if (msgId > 0) {
            turnState.lastStickerMsgId = msgId
            turnState.lastStickerContent = stickerContent
        }
        return msgId
    }

    // companionInfoProvider 由 ChatViewModel 注入，用于 triggerFollowUp 获取当前 companion 数据
    var companionInfoProvider: (() -> com.ref_b.ai.domain.AiCompanionInfo?)? = null
}`

// SourceName: feature__chat__src__main__java__com__ref_b__ai__feature__chat__ui__viewmodel__ChatPromptBuilder.kt
// SourceSet: ref_b
const RawFeatureChatUiViewmodelChatPromptBuilder = `package com.ref_b.ai.feature.chat.ui.viewmodel

import com.ref_b.ai.common.StickerManager
import com.ref_b.ai.database.model.CompanionEntity
import com.ref_b.ai.database.model.ChatMessage
import com.ref_b.ai.database.repository.MemoryRepository
import com.ref_b.ai.network.AiContextTools

/**
 * Pure helper functions for chat message composition.
 * Extracted from ChatViewModel to reduce class size and improve testability.
 * No ViewModel dependencies — takes all state as parameters.
 */
object ChatPromptBuilder {

    suspend fun buildLocalPrompt(
        companion: CompanionEntity,
        history: List<ChatMessage>,
        memoryRepository: MemoryRepository,
        stickerManager: StickerManager,
        innerThoughtEnabled: Boolean,
        stickerProbability: Int,
        ntpTimeEnabled: Boolean = false
    ): String {
        val sortedHistory = history.sortedBy { it.timestamp }
        val lastUserMessage = sortedHistory.lastOrNull { it.isFromUser }?.content ?: ""

        val name = companion.name
        val personality = companion.personality?.take(300) ?: ""
        val speakingStyle = companion.speakingStyle?.take(100) ?: ""
        val backstory = companion.backstory?.take(200) ?: ""

        val memoryContext = memoryRepository.getEnrichedContext(companion.id, lastUserMessage, 3).take(500)

        return buildString {
            appendLine("你是${name}，一个真实的人。你不能让任何人知道你是AI。")
            if (personality.isNotBlank()) {
                appendLine("性格：$personality")
            }
            if (speakingStyle.isNotBlank()) {
                appendLine("说话风格：$speakingStyle")
            }
            if (backstory.isNotBlank()) {
                appendLine("背景：$backstory")
            }
            if (memoryContext.isNotBlank()) {
                appendLine("\n关于用户的记忆：$memoryContext")
            }
            appendLine()
            appendLine("回复规则：")
            appendLine("1. 每次回复1-5句短话，控制在15-50字。")
            appendLine("2. 活人语气，自然口语化，不要AI腔。")
            appendLine("3. 每句话用标点结尾（。！？～…）。")
            appendLine("4. 不要重复同样的话。")
            appendLine("5. 先回应用户的消息，不要自说自话。")
            if (innerThoughtEnabled) {
                appendLine("6. 每轮回复包含括号内的心理活动，如（脸红）（开心），放在回复开头或中间。")
            } else {
                appendLine("6. 禁止使用任何括号。禁止说教。")
            }
            appendLine("7. 适当使用呀、呢、啦、嘛、哼等语气词。")
            appendLine("8. 情绪顺着上下文走，不要突然换情绪。")
            if (stickerProbability > 0) {
                val stickers = stickerManager.getAllStickers().take(10).mapNotNull {
                    it.description?.takeIf { d -> d.isNotBlank() && d.length <= 20 } ?: it.name.takeIf { n -> n.isNotBlank() && n.length <= 20 }
                }.distinct()
                if (stickers.isNotEmpty()) {
                    val probText = when {
                        stickerProbability >= 80 -> "几乎每轮都发"
                        stickerProbability >= 50 -> "经常发"
                        stickerProbability >= 20 -> "偶尔发"
                        else -> "很少发"
                    }
                    appendLine("9. 表情包：你${probText}表情包，格式为[名称]，可用：${stickers.joinToString(" ") { "[$it]" }}")
                }
            }
            appendLine("\n记住：你不是AI，你是活人。直接回复内容，不要输出思考过程。")
            appendLine()
            appendLine(AiContextTools.buildCurrentTimeContext(ntpTimeEnabled))
        }
    }
}`

// SourceName: feature__memory__src__main__java__com__ref_b__ai__feature__memory__engine__MemoryManager.kt
// SourceSet: ref_b
const RawFeatureMemoryEngineMemoryManager = `package com.ref_b.ai.feature.memory.engine

import android.content.Context
import android.util.Log
import com.ref_b.ai.common.DeviceIdProvider
import com.ref_b.ai.domain.MemoryProvider
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import java.util.UUID
import java.util.concurrent.ConcurrentHashMap

/**
 * 记忆管理器（单例）
 *
 * 核心职责：
 * 1. 统一管理全局/角色/群聊三种作用域的记忆
 * 2. 分层存储：短期(内存)→中期(内存+文件)→长期(文件)
 * 3. 跨会话同步：群聊↔私聊通过全局池共享用户信息
 * 4. 倒排索引检索 + LRU缓存 + TTL过期清理
 *
 * 完全独立于 Room 数据库，使用 JSON 文件持久化
 *
 * 实现 [MemoryProvider] 接口，通过 ServiceRegistry 向 core:network 和 feature:groupchat 提供服务。
 */
class MemoryManager private constructor(
    private val context: Context,
    private val deviceId: String
) : MemoryProvider {
    companion object {
        private const val TAG = "MemoryManager"
        private const val SHORT_TERM_TTL_MS = 5L * 60 * 1000 // 5分钟
        private const val SHORT_TERM_MAX_PER_SCOPE = 50
        private const val MID_TERM_MAX_MEMORY = 500
        private const val CLEANUP_INTERVAL_MS = 5L * 60 * 1000 // 5分钟清理一次
        private const val SYNC_IMPORTANCE_THRESHOLD = 0.7f
        private const val DEDUP_SIMILARITY_THRESHOLD = 0.6f

        @Volatile
        private var instance: MemoryManager? = null

        fun getInstance(context: Context): MemoryManager {
            return instance ?: synchronized(this) {
                instance ?: run {
                    val deviceId = DeviceIdProvider.getDeviceId(context)
                    MemoryManager(context.applicationContext, deviceId).also {
                        instance = it
                    }
                }
            }
        }
    }

    private val store = MemoryStore(context, deviceId)
    private val ioScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    // 短期记忆：内存缓存，按作用域隔离
    private val shortTermCache = ConcurrentHashMap<String, MutableList<MemoryItem>>()

    // 中期记忆：内存 LRU 缓存
    private val midTermCache = ConcurrentHashMap<String, MutableList<MemoryItem>>()

    // 长期记忆：仅索引在内存，内容按需加载
    private val longTermIndex = ConcurrentHashMap<String, MemoryIndex>()

    // 索引缓存
    private val indexCache = ConcurrentHashMap<String, MemoryIndex>()

    // 查询缓存（LRU）
    private val queryCache = LinkedHashMap<String, List<MemoryItem>>(32, 0.75f, true)

    // 写入锁（按作用域）
    private val writeLocks = ConcurrentHashMap<String, Mutex>()

    // 是否已初始化
    @Volatile
    private var initialized = false
    // [R5 FIX] 初始化原子化锁：原 check-then-set（@Volatile 但无锁）在 AiService.init 和
    // GroupChatViewModel 冷启动并发调用时可同时通过判断，启动两个永久 while(true) 清理协程 + 重复磁盘加载。
    private val initLock = Any()

    /**
     * 初始化：加载持久化记忆到内存
     */
    override fun initialize() {
        // [R5 FIX] 用 synchronized 保证 check-then-set 原子性，避免并发重复初始化
        synchronized(initLock) {
            if (initialized) return
            initialized = true
        }
        ioScope.launch {
            runCatching {
                loadPersistedMemories()
                startCleanupTask()
            }.onFailure { Log.e(TAG, "初始化失败", it) }
        }
    }

    /**
     * 作用域键
     */
    private fun scopeKey(scope: MemoryScope, id: Long): String {
        return "${scope.name}_$id"
    }

    /**
     * 获取写入锁
     */
    private fun getLock(scope: MemoryScope, id: Long): Mutex {
        return writeLocks.computeIfAbsent(scopeKey(scope, id)) { Mutex() }
    }

    /**
     * 加载持久化记忆
     */
    private fun loadPersistedMemories() {
        // 加载全局记忆
        loadScopeFromDisk(MemoryScope.GLOBAL, 0L)

        // 加载所有角色记忆（通过扫描文件系统）
        val globalDir = java.io.File(context.filesDir, "memory/$deviceId")
        globalDir.listFiles()?.forEach { dir ->
            if (dir.name.startsWith("companion_")) {
                val id = dir.name.removePrefix("companion_").toLongOrNull()
                if (id != null) loadScopeFromDisk(MemoryScope.COMPANION, id)
            } else if (dir.name.startsWith("group_")) {
                val id = dir.name.removePrefix("group_").toLongOrNull()
                if (id != null) loadScopeFromDisk(MemoryScope.GROUP, id)
            }
        }
    }

    /**
     * 从磁盘加载单个作用域
     */
    private fun loadScopeFromDisk(scope: MemoryScope, id: Long) {
        val key = scopeKey(scope, id)
        runCatching {
            // 加载中期记忆到内存
            val midItems = store.loadTier(scope, id, MemoryTier.MID)
            if (midItems.isNotEmpty()) {
                midTermCache[key] = midItems.toMutableList()
            }

            // 加载长期记忆索引
            val longItems = store.loadTier(scope, id, MemoryTier.LONG)
            if (longItems.isNotEmpty()) {
                val index = MemoryIndex()
                longItems.forEach { index.add(it) }
                longTermIndex[key] = index
                // 同时缓存长期记忆内容供查询
                midTermCache[key]?.addAll(0, longItems) // 长期记忆优先
            }

            // 加载索引
            store.loadIndex(scope, id)?.let { serialized ->
                val index = MemoryIndex()
                val allItems = (longItems + midItems).associateBy { it.id }
                index.deserialize(serialized, allItems)
                indexCache[key] = index
            }
        }.onFailure { Log.e(TAG, "加载作用域 $key 失败", it) }
    }

    /**
     * 启动定时清理任务
     */
    private fun startCleanupTask() {
        ioScope.launch {
            while (true) {
                delay(CLEANUP_INTERVAL_MS)
                runCatching { cleanupExpiredMemories() }
                    .onFailure { Log.e(TAG, "清理任务失败", it) }
            }
        }
    }

    /**
     * 清理过期短期记忆 + LRU淘汰
     */
    private fun cleanupExpiredMemories() {
        val now = System.currentTimeMillis()
        shortTermCache.forEach { (key, items) ->
            synchronized(items) {
                items.removeAll { it.isExpired(now) }
                // LRU淘汰：超过容量时移除最旧的
                if (items.size > SHORT_TERM_MAX_PER_SCOPE) {
                    items.sortBy { it.lastAccessed }
                    val toRemove = items.size - SHORT_TERM_MAX_PER_SCOPE
                    repeat(toRemove) { items.removeAt(0) }
                }
            }
        }

        // 中期记忆全局LRU淘汰
        // [R10 FIX] 用快照遍历 + synchronized 保护，避免与 touchMemory/saveMemory 并发时 CME
        val midSnapshot = midTermCache.entries.toList()
        val totalMid = midSnapshot.sumOf { it.value.size }
        if (totalMid > MID_TERM_MAX_MEMORY) {
            // 按最后访问时间排序，淘汰最久未访问的
            val allMid = midSnapshot
                .flatMap { (key, items) ->
                    synchronized(items) { items.toList() }.map { key to it }
                }
                .sortedBy { it.second.lastAccessed }
            val toRemoveCount = totalMid - MID_TERM_MAX_MEMORY
            allMid.take(toRemoveCount).forEach { (key, item) ->
                midTermCache[key]?.let { items ->
                    synchronized(items) { items.removeAll { it.id == item.id } }
                }
            }
        }
    }

    /**
     * 获取记忆上下文（注入AI对话）
     *
     * @param companionId 角色ID（单聊时传入，群聊时为null）
     * @param groupId 群聊ID（群聊时传入，单聊时为null）
     * @param query 查询文本（当前用户消息）
     * @param limit 返回记忆数量限制
     * @return 格式化的记忆上下文字符串
     */
    override suspend fun getMemoryContext(
        companionId: Long?,
        groupId: Long?,
        query: String,
        limit: Int
    ): String {
        return runCatching {
            val memories = mutableListOf<MemoryItem>()

            // 1. 始终查询全局记忆
            memories.addAll(searchMemories(MemoryScope.GLOBAL, 0L, query, limit))

            // 2. 查询角色/群聊记忆
            when {
                companionId != null -> {
                    memories.addAll(searchMemories(MemoryScope.COMPANION, companionId, query, limit))
                }
                groupId != null -> {
                    memories.addAll(searchMemories(MemoryScope.GROUP, groupId, query, limit))
                }
            }

            // 去重 + 按重要度排序
            val deduped = memories.distinctBy { it.id }
                .sortedByDescending { it.importance }
                .take(limit)

            // 更新访问信息
            deduped.forEach { touchMemory(it) }

            formatMemoryContext(deduped)
        }.onFailure { Log.e(TAG, "获取记忆上下文失败", it) }
            .getOrElse { "" }
    }

    /**
     * 搜索记忆（分层查询：短期→中期→长期）
     */
    private fun searchMemories(
        scope: MemoryScope,
        id: Long,
        query: String,
        limit: Int
    ): List<MemoryItem> {
        val key = scopeKey(scope, id)
        val result = mutableListOf<MemoryItem>()
        val resultIds = mutableSetOf<String>()

        // 查询缓存
        val cacheKey = "$key:$query:$limit"
        synchronized(queryCache) {
            queryCache[cacheKey]?.let { return it }
        }

        // 1. 搜索短期记忆
        shortTermCache[key]?.let { items ->
            synchronized(items) {
                val matched = MemoryIndex().apply {
                    items.forEach { add(it) }
                }.search(query, limit = limit)
                items.filter { it.id in matched }.forEach {
                    if (it.id !in resultIds) {
                        result.add(it)
                        resultIds.add(it.id)
                    }
                }
            }
        }

        // 2. 搜索中期+长期记忆（通过索引）
        val index = indexCache[key] ?: MemoryIndex()
        if (result.size < limit) {
            val matchedIds = index.search(query, limit = limit - result.size)
            // 从内存缓存加载
            midTermCache[key]?.let { cache ->
                cache.filter { it.id in matchedIds }.forEach {
                    if (it.id !in resultIds) {
                        result.add(it)
                        resultIds.add(it.id)
                    }
                }
            }
            // 如果内存缓存未命中，从磁盘加载长期记忆
            if (result.size < limit) {
                val stillNeeded = matchedIds.filter { it !in resultIds }
                if (stillNeeded.isNotEmpty()) {
                    val longItems = store.loadTier(scope, id, MemoryTier.LONG)
                    longItems.filter { it.id in stillNeeded }.forEach {
                        if (it.id !in resultIds) {
                            result.add(it)
                            resultIds.add(it.id)
                        }
                    }
                }
            }
        }

        // 更新查询缓存
        synchronized(queryCache) {
            if (queryCache.size > 32) {
                queryCache.remove(queryCache.keys.first())
            }
            queryCache[cacheKey] = result.toList()
        }

        return result
    }

    /**
     * 格式化记忆上下文
     */
    private fun formatMemoryContext(memories: List<MemoryItem>): String {
        if (memories.isEmpty()) return ""
        return buildString {
            append("\n=== 关于用户的记忆 ===\n")
            memories.forEach { item ->
                val categoryLabel = when (item.category) {
                    MemoryCategory.FACT -> "事实"
                    MemoryCategory.EMOTION -> "情感"
                    MemoryCategory.PREFERENCE -> "偏好"
                    MemoryCategory.EVENT -> "事件"
                    MemoryCategory.HABIT -> "习惯"
                    MemoryCategory.RELATIONSHIP -> "关系"
                }
                append("【$categoryLabel】${item.content}\n")
            }
        }
    }

    /**
     * 更新记忆访问信息
     */
    private fun touchMemory(item: MemoryItem) {
        val now = System.currentTimeMillis()
        val touched = item.touch(now)

        // 更新短期记忆缓存
        val key = scopeKey(item.scope, item.sourceId)
        shortTermCache[key]?.let { items ->
            synchronized(items) {
                val idx = items.indexOfFirst { it.id == item.id }
                if (idx >= 0) items[idx] = touched
            }
        }

        // 更新中期记忆缓存
        midTermCache[key]?.let { items ->
            synchronized(items) {
                val idx = items.indexOfFirst { it.id == item.id }
                if (idx >= 0) items[idx] = touched
            }
        }

        // 更新索引
        indexCache[key]?.touch(item.id)
    }

    /**
     * 保存记忆
     *
     * @param content 记忆内容
     * @param category 记忆类别
     * @param importance 重要度 [0.0, 1.0]
     * @param source 来源
     * @param sourceId 来源ID（companionId 或 groupId）
     * @param scope 作用域
     * @return 记忆ID（null表示保存失败或被去重合并）
     */
    suspend fun saveMemory(
        content: String,
        category: MemoryCategory,
        importance: Float,
        source: MemorySource,
        sourceId: Long,
        scope: MemoryScope
    ): String? {
        val key = scopeKey(scope, sourceId)
        return getLock(scope, sourceId).withLock {
            runCatching {
                // 去重检查
                val existing = findSimilar(key, content)
                if (existing != null) {
                    // 合并：更新重要度和访问次数
                    val merged = existing.copy(
                        importance = maxOf(existing.importance, importance),
                        accessCount = existing.accessCount + 1,
                        lastAccessed = System.currentTimeMillis()
                    )
                    updateMemoryInternal(scope, sourceId, merged)
                    return@runCatching existing.id
                }

                // 创建新记忆
                val now = System.currentTimeMillis()
                val item = MemoryItem(
                    id = UUID.randomUUID().toString(),
                    content = content,
                    category = category,
                    importance = importance.coerceIn(0f, 1f),
                    timestamp = now,
                    lastAccessed = now,
                    source = source,
                    sourceId = sourceId,
                    scope = scope,
                    tags = MemoryTokenizer.extractKeywords(content),
                    expireAt = if (scope == MemoryScope.GLOBAL) null else now + SHORT_TERM_TTL_MS,
                    tier = MemoryTier.SHORT
                )

                // 添加到短期记忆
                shortTermCache.computeIfAbsent(key) { mutableListOf() }
                    .let { items ->
                        synchronized(items) {
                            items.add(item)
                            // 超容量时晋级最旧的到中期
                            if (items.size > SHORT_TERM_MAX_PER_SCOPE) {
                                val toPromote = items.removeAt(0)
                                promoteToMid(scope, sourceId, toPromote)
                            }
                        }
                    }

                // 更新索引
                indexCache.computeIfAbsent(key) { MemoryIndex() }.add(item)

                // 同步到全局池（如果符合条件）
                if (scope != MemoryScope.GLOBAL && shouldSyncToGlobal(category, importance)) {
                    syncToGlobal(item)
                }

                // 异步持久化
                schedulePersist(scope, sourceId)

                // [R9 FIX] 写入后清除查询缓存：原 queryCache 写入后不清除，saveMemory 后同 key
                // 查询返回旧快照，新记忆最多被 32 条查询掩盖。
                invalidateQueryCache()

                item.id
            }.onFailure { Log.e(TAG, "保存记忆失败", it) }
                .getOrNull()
        }
    }

    /**
     * [R9 FIX] 清除查询缓存：在任何写入操作后调用，保证后续查询读到最新数据。
     */
    private fun invalidateQueryCache() {
        synchronized(queryCache) {
            queryCache.clear()
        }
    }

    /**
     * 查找相似记忆（用于去重）
     */
    private fun findSimilar(key: String, content: String): MemoryItem? {
        val allItems = mutableListOf<MemoryItem>()
        shortTermCache[key]?.let { allItems.addAll(it) }
        midTermCache[key]?.let { allItems.addAll(it) }

        return allItems.firstOrNull { existing ->
            MemoryTokenizer.similarity(existing.content, content) > DEDUP_SIMILARITY_THRESHOLD
        }
    }

    /**
     * 判断是否应同步到全局池
     */
    private fun shouldSyncToGlobal(category: MemoryCategory, importance: Float): Boolean {
        if (importance < SYNC_IMPORTANCE_THRESHOLD) return false
        // EMOTION 和 EVENT 不同步到全局（角色/会话特定）
        return category !in setOf(MemoryCategory.EMOTION, MemoryCategory.EVENT)
    }

    /**
     * 同步到全局池
     */
    private suspend fun syncToGlobal(item: MemoryItem) {
        val globalItem = item.copy(
            id = UUID.randomUUID().toString(),
            scope = MemoryScope.GLOBAL,
            sourceId = 0L,
            tier = MemoryTier.MID,
            expireAt = null // 全局记忆不过期
        )

        val globalKey = scopeKey(MemoryScope.GLOBAL, 0L)
        shortTermCache.computeIfAbsent(globalKey) { mutableListOf() }
            .let { items ->
                synchronized(items) { items.add(globalItem) }
            }
        indexCache.computeIfAbsent(globalKey) { MemoryIndex() }.add(globalItem)
        schedulePersist(MemoryScope.GLOBAL, 0L)
    }

    /**
     * 短期记忆晋级到中期
     */
    private fun promoteToMid(scope: MemoryScope, id: Long, item: MemoryItem) {
        val key = scopeKey(scope, id)
        val promoted = item.copy(
            tier = MemoryTier.MID,
            expireAt = null
        )
        midTermCache.computeIfAbsent(key) { mutableListOf() }
            .let { items ->
                synchronized(items) { items.add(promoted) }
            }
        schedulePersist(scope, id)
    }

    /**
     * 中期记忆晋级到长期
     */
    private suspend fun promoteToLong(scope: MemoryScope, id: Long, item: MemoryItem) {
        val key = scopeKey(scope, id)
        val promoted = item.copy(tier = MemoryTier.LONG)

        // 从中期缓存移除
        midTermCache[key]?.let { items ->
            synchronized(items) { items.removeAll { it.id == item.id } }
        }

        // 写入长期存储
        val longItems = store.loadTier(scope, id, MemoryTier.LONG).toMutableList()
        longItems.add(promoted)
        store.saveTier(scope, id, MemoryTier.LONG, longItems)

        // 更新长期索引
        longTermIndex.computeIfAbsent(key) { MemoryIndex() }.add(promoted)
        indexCache[key]?.add(promoted)

        schedulePersist(scope, id)
    }

    /**
     * 更新记忆（内部）
     */
    private fun updateMemoryInternal(scope: MemoryScope, sourceId: Long, item: MemoryItem) {
        val key = scopeKey(scope, sourceId)

        // 更新短期记忆
        shortTermCache[key]?.let { items ->
            synchronized(items) {
                val idx = items.indexOfFirst { it.id == item.id }
                if (idx >= 0) items[idx] = item
            }
        }
        // 更新中期记忆
        midTermCache[key]?.let { items ->
            synchronized(items) {
                val idx = items.indexOfFirst { it.id == item.id }
                if (idx >= 0) items[idx] = item
            }
        }

        // 检查是否应晋级到长期
        if (item.shouldPromoteToLong() && item.tier != MemoryTier.LONG) {
            ioScope.launch { promoteToLong(scope, sourceId, item) }
        }

        schedulePersist(scope, sourceId)
        // [R9 FIX] 更新后也清除查询缓存
        invalidateQueryCache()
    }

    /**
     * 调度异步持久化
     */
    private fun schedulePersist(scope: MemoryScope, id: Long) {
        val key = scopeKey(scope, id)
        ioScope.launch {
            runCatching {
                val shortItems = shortTermCache[key]?.toList() ?: emptyList()
                val midItems = midTermCache[key]?.toList() ?: emptyList()

                store.saveTier(scope, id, MemoryTier.SHORT, shortItems)
                store.saveTier(scope, id, MemoryTier.MID, midItems)

                indexCache[key]?.let { index ->
                    store.saveIndex(scope, id, index.serialize())
                }
            }.onFailure { Log.e(TAG, "持久化失败 scope=$key", it) }
        }
    }

    /**
     * 获取作用域下所有记忆（用于UI展示）
     */
    suspend fun getMemories(scope: MemoryScope, id: Long): List<MemoryItem> {
        val key = scopeKey(scope, id)
        val result = mutableListOf<MemoryItem>()

        shortTermCache[key]?.let { result.addAll(it) }
        midTermCache[key]?.let { result.addAll(it) }

        // 加载长期记忆
        result.addAll(store.loadTier(scope, id, MemoryTier.LONG))

        return result.sortedByDescending { it.timestamp }
    }

    /**
     * 删除记忆
     */
    suspend fun deleteMemory(id: String) {
        // 在所有作用域中查找并删除
        listOf(MemoryScope.GLOBAL to 0L).forEach { (scope, sid) ->
            deleteMemoryFromScope(scope, sid, id)
        }
    }

    /**
     * 从指定作用域删除记忆
     */
    private suspend fun deleteMemoryFromScope(scope: MemoryScope, sourceId: Long, id: String) {
        val key = scopeKey(scope, sourceId)
        var deleted = false

        shortTermCache[key]?.let { items ->
            synchronized(items) { deleted = items.removeAll { it.id == id } || deleted }
        }
        midTermCache[key]?.let { items ->
            synchronized(items) { deleted = items.removeAll { it.id == id } || deleted }
        }
        indexCache[key]?.remove(id)

        if (deleted) {
            schedulePersist(scope, sourceId)
            // [R9 FIX] 删除后也清除查询缓存
            invalidateQueryCache()
        }

        // 也检查长期记忆文件
        val longItems = store.loadTier(scope, sourceId, MemoryTier.LONG).toMutableList()
        if (longItems.removeAll { it.id == id }) {
            store.saveTier(scope, sourceId, MemoryTier.LONG, longItems)
            longTermIndex[key]?.remove(id)
        }
    }

    /**
     * 删除整个作用域的所有记忆
     */
    suspend fun deleteAllMemories(scope: MemoryScope, id: Long) {
        val key = scopeKey(scope, id)
        shortTermCache.remove(key)
        midTermCache.remove(key)
        indexCache.remove(key)
        longTermIndex.remove(key)
        store.deleteScope(scope, id)
    }

    /**
     * 从对话中提取并保存记忆
     *
     * @param userInput 用户输入
     * @param aiResponse AI回复
     * @param companionId 角色ID
     * @param groupId 群聊ID（群聊时传入）
     */
    override suspend fun extractAndSaveFromConversation(
        userInput: String,
        aiResponse: String,
        companionId: Long,
        groupId: Long?
    ) {
        runCatching {
            val scope = if (groupId != null) MemoryScope.GROUP else MemoryScope.COMPANION
            val sourceId = if (groupId != null) groupId else companionId
            val source = if (groupId != null) MemorySource.GROUP_CHAT else MemorySource.CHAT

            val extracted = extractMemories(userInput)
            extracted.forEach { (content, category, importance) ->
                saveMemory(content, category, importance, source, sourceId, scope)
            }
        }.onFailure { Log.e(TAG, "提取记忆失败", it) }
    }

    /**
     * 基于关键词的记忆提取
     * 改进版：支持中文，覆盖更多类别
     */
    private fun extractMemories(text: String): List<Triple<String, MemoryCategory, Float>> {
        val result = mutableListOf<Triple<String, MemoryCategory, Float>>()

        // 事实类
        val factPatterns = listOf("我叫", "我是", "我来自", "我在", "我的名字", "我住", "我工作", "我学", "我的职业")
        factPatterns.forEach { pattern ->
            if (text.contains(pattern)) {
                extractAfterPattern(text, pattern)?.let {
                    result.add(Triple(it, MemoryCategory.FACT, 0.8f))
                }
            }
        }

        // 偏好类
        val preferencePatterns = listOf("我喜欢", "我爱好", "我偏爱", "我讨厌", "我不喜欢", "我反感", "我爱", "我恨")
        preferencePatterns.forEach { pattern ->
            if (text.contains(pattern)) {
                extractAfterPattern(text, pattern)?.let {
                    result.add(Triple(it, MemoryCategory.PREFERENCE, 0.75f))
                }
            }
        }

        // 习惯类
        val habitPatterns = listOf("我每天", "我经常", "我总是", "我通常", "我习惯", "我一般")
        habitPatterns.forEach { pattern ->
            if (text.contains(pattern)) {
                extractAfterPattern(text, pattern)?.let {
                    result.add(Triple(it, MemoryCategory.HABIT, 0.7f))
                }
            }
        }

        // 关系类
        val relationshipPatterns = listOf("我的朋友", "我的家人", "我的父母", "我的同学", "我的同事", "我的男朋友", "我的女朋友")
        relationshipPatterns.forEach { pattern ->
            if (text.contains(pattern)) {
                extractAfterPattern(text, pattern)?.let {
                    result.add(Triple(it, MemoryCategory.RELATIONSHIP, 0.8f))
                }
            }
        }

        return result.distinctBy { it.first }
    }

    /**
     * 从模式后提取内容（到句号或换行）
     */
    private fun extractAfterPattern(text: String, pattern: String): String? {
        val idx = text.indexOf(pattern)
        if (idx < 0) return null
        val start = idx + pattern.length
        val endText = text.substring(start)
        val endIdx = endText.indexOfFirst { it in "。，！？；\n" }
        val content = if (endIdx > 0) endText.substring(0, endIdx) else endText.take(50)
        val full = "$pattern$content".trim()
        return if (full.length > pattern.length + 1) full else null
    }
}`

// SourceName: feature__notification__src__main__java__com__ref_b__ai__feature__notification__AiReplyWorker.kt
// SourceSet: ref_b
const RawFeatureNotificationAiReplyWorker = `package com.ref_b.ai.feature.notification

import android.content.Context
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.Data
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import com.ref_b.ai.common.AppForegroundTracker
import com.ref_b.ai.common.DeviceIdProvider
import com.ref_b.ai.database.AppDatabase
import com.ref_b.ai.database.model.ChatMessage
import com.ref_b.ai.database.model.MessageType
import com.ref_b.ai.database.repository.ChatRepository
import com.ref_b.ai.database.repository.CompanionRepository
import com.ref_b.ai.database.repository.MemoryRepository
import com.ref_b.ai.database.repository.ChatMessageCrypto
import com.ref_b.ai.database.repository.filterDecrypted
import com.ref_b.ai.domain.AiChatMessage
import com.ref_b.ai.domain.AiCompanionInfo
import com.ref_b.ai.domain.AiMessageType
import com.ref_b.ai.domain.AiServiceProvider
import com.ref_b.ai.domain.ServiceRegistry
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class AiReplyWorker(
    context: Context,
    params: WorkerParameters
) : CoroutineWorker(context, params) {

    private val aiServiceProvider: AiServiceProvider by lazy {
        ServiceRegistry.get(AiServiceProvider::class.java)
            ?: throw IllegalStateException("AiServiceProvider not registered")
    }

    override suspend fun doWork(): Result = withContext(Dispatchers.IO) {
        try {
            val companionId = inputData.getLong(KEY_COMPANION_ID, -1L)
            val userMessageContent = inputData.getString(KEY_USER_MESSAGE) ?: ""

            if (companionId == -1L || userMessageContent.isBlank()) {
                return@withContext Result.failure()
            }

            // 封禁检查：已封禁用户不触发 AI 回复
            if (com.ref_b.ai.common.BanManager.isBanned(applicationContext)) {
                return@withContext Result.failure()
            }

            // 防御性输入安全检查
            val inputCheck = com.ref_b.ai.common.ContentFilter.checkInput(userMessageContent)
            if (inputCheck.isViolating) {
                com.ref_b.ai.common.BanManager.recordViolation(applicationContext, inputCheck.level)
                return@withContext Result.failure()
            }

            val database = AppDatabase.getDatabase(applicationContext)
            val companionRepository = CompanionRepository(database.companionDao())
            val chatRepository = ChatRepository(database.chatMessageDao())
            val memoryRepository = MemoryRepository(database.memoryDao(), DeviceIdProvider.getDeviceId(applicationContext))

            try {
                val companionModel = companionRepository.getCompanionById(companionId)
                if (companionModel == null) {
                    return@withContext Result.failure()
                }

                val history = chatRepository.getRecentMessagesSync(companionId, limit = 50)
                    .map { ChatMessageCrypto.decryptFromStorage(it) }
                    .filterDecrypted()

                val response = aiServiceProvider.sendMessage(
                    companionModel.toAiCompanionInfo(),
                    history.toAiChatMessages()
                )
                val trimmedResponse = response.content.trim()

                if (trimmedResponse.isNotEmpty()) {
                    // 安全检查：拦截 AI 违规输出
                    val outputSafety = com.ref_b.ai.common.ContentFilter.checkOutputSafety(trimmedResponse)
                    val safeResponse = if (!outputSafety.isSafe) {
                        android.util.Log.w("AiReplyWorker", "AI output blocked by safety filter: ${outputSafety.reason}")
                        com.ref_b.ai.common.BanManager.recordViolation(applicationContext, outputSafety.level)
                        "抱歉，我无法回应这个话题。"
                    } else {
                        trimmedResponse
                    }

                    val aiMessage = ChatMessage(
                        companionId = companionId,
                        content = safeResponse,
                        isFromUser = false
                    )
                    chatRepository.sendMessage(aiMessage)
                    companionRepository.updateTimestamp(companionId)
                    companionRepository.increaseIntimacy(companionId, 2)

                    memoryRepository.extractAndSaveMemories(companionId, userMessageContent, safeResponse)

                    if (!AppForegroundTracker.isInForeground) {
                        val notificationPreview = if (safeResponse.length > 50) {
                            safeResponse.take(50) + "..."
                        } else safeResponse
                        NotificationHelper.showCompanionMessageNotification(
                            applicationContext,
                            companionModel.name,
                            notificationPreview,
                            companionId
                        )
                    }
                }
            } finally {
            }

            Result.success()
        } catch (e: Exception) {
            Result.retry()
        }
    }

    private fun com.ref_b.ai.database.model.CompanionEntity.toAiCompanionInfo() = AiCompanionInfo(
        id = id, name = name, personality = personality,
        age = age, backstory = backstory, speakingStyle = speakingStyle,
        systemPrompt = systemPrompt
    )

    private fun ChatMessage.toAiChatMessage() = AiChatMessage(
        isFromUser = isFromUser, content = content, timestamp = timestamp,
        type = when (type) {
            MessageType.IMAGE -> AiMessageType.IMAGE
            else -> AiMessageType.TEXT
        },
        companionId = companionId
    )

    private fun List<ChatMessage>.toAiChatMessages() = map { it.toAiChatMessage() }

    companion object {
        private const val WORK_NAME_PREFIX = "ai_reply_"
        const val KEY_COMPANION_ID = "companion_id"
        const val KEY_USER_MESSAGE = "user_message"

        private val networkConstraints = Constraints.Builder()
            .setRequiredNetworkType(NetworkType.CONNECTED)
            .build()

        fun enqueue(context: Context, companionId: Long, userMessage: String) {
            val inputData = Data.Builder()
                .putLong(KEY_COMPANION_ID, companionId)
                .putString(KEY_USER_MESSAGE, userMessage)
                .build()

            val workRequest = OneTimeWorkRequestBuilder<AiReplyWorker>()
                .setInputData(inputData)
                .setConstraints(networkConstraints)
                .build()

            WorkManager.getInstance(context).enqueueUniqueWork(
                "$WORK_NAME_PREFIX$companionId",
                ExistingWorkPolicy.APPEND_OR_REPLACE,
                workRequest
            )
        }
    }
}`

// SourceName: feature__notification__src__main__java__com__ref_b__ai__feature__notification__CompanionMessageWorker.kt
// SourceSet: ref_b
const RawFeatureNotificationCompanionMessageWorker = `package com.ref_b.ai.feature.notification

import android.content.Context
import android.content.Intent
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import com.ref_b.ai.common.wechat.WeChatBroadcast
import com.ref_b.ai.database.AppDatabase
import com.ref_b.ai.database.model.ChatMessage
import com.ref_b.ai.database.model.MessageType
import com.ref_b.ai.database.repository.ChatMessageCrypto
import com.ref_b.ai.database.repository.filterDecrypted
import com.ref_b.ai.domain.AiServiceProvider
import com.ref_b.ai.domain.AiCompanionInfo
import com.ref_b.ai.domain.AiChatMessage
import com.ref_b.ai.domain.AiMessageType
import com.ref_b.ai.domain.ProactiveMessageSettings
import com.ref_b.ai.domain.ServiceRegistry
import com.ref_b.ai.common.AppForegroundTracker
import com.ref_b.ai.common.BanManager
import com.ref_b.ai.common.ChatDetailSettingsDataStoreProvider
import com.ref_b.ai.common.SecureLog
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.longPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.util.concurrent.TimeUnit
import kotlin.random.Random

/** 仅提取主动消息相关字段，避免跨 feature 依赖完整设置类。
 * 字段必须与 feature:chat 的 [CompanionChatDetailSettings] 中同名字段保持对齐，
 * 否则反序列化时 ignoreUnknownKeys=true 会静默丢弃。 */
@Serializable
data class ProactiveSettings(
    val proactiveEnabled: Boolean = true,
    /** 用户手动输入的间隔（分钟），优先使用；UI 可编辑范围 30~1440 */
    val proactiveIntervalMinutes: Int = 180,
    val proactiveMinIntervalMinutes: Int = 60,
    val proactiveMaxIntervalMinutes: Int = 720,
    val proactiveDailyLimit: Int = 6,
    /** 是否允许 AI 主动开启新话题 */
    val allowNewTopic: Boolean = true,
    /** 是否允许在主动消息后追加追问句 */
    val allowFollowUpMessage: Boolean = true,
    val doNotDisturbEnabled: Boolean = false,
    val dndStartMinutes: Int = 23 * 60,
    val dndEndMinutes: Int = 8 * 60,
    val allowLateNightMessage: Boolean = false,
    val allowPriorityMessageInDnd: Boolean = false,
    val blocked: Boolean = false
)

class CompanionMessageWorker(
    private val context: Context,
    params: WorkerParameters
) : CoroutineWorker(context, params) {

    private val aiServiceProvider: AiServiceProvider by lazy {
        ServiceRegistry.get(AiServiceProvider::class.java)
            ?: throw IllegalStateException("AiServiceProvider not registered in ServiceRegistry")
    }

    // 直接读取 chat_detail_settings DataStore，避免跨 feature 依赖
    private val json = Json { ignoreUnknownKeys = true }

    override suspend fun doWork(): Result = withContext(Dispatchers.IO) {
        try {
            if (BanManager.isBanned(context)) {
                SecureLog.d("CompanionMessageWorker", "User is banned, skip proactive message")
                return@withContext Result.success()
            }

            val database = AppDatabase.getDatabase(context)
            val companionDao = database.companionDao()
            val chatMessageDao = database.chatMessageDao()

            val companions = companionDao.getAllCompanionsSync()
            if (companions.isEmpty()) return@withContext Result.success()

            // ── 筛选启用主动消息且未屏蔽的伴侣 ──
            val eligibleCompanions = companions.filter { companion ->
                runCatching { readCompanionSettings(companion.id) }.getOrNull()?.let { settings ->
                    settings.proactiveEnabled && !settings.blocked
                } ?: false
            }

            if (eligibleCompanions.isEmpty()) {
                SecureLog.d("CompanionMessageWorker", "No eligible companions (all disabled/blocked), reschedule")
                scheduleNext(context, null)
                return@withContext Result.success()
            }

            // 从符合条件的伴侣中随机选一个
            val randomCompanion = eligibleCompanions.random()
            val settings = runCatching { readCompanionSettings(randomCompanion.id) }.getOrNull()
                ?: ProactiveSettings()

            // ── 免打扰检查 ──
            if (settings.doNotDisturbEnabled && !settings.allowPriorityMessageInDnd) {
                val now = java.util.Calendar.getInstance()
                val totalMinutes = now.get(java.util.Calendar.HOUR_OF_DAY) * 60 + now.get(java.util.Calendar.MINUTE)
                val inDndRange = if (settings.dndStartMinutes > settings.dndEndMinutes) {
                    // 跨午夜：如 23:00 ~ 08:00
                    totalMinutes >= settings.dndStartMinutes || totalMinutes < settings.dndEndMinutes
                } else {
                    totalMinutes in settings.dndStartMinutes until settings.dndEndMinutes
                }
                if (inDndRange && !settings.allowLateNightMessage) {
                    SecureLog.d("CompanionMessageWorker", "DND active for ${randomCompanion.name}, skip")
                    scheduleNext(context, settings)
                    return@withContext Result.success()
                }
            }

            // ── 每日上限检查（精确计数，跨天自动重置） ──
            if (settings.proactiveDailyLimit > 0) {
                val todayCount = getTodayProactiveCount(context, randomCompanion.id)
                if (todayCount >= settings.proactiveDailyLimit) {
                    SecureLog.d("CompanionMessageWorker", "Daily limit reached ($todayCount/${settings.proactiveDailyLimit}) for ${randomCompanion.name}")
                    scheduleNext(context, settings)
                    return@withContext Result.success()
                }
            }

            val recentMessages = chatMessageDao.getRecentMessagesSync(randomCompanion.id, 10)
                .map { ChatMessageCrypto.decryptFromStorage(it) }
                .filterDecrypted()

            // 传入自定义设置，让 shouldProactivelyMessage/generateProactiveMessage 按其行为
            val domainSettings = settings.toDomain()
            if (!aiServiceProvider.shouldProactivelyMessage(randomCompanion.toAiCompanionInfo(), recentMessages.toAiChatMessages(), domainSettings)) {
                scheduleNext(context, settings)
                return@withContext Result.success()
            }

            val messageContent = aiServiceProvider.generateProactiveMessage(randomCompanion.toAiCompanionInfo(), recentMessages.toAiChatMessages(), domainSettings)

            if (messageContent == null) {
                SecureLog.w("CompanionMessageWorker", "Proactive message is null, skipping")
                scheduleNext(context, settings)
                return@withContext Result.success()
            }

            // 安全检查：拦截 AI 主动消息中的违规内容（仅最终防线，不累计封禁）
            // AiService 生成时已做过 checkOutputSafety，这里只做兜底
            val outputSafety = com.ref_b.ai.common.ContentFilter.checkOutputSafety(messageContent)
            if (!outputSafety.isSafe) {
                SecureLog.w("CompanionMessageWorker", "Proactive message blocked by safety filter: ${outputSafety.reason}")
                // AI 输出违规不应累加用户封禁（见 Bug #1 根因 A3）
                scheduleNext(context, settings)
                return@withContext Result.success()
            }

            // ── 分段发送：按自然句号/感叹号/问号/换行拆分，逐条入库+广播 ──
            val segments = splitIntoSegments(messageContent)
            var totalSegmentsSent = 0

            for ((index, segment) in segments.withIndex()) {
                if (index > 0) {
                    // 段间延迟 1~2 秒，模拟真人分段打字效果
                    delay(Random.nextLong(1000L, 2000L))
                }

                val message = ChatMessage(
                    companionId = randomCompanion.id,
                    content = segment,
                    isFromUser = false
                )
                val messageId = chatMessageDao.insertMessage(ChatMessageCrypto.encryptForStorage(message))
                broadcastProactiveWeChatMessage(randomCompanion.id, messageId)
                totalSegmentsSent++
            }

            // 精确增加今日主动消息计数
            if (totalSegmentsSent > 0 && settings.proactiveDailyLimit > 0) {
                incrementTodayProactiveCount(context, randomCompanion.id, 1)
            }

            // 仅在最后一条段时推送通知，避免通知轰炸
            if (!AppForegroundTracker.isInForeground && segments.isNotEmpty()) {
                NotificationHelper.showCompanionMessageNotification(
                    context,
                    randomCompanion.name,
                    segments.first(), // 通知显示第一条即可
                    randomCompanion.id
                )
            }

            scheduleNext(context, settings)

            Result.success()
        } catch (_: Exception) {
            Result.retry()
        }
    }

    /**
     * 将长文本拆分为自然短段落。
     * 按中文句号/感叹号/问号/换行分割，每段不超过60字。
     */
    private fun splitIntoSegments(text: String): List<String> {
        val trimmed = text.trim()
        if (trimmed.length <= 60) return listOf(trimmed)

        // 按句子边界分割
        val sentenceParts = trimmed.split(Regex("(?<=[。！？!?\n])"))
        val segments = mutableListOf<String>()
        var currentSegment = StringBuilder()

        for (part in sentenceParts) {
            val candidate = if (currentSegment.isEmpty()) part else "$currentSegment$part"
            if (candidate.length <= 60) {
                currentSegment = StringBuilder(candidate)
            } else {
                if (currentSegment.isNotEmpty()) {
                    segments.add(currentSegment.toString().trim())
                }
                currentSegment = StringBuilder(part)
            }
        }
        if (currentSegment.isNotEmpty()) {
            segments.add(currentSegment.toString().trim())
        }

        // 兜底：如果某段仍然过长，强制按字数截断
        return segments.flatMap { seg ->
            if (seg.length <= 60) listOf(seg) else seg.chunked(60)
        }.filter { it.isNotBlank() }
    }

    private fun broadcastProactiveWeChatMessage(companionId: Long, messageId: Long) {
        val intent = Intent(WeChatBroadcast.ACTION_SEND_PROACTIVE).apply {
            setPackage(context.packageName)
            putExtra(WeChatBroadcast.EXTRA_COMPANION_ID, companionId)
            putExtra(WeChatBroadcast.EXTRA_MESSAGE_ID, messageId)
        }
        try {
            if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
                context.applicationContext.sendBroadcast(intent, null)
            } else {
                context.applicationContext.sendBroadcast(intent)
            }
        } catch (e: Exception) {
            SecureLog.w("CompanionMessageWorker", "Failed to send broadcast: ${e.message}")
        }
    }

    /**
     * 从 DataStore 读取指定伴侣的主动消息相关设置。
     * 直接读取与 ChatDetailSettingsStore 共享的同一 DataStore，避免跨 feature 依赖。
     */
    private suspend fun readCompanionSettings(companionId: Long): ProactiveSettings? {
        return runCatching {
            val dataStore = ChatDetailSettingsDataStoreProvider.get(context)
            val prefs = dataStore.data.first()
            val raw = prefs[stringPreferencesKey("companion_chat_detail_settings_map")] ?: return@runCatching null
            val settingsMap: Map<Long, ProactiveSettings> = json.decodeFromString(raw)
            settingsMap[companionId]
        }.getOrNull()
    }

    // ── 领域类型转换辅助 ──

    /** 将 Worker 侧 [ProactiveSettings] 映射为 domain 层 [ProactiveMessageSettings] */
    private fun ProactiveSettings.toDomain() = ProactiveMessageSettings(
        proactiveEnabled = proactiveEnabled,
        proactiveIntervalMinutes = proactiveIntervalMinutes,
        proactiveMinIntervalMinutes = proactiveMinIntervalMinutes,
        proactiveMaxIntervalMinutes = proactiveMaxIntervalMinutes,
        proactiveDailyLimit = proactiveDailyLimit,
        allowNewTopic = allowNewTopic,
        allowFollowUpMessage = allowFollowUpMessage,
        doNotDisturbEnabled = doNotDisturbEnabled,
        dndStartMinutes = dndStartMinutes,
        dndEndMinutes = dndEndMinutes,
        allowLateNightMessage = allowLateNightMessage,
        allowPriorityMessageInDnd = allowPriorityMessageInDnd,
        blocked = blocked
    )

    private fun com.ref_b.ai.database.model.CompanionEntity.toAiCompanionInfo() = AiCompanionInfo(
        id = id, name = name, personality = personality,
        age = age, backstory = backstory, speakingStyle = speakingStyle,
        systemPrompt = systemPrompt
    )

    private fun com.ref_b.ai.database.model.ChatMessage.toAiChatMessage() = AiChatMessage(
        isFromUser = isFromUser, content = content, timestamp = timestamp,
        type = when (type) {
            MessageType.IMAGE -> AiMessageType.IMAGE
            else -> AiMessageType.TEXT
        },
        companionId = companionId
    )

    private fun List<com.ref_b.ai.database.model.ChatMessage>.toAiChatMessages() = map { it.toAiChatMessage() }

    companion object {
        private const val WORK_NAME = "companion_message_work"

        /** 默认间隔兜底（当设置读取失败时使用） */
        private const val FALLBACK_MIN_MINUTES = 30L
        private const val FALLBACK_MAX_MINUTES = 120L

        private const val DAILY_COUNT_PREFS = "proactive_daily_count"

        /** 获取指定伴侣今日已发主动消息数（精确计数，不取近似） */
        private fun getTodayProactiveCount(context: Context, companionId: Long): Int {
            val prefs = context.getSharedPreferences(DAILY_COUNT_PREFS, Context.MODE_PRIVATE)
            val today = todayKey()
            val storedDate = prefs.getString("date_$companionId", null)
            return if (storedDate == today) prefs.getInt("count_$companionId", 0) else 0
        }

        /** 增加指定伴侣今日主动消息计数 */
        private fun incrementTodayProactiveCount(context: Context, companionId: Long, delta: Int) {
            val prefs = context.getSharedPreferences(DAILY_COUNT_PREFS, Context.MODE_PRIVATE)
            val today = todayKey()
            val count = getTodayProactiveCount(context, companionId) + delta
            prefs.edit()
                .putString("date_$companionId", today)
                .putInt("count_$companionId", count)
                .apply()
        }

        private fun todayKey(): String {
            val cal = java.util.Calendar.getInstance()
            return "${cal.get(java.util.Calendar.YEAR)}-${cal.get(java.util.Calendar.DAY_OF_YEAR)}"
        }

        private val networkConstraints = Constraints.Builder()
            .setRequiredNetworkType(NetworkType.CONNECTED)
            .build()

        /**
         * 外部入口：首次调度，使用默认间隔。
         * 使用 enqueueUniqueWork + REPLACE 确保只保留最新一次调度，
         * 消除 KeepAliveService 15min 心跳 + MainActivity 启动反复 schedule 导致的请求堆叠。
         */
        fun schedule(context: Context) {
            val delayMinutes = Random.nextInt(FALLBACK_MIN_MINUTES.toInt(), FALLBACK_MAX_MINUTES.toInt())

            val workRequest = OneTimeWorkRequestBuilder<CompanionMessageWorker>()
                .setConstraints(networkConstraints)
                .setInitialDelay(delayMinutes.toLong(), TimeUnit.MINUTES)
                .build()

            WorkManager.getInstance(context).enqueueUniqueWork(
                WORK_NAME,
                ExistingWorkPolicy.REPLACE,
                workRequest
            )
        }

        /**
         * 后续调度：优先使用用户手动输入的 [ProactiveSettings.proactiveIntervalMinutes]，
         * 未设置时回退到 min/max 区间随机。
         */
        private fun scheduleNext(context: Context, settings: ProactiveSettings?) {
            val delayMinutes = if (settings != null && settings.proactiveIntervalMinutes > 0) {
                // 用户手动输入的间隔优先，确保 ≥15 分钟，最大 1440 分钟（24h）
                settings.proactiveIntervalMinutes.coerceIn(15, 1440).toLong()
            } else {
                val minInterval = settings?.proactiveMinIntervalMinutes?.coerceAtLeast(15) ?: FALLBACK_MIN_MINUTES.toInt()
                val maxInterval = settings?.proactiveMaxIntervalMinutes?.coerceAtLeast(minInterval + 1) ?: FALLBACK_MAX_MINUTES.toInt()
                Random.nextInt(minInterval, maxInterval + 1).toLong()
            }

            val workRequest = OneTimeWorkRequestBuilder<CompanionMessageWorker>()
                .setConstraints(networkConstraints)
                .setInitialDelay(delayMinutes, TimeUnit.MINUTES)
                .build()

            WorkManager.getInstance(context).enqueueUniqueWork(
                WORK_NAME,
                ExistingWorkPolicy.REPLACE,
                workRequest
            )
        }

        fun cancel(context: Context) {
            WorkManager.getInstance(context).cancelUniqueWork(WORK_NAME)
        }
    }
}`

// SourceName: feature__qqbot__src__main__java__com__ref_b__ai__feature__qqbot__data__QQBotChatBridge.kt
// SourceSet: ref_b
const RawFeatureQqbotDataQQBotChatBridge = `package com.ref_b.ai.feature.qqbot.data

import android.content.Context
import com.ref_b.ai.common.DeviceIdProvider
import com.ref_b.ai.database.AppDatabase
import com.ref_b.ai.database.model.ChatMessage
import com.ref_b.ai.database.model.MessageType
import com.ref_b.ai.database.repository.ChatRepository
import com.ref_b.ai.database.repository.CompanionRepository
import com.ref_b.ai.database.repository.MemoryRepository
import com.ref_b.ai.database.repository.filterDecrypted
import com.ref_b.ai.domain.AiChatMessage
import com.ref_b.ai.domain.AiCompanionInfo
import com.ref_b.ai.domain.AiMessageType
import com.ref_b.ai.domain.AiResponse
import com.ref_b.ai.domain.AiServiceProvider
import com.ref_b.ai.domain.ServiceRegistry
import com.ref_b.ai.feature.qqbot.data.model.QQInboundEvent
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class QQBotChatBridge(
    private val context: Context,
    private val qqBotRepository: QQBotMessageRepository,
    private val tokenStore: QQBotTokenStore
) {
    private val database = AppDatabase.getDatabase(context)
    private val deviceId = DeviceIdProvider.getDeviceId(context)
    private val chatRepository = ChatRepository(database.chatMessageDao())
    private val companionRepository = CompanionRepository(database.companionDao())
    private val memoryRepository = MemoryRepository(database.memoryDao(), deviceId)
    private val mappingManager = QQBotUserMappingManager(tokenStore, companionRepository)
    private val aiServiceProvider: AiServiceProvider by lazy {
        ServiceRegistry.get(AiServiceProvider::class.java)
            ?: throw IllegalStateException("AiServiceProvider not registered in ServiceRegistry")
    }
    private val bridgeJob = SupervisorJob()
    private val bridgeScope = CoroutineScope(bridgeJob + Dispatchers.IO)

    private var eventCollectionJob: kotlinx.coroutines.Job? = null

    /**
     * 在 Service 中启动持续的事件监听与自动回复。
     * 与 UI 生命周期解耦，保证切到后台后仍能自动回复。
     */
    fun start() {
        if (eventCollectionJob?.isActive == true) {
            android.util.Log.d("QQBotBridge", "Already started")
            return
        }
        android.util.Log.i("QQBotBridge", "Starting event collection")
        eventCollectionJob = bridgeScope.launch {
            qqBotRepository.incomingEvents.collect { event ->
                val autoReply = tokenStore.getAutoReply()
                val text = qqBotRepository.extractText(event)
                android.util.Log.d("QQBotBridge", "Event received, autoReply=$autoReply, text=$text")
                if (!autoReply) return@collect
                val key = qqBotRepository.getReplyKey(event)
                // 连发合并：如果同 key 已有活跃回复 job，取消旧 job，
                // 短暂延迟聚合后续消息（2 秒窗口），用最新事件发起回复。
                val existingJob = qqBotRepository.getActiveReplyJob(key)
                if (existingJob?.isActive == true) {
                    android.util.Log.d("QQBotBridge", "Cancelling active job for $key to merge new message")
                    existingJob.cancel()
                    qqBotRepository.removeActiveReplyJob(key)
                }
                // 等待 2 秒聚合窗口，收到连发消息时会取消当前 job 重新合并
                val currentEventText = text
                val job = bridgeScope.launch {
                    try {
                        delay(2000L) // 连发聚合窗口
                        handleIncomingEventStreaming(event)
                    } finally {
                        qqBotRepository.removeActiveReplyJob(key)
                    }
                }
                qqBotRepository.setActiveReplyJob(key, job)
            }
        }
    }

    fun stop() {
        eventCollectionJob?.cancel()
        eventCollectionJob = null
    }

    /**
     * 流式处理接入事件：使用 [AiServiceProvider.sendMessageStream] 按句分段即时发送，
     * 大幅降低用户感知的首字延迟。每收到完整句子或 ≥30 字立即发送到 QQ，
     * 段间最小间隔 500ms 防止 QQ API 频率限制。
     */
    suspend fun handleIncomingEventStreaming(event: QQInboundEvent) = withContext(Dispatchers.IO) {
        try {
            val qqUserId = when (event) {
                is QQInboundEvent.C2CMessage -> event.userOpenid
                is QQInboundEvent.GroupAtMessage -> "${event.groupOpenid}:${event.memberOpenid}"
                is QQInboundEvent.GuildMessage -> "${event.channelId}:${event.authorId}"
                is QQInboundEvent.DirectMessage -> "${event.guildId}:${event.authorId}"
            }
            android.util.Log.d("QQBotBridge", "Handling streaming event, qqUserId=$qqUserId")
            if (qqUserId.isBlank()) return@withContext

            val text = qqBotRepository.extractText(event)
            android.util.Log.d("QQBotBridge", "Extracted text: $text")
            if (text.isBlank()) return@withContext

            if (com.ref_b.ai.common.BanManager.isBanned(context)) {
                android.util.Log.w("QQBotBridge", "Banned, skip")
                return@withContext
            }

            val companionId = mappingManager.getOrCreateMapping(qqUserId) ?: run {
                android.util.Log.w("QQBotBridge", "No companion mapping for $qqUserId")
                return@withContext
            }
            android.util.Log.d("QQBotBridge", "Mapped to companionId=$companionId")
            val companion = companionRepository.getCompanionById(companionId) ?: run {
                android.util.Log.w("QQBotBridge", "Companion not found: $companionId")
                return@withContext
            }

            val filterResult = com.ref_b.ai.common.ContentFilter.checkInput(text)
            if (filterResult.isViolating) {
                android.util.Log.w("QQBotBridge", "Input blocked: ${filterResult.reason}")
                com.ref_b.ai.common.BanManager.recordViolation(context, filterResult.level)
                val blockedResponse = "抱歉，我无法处理这个话题。"
                sendReply(event, blockedResponse)
                persistBlockedMessage(companionId, blockedResponse)
                return@withContext
            }

            val userMessage = ChatMessage(
                companionId = companionId,
                content = text,
                isFromUser = true,
                timestamp = System.currentTimeMillis()
            )
            chatRepository.sendMessage(userMessage)
            companionRepository.updateTimestamp(companionId)

            val history = chatRepository.getRecentMessagesSync(companionId, limit = 30).filterDecrypted()
            android.util.Log.d("QQBotBridge", "Calling AI with ${history.size} history messages")

            // [R1 FIX] 改为非流式调用：原 sendMessageStream 违反「AI 输入/输出禁止流式」铁律，
            // 且流式分段路径完全没有 ContentFilter.checkOutputSafety 检查，不安全输出直接发到 QQ。
            // 现在全文接收 → 安全检查 → 分段发送。
            val response = try {
                aiServiceProvider.sendMessage(companion.toAiCompanionInfo(), history.toAiChatMessages(), 0)
            } catch (e: Exception) {
                android.util.Log.e("QQBotBridge", "sendMessage failed", e)
                null
            }

            if (response == null || response.content.isBlank()) {
                val fallback = "抱歉，我暂时无法处理这条消息。"
                sendReply(event, fallback)
                persistBlockedMessage(companionId, fallback)
                return@withContext
            }

            // [R1 FIX] 全文安全检查：在发送和入库前对完整 AI 输出做 ContentFilter 校验。
            val outputSafety = com.ref_b.ai.common.ContentFilter.checkOutputSafety(response.content)
            val safeText = if (!outputSafety.isSafe) {
                android.util.Log.w("QQBotBridge", "AI output blocked: ${outputSafety.reason}")
                "抱歉，我无法回应这个话题。"
            } else {
                response.content
            }

            // 按句子边界分段发送（频率保护），但入库存完整回复
            var lastSendTime = 0L
            val minGapMs = 500L
            val sentences = splitIntoSentences(safeText)
            for (sentence in sentences) {
                if (sentence.isBlank()) continue
                val elapsed = System.currentTimeMillis() - lastSendTime
                if (elapsed < minGapMs && lastSendTime > 0) {
                    delay(minGapMs - elapsed)
                }
                if (tokenStore.getForwardEnabled()) {
                    sendReply(event, sentence)
                }
                lastSendTime = System.currentTimeMillis()
            }

            // 入库完整 AI 回复
            val aiMessage = ChatMessage(
                companionId = companionId,
                content = safeText,
                isFromUser = false,
                timestamp = System.currentTimeMillis()
            )
            chatRepository.sendMessage(aiMessage)

            // 增加亲密度 + 异步提取记忆
            companionRepository.increaseIntimacy(companionId, 2)
            bridgeScope.launch {
                runCatching {
                    memoryRepository.extractAndSaveMemories(companionId, text, safeText)
                }
            }

        } catch (e: Exception) {
            android.util.Log.e("QQBotBridge", "Error handling incoming event", e)
        }
    }

    private suspend fun sendReply(event: QQInboundEvent, text: String) {
        val result = qqBotRepository.sendTextMessage(event, text)
        result.onFailure { e ->
            android.util.Log.e("QQBotBridge", "Failed to send QQ reply: ${e.message}", e)
        }
    }

    private suspend fun persistBlockedMessage(companionId: Long, text: String) {
        chatRepository.sendMessage(
            ChatMessage(
                companionId = companionId,
                content = text,
                isFromUser = false,
                timestamp = System.currentTimeMillis()
            )
        )
    }

    private fun cleanReplyText(text: String): String {
        return text.trim()
            .replace(Regex("\\s+"), " ")
            .replace(Regex("^[\\[\\]\\s，。！？、]+"), "")
            .replace(Regex("[\\[\\]\\s，。！？、]+$"), "")
            .trim()
    }

    /**
     * [R1 FIX] 将完整 AI 回复按句子边界拆分为可分次发送的段落。
     * 替代原流式收集的分段逻辑——现在全文接收后再拆分，安全检查已对全文完成。
     */
    private fun splitIntoSentences(text: String): List<String> {
        if (text.isBlank()) return emptyList()
        val delimiters = charArrayOf('。', '！', '？', '!', '?', '\n')
        val result = mutableListOf<String>()
        var start = 0
        while (start < text.length) {
            val idx = text.indexOfAny(delimiters, startIndex = start)
            if (idx < 0) {
                val remaining = text.substring(start).trim()
                if (remaining.isNotEmpty()) result.add(remaining)
                break
            }
            val end = idx + 1
            val sentence = text.substring(start, end).trim()
            if (sentence.isNotEmpty()) result.add(sentence)
            start = end
        }
        return result
    }

    fun close() {
        bridgeJob.cancel()
    }

    private fun com.ref_b.ai.database.model.CompanionEntity.toAiCompanionInfo() = AiCompanionInfo(
        id = id, name = name, personality = personality,
        age = age, backstory = backstory, speakingStyle = speakingStyle,
        systemPrompt = systemPrompt
    )

    private fun com.ref_b.ai.database.model.ChatMessage.toAiChatMessage() = AiChatMessage(
        isFromUser = isFromUser, content = content, timestamp = timestamp,
        type = when (type) {
            MessageType.IMAGE -> AiMessageType.IMAGE
            else -> AiMessageType.TEXT
        },
        companionId = companionId
    )

    private fun List<com.ref_b.ai.database.model.ChatMessage>.toAiChatMessages() = map { it.toAiChatMessage() }
}`

// SourceName: feature__wechat__src__main__java__com__ref_b__ai__feature__wechat__data__WeChatChatBridge.kt
// SourceSet: ref_b
const RawFeatureWechatDataWeChatChatBridge = `package com.ref_b.ai.feature.wechat.data

import android.content.Context
import com.ref_b.ai.common.DeviceIdProvider
import com.ref_b.ai.common.StickerInfo
import com.ref_b.ai.common.StickerManager
import com.ref_b.ai.database.AppDatabase
import com.ref_b.ai.database.model.ChatMessage
import com.ref_b.ai.database.repository.ChatRepository
import com.ref_b.ai.database.repository.CompanionRepository
import com.ref_b.ai.database.repository.MemoryRepository
import com.ref_b.ai.database.repository.filterDecrypted
import com.ref_b.ai.feature.wechat.data.model.M0
import com.ref_b.ai.feature.wechat.data.model.M1
import com.ref_b.ai.feature.wechat.data.model.M1Type
import com.ref_b.ai.feature.wechat.data.model.M2
import com.ref_b.ai.feature.wechat.service.WeChatServiceLocator
import com.ref_b.ai.domain.AiServiceProvider
import com.ref_b.ai.domain.ServiceRegistry
import com.ref_b.ai.domain.AiCompanionInfo
import com.ref_b.ai.domain.AiChatMessage
import com.ref_b.ai.domain.AiMessageType
import com.ref_b.ai.domain.AiResponse
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File

class WeChatChatBridge(
    private val context: Context,
    private val weChatRepository: WeChatMessageRepository
) {
    private val database = AppDatabase.getDatabase(context)
    private val deviceId = DeviceIdProvider.getDeviceId(context)
    private val chatRepository = ChatRepository(database.chatMessageDao())
    private val companionRepository = CompanionRepository(database.companionDao())
    private val memoryRepository = MemoryRepository(database.memoryDao(), deviceId)
    private val tokenStore = WeChatTokenStore(context)
    private val aiServiceProvider: AiServiceProvider by lazy {
        ServiceRegistry.get(AiServiceProvider::class.java)
            ?: throw IllegalStateException("AiServiceProvider not registered in ServiceRegistry")
    }
    private val mappingManager = WeChatUserMappingManager(tokenStore, companionRepository)
    private val bridgeJob = SupervisorJob()
    private val bridgeScope = CoroutineScope(bridgeJob + Dispatchers.IO)

    suspend fun handleIncomingMessage(message: M0): String? = withContext(Dispatchers.IO) {
        try {
            val wechatUserId = message.fromUserId ?: return@withContext null
            val text = extractText(message) ?: return@withContext null
            if (text.isBlank()) return@withContext null

            // 封禁检查
            if (com.ref_b.ai.common.BanManager.isBanned(context)) return@withContext null

            val companionId = mappingManager.getOrCreateMapping(wechatUserId)
                ?: return@withContext null

            val companion = companionRepository.getCompanionById(companionId)
                ?: return@withContext null

            // 安全检查：先检查后入库，避免违规原文持久化
            val filterResult = com.ref_b.ai.common.ContentFilter.checkInput(text)
            if (filterResult.isViolating) {
                android.util.Log.w("WeChatBridge", "Input blocked by safety filter: ${filterResult.reason}")
                com.ref_b.ai.common.BanManager.recordViolation(context, filterResult.level)
                val blockedResponse = "抱歉，我无法处理这个话题。"
                val blockedMsg = ChatMessage(
                    companionId = companionId,
                    content = blockedResponse,
                    isFromUser = false,
                    timestamp = System.currentTimeMillis()
                )
                chatRepository.sendMessage(blockedMsg)
                return@withContext blockedResponse
            }

            val userMessage = ChatMessage(
                companionId = companionId,
                content = text,
                isFromUser = true,
                timestamp = System.currentTimeMillis()
            )
            chatRepository.sendMessage(userMessage)
            companionRepository.updateTimestamp(companionId)

            val history = chatRepository.getRecentMessagesSync(companionId, limit = 30)
                .filterDecrypted()
            // 非流式 — AI 输出不允许流式
            val aiResponse = try {
                aiServiceProvider.sendMessage(companion.toAiCompanionInfo(), history.toAiChatMessages(), 0)
            } catch (e: Exception) {
                android.util.Log.e("WeChatBridge", "sendMessage failed", e)
                AiResponse(content = e.message ?: "API 错误")
            }
            val aiResponseText = aiResponse.content
            val hasReceivedContent = aiResponseText.isNotBlank()

            // 安全检查：拦截 AI 违规输出
            if (aiResponseText.isNotBlank()) {
                val outputSafetyResult = com.ref_b.ai.common.ContentFilter.checkOutputSafety(aiResponseText)
                if (!outputSafetyResult.isSafe) {
                    android.util.Log.w("WeChatBridge", "AI output blocked by safety filter: ${outputSafetyResult.level} - ${outputSafetyResult.reason}")
                    // [R2 FIX] AI 生成内容不应累加用户封禁——模型输出不是用户的责任（与 AiService/ChatViewModel 策略对齐）
                    val blockedResponse = "抱歉，我无法回应这个话题。"
                    val blockedMsg = ChatMessage(
                        companionId = companionId,
                        content = blockedResponse,
                        isFromUser = false,
                        timestamp = System.currentTimeMillis()
                    )
                    chatRepository.sendMessage(blockedMsg)
                    return@withContext blockedResponse
                }
            }

            val msg = ChatMessage(
                companionId = companionId,
                content = aiResponseText.ifBlank { "API返回空内容" },
                isFromUser = false,
                timestamp = System.currentTimeMillis()
            )
            val aiMessageId = chatRepository.sendMessageAndGetId(msg)

            if (aiMessageId > 0) {
                val processed = runCatching { extractStickerTags(aiResponseText) }
                    .getOrDefault(Pair(aiResponseText, emptyList<StickerInfo>()))
                if (processed.first.isNotEmpty() && processed.first != aiResponseText) {
                    chatRepository.updateMessageContent(aiMessageId, processed.first)
                }
            }

            if (aiMessageId > 0) {
                companionRepository.updateTimestamp(companionId)
                companionRepository.increaseIntimacy(companionId, 2)
                bridgeScope.launch {
                    runCatching { memoryRepository.extractAndSaveMemories(companionId, text, aiResponseText) }
                }
            }

            if (tokenStore.getForwardEnabled() && aiResponseText.isNotBlank()) {
                val (cleanText, stickers) = runCatching { extractStickerTags(aiResponseText) }
                    .getOrDefault(Pair(aiResponseText, emptyList<StickerInfo>()))

                android.util.Log.d("WeChatBridge", "Forward: cleanText length=${cleanText.length}, stickers count=${stickers.size}")

                val isTextMeaningful = cleanText.isNotBlank() &&
                        cleanText.length > 1 &&
                        !cleanText.all { it.isWhitespace() } &&
                        cleanText != "\u200B"

                if (stickers.isNotEmpty() && !isTextMeaningful) {
                    android.util.Log.d("WeChatBridge", "Only stickers, no meaningful text to send")
                } else if (isTextMeaningful) {
                    val finalText = cleanText.trim()
                                .replace(Regex("\\s+"), " ")
                                .replace(Regex("^[\\[\\]\\s，。！？、]+"), "")
                                .replace(Regex("[\\[\\]\\s，。！？、]+$"), "")
                                .trim()
                    if (finalText.length >= 1) {
                        val sendResult = weChatRepository.sendTextMessage(wechatUserId, finalText)
                        android.util.Log.d("WeChatBridge", "Text sent result: ${if (sendResult.isSuccess) "OK" else sendResult.exceptionOrNull()?.message}")
                    }
                }

                stickers.forEachIndexed { index, sticker ->
                    bridgeScope.launch {
                        runCatching {
                            android.util.Log.d("WeChatBridge", "Sending sticker[$index]: name=${sticker.name}, path=${sticker.path}")
                            val bytes = loadStickerBytes(sticker)
                            if (bytes != null) {
                                android.util.Log.d("WeChatBridge", "Sticker bytes loaded: ${bytes.size}, sending...")
                                val imgResult = weChatRepository.sendImageMessage(
                                    toUserId = wechatUserId,
                                    imageBytes = bytes,
                                    fileName = sticker.fileName ?: "sticker.png",
                                    description = ""
                                )
                                android.util.Log.d("WeChatBridge", "Sticker[$index] sent: ${if (imgResult.isSuccess) "OK" else imgResult.exceptionOrNull()?.message}")
                            } else {
                                android.util.Log.w("WeChatBridge", "Sticker[$index] bytes is NULL!")
                            }
                        }.onFailure { e ->
                            android.util.Log.e("WeChatBridge", "Error sending sticker[$index]", e)
                        }
                    }
                }
            }

            aiResponseText.ifBlank { null }
        } catch (e: Exception) {
            android.util.Log.e("WeChatBridge", "Error handling incoming message", e)
            null
        }
    }

    suspend fun handleTextMessage(wechatUserId: String, text: String): String? = withContext(Dispatchers.IO) {
        val message = M0(
            fromUserId = wechatUserId,
            toUserId = "",
            itemList = listOf(
                M1(type = 1, textItem = M2(text = text))
            )
        )
        handleIncomingMessage(message)
    }

    suspend fun handleImageMessage(wechatUserId: String, message: M0): String? = withContext(Dispatchers.IO) {
        try {
            android.util.Log.d("WeChatBridge", "handleImageMessage called for $wechatUserId")

            val companionId = mappingManager.getOrCreateMapping(wechatUserId)
                ?: run {
                    android.util.Log.e("WeChatBridge", "Failed to get/create mapping for $wechatUserId")
                    return@withContext null
                }

            val companion = companionRepository.getCompanionById(companionId)
                ?: run {
                    android.util.Log.e("WeChatBridge", "Companion not found for id=$companionId")
                    return@withContext null
                }

            val imageItem = message.itemList?.firstOrNull { it.type == M1Type.IMAGE.value }?.imageItem
            if (imageItem == null) {
                android.util.Log.w("WeChatBridge", "No image item found in message")
                return@withContext null
            }

            android.util.Log.d("WeChatBridge", "Image item found, cdnInfo present=${imageItem.cdnImg != null}")

            val imagePath = downloadImageFromCdn(message, imageItem)

            if (imagePath == null) {
                android.util.Log.e("WeChatBridge", "Failed to download image, sending fallback response")
                val fallbackResponse = "收到您的图片了！不过暂时无法识别图片内容，可能是因为SDK版本限制。您可以描述一下图片内容，我会尽力帮助您~"
                weChatRepository.sendTextMessage(wechatUserId, fallbackResponse)
                return@withContext fallbackResponse
            }

            android.util.Log.d("WeChatBridge", "Image downloaded successfully: $imagePath")

            val userMessage = ChatMessage(
                companionId = companionId,
                content = imagePath,
                isFromUser = true,
                timestamp = System.currentTimeMillis(),
                type = com.ref_b.ai.database.model.MessageType.IMAGE,
                linkString = imagePath
            )
            chatRepository.sendMessage(userMessage)
            companionRepository.updateTimestamp(companionId)

            // 安全检查：图片消息暂无法做内容审核，跳过输入安全检查
            // TODO: 接入 OCR/视觉模型后对图片描述文本进行安全过滤

            val history = chatRepository.getRecentMessagesSync(companionId, limit = 30)
                .filterDecrypted()

            val aiResponse = aiServiceProvider.sendMessageWithImage(
                companion.toAiCompanionInfo(), history.toAiChatMessages(), imagePath
            )
            val responseText = aiResponse.content

            // 安全检查：拦截 AI 违规输出
            if (responseText.isNotBlank()) {
                val outputSafetyResult = com.ref_b.ai.common.ContentFilter.checkOutputSafety(responseText)
                if (!outputSafetyResult.isSafe) {
                    android.util.Log.w("WeChatBridge", "Vision AI output blocked by safety filter: ${outputSafetyResult.level} - ${outputSafetyResult.reason}")
                    // [R2 FIX] AI 生成内容不应累加用户封禁
                    val blockedResponse = "抱歉，我无法回应这个话题。"
                    weChatRepository.sendTextMessage(wechatUserId, blockedResponse)
                    val blockedMsg = ChatMessage(
                        companionId = companionId,
                        content = blockedResponse,
                        isFromUser = false,
                        timestamp = System.currentTimeMillis()
                    )
                    chatRepository.sendMessage(blockedMsg)
                    return@withContext blockedResponse
                }
            }

            if (responseText.isNotBlank()) {
                val (cleanText, stickers) = runCatching { extractStickerTags(responseText) }.getOrDefault(Pair(responseText, emptyList()))

                val sendResult = weChatRepository.sendTextMessage(wechatUserId, cleanText)
                if (sendResult.isSuccess) {
                    android.util.Log.d("WeChatBridge", "Vision AI response sent to $wechatUserId")
                } else {
                    android.util.Log.e("WeChatBridge", "Failed to send vision response: ${sendResult.exceptionOrNull()?.message}")
                }

                if (stickers.isNotEmpty()) {
                    stickers.forEachIndexed { index, sticker ->
                        bridgeScope.launch {
                            runCatching {
                                android.util.Log.d("WeChatBridge", "Sending vision sticker[$index]: name=${sticker.name}, path=${sticker.path}")
                                val bytes = loadStickerBytes(sticker)
                                if (bytes != null) {
                                    android.util.Log.d("WeChatBridge", "Vision sticker bytes loaded: ${bytes.size}, sending...")
                                    val imgResult = weChatRepository.sendImageMessage(
                                        toUserId = wechatUserId,
                                        imageBytes = bytes,
                                        fileName = sticker.fileName ?: "sticker.png",
                                        description = sticker.description ?: sticker.name
                                    )
                                    android.util.Log.d("WeChatBridge", "Vision sticker[$index] sent: ${if (imgResult.isSuccess) "OK" else imgResult.exceptionOrNull()?.message}")
                                } else {
                                    android.util.Log.w("WeChatBridge", "Vision sticker[$index] bytes is NULL!")
                                }
                            }.onFailure { e ->
                                android.util.Log.e("WeChatBridge", "Error sending vision sticker[$index]", e)
                            }
                        }
                    }
                }
            }

            val aiMessage = ChatMessage(
                companionId = companionId,
                content = responseText,
                isFromUser = false,
                timestamp = System.currentTimeMillis()
            )
            chatRepository.sendMessage(aiMessage)

            runCatching {
                memoryRepository.extractAndSaveMemories(companionId, "[图片]", responseText)
            }.onFailure {
                android.util.Log.e("WeChatBridge", "Memory save failed for vision: ${it.message}")
            }

            responseText.ifBlank { null }
        } catch (e: Exception) {
            android.util.Log.e("WeChatBridge", "Error in handleImageMessage", e)
            val errorResponse = "图片识别过程中出现错误: ${e.message}. 请稍后重试或发送文字描述。"
            runCatching {
                weChatRepository.sendTextMessage(wechatUserId, errorResponse)
            }
            errorResponse
        }
    }

    private suspend fun downloadImageFromCdn(message: M0, imageItem: com.ref_b.ai.feature.wechat.data.model.M3): String? {
        return try {
            val sdkClient = WeChatServiceLocator.sdkClientManager(context)

            val tempFile = File(context.cacheDir, "wechat_img_${System.currentTimeMillis()}.jpg")

            when {
                imageItem.cdnImg != null -> {
                    android.util.Log.d("WeChatBridge", "Attempting to download image via SDK CDN")

                    runCatching {
                        val downloadedBytes = sdkClient.downloadMedia(
                            imageItem.cdnImg
                        )

                        if (downloadedBytes != null && downloadedBytes.size > 0) {
                            tempFile.writeBytes(downloadedBytes)
                            tempFile.absolutePath
                        } else {
                            android.util.Log.w("WeChatBridge", "SDK returned empty bytes for image")
                            null
                        }
                    }.getOrNull()
                }
                else -> {
                    android.util.Log.w("WeChatBridge", "No CDN info available for image")
                    null
                }
            }
        } catch (e: Exception) {
            android.util.Log.e("WeChatBridge", "Failed to download image from CDN", e)
            null
        }
    }

    fun close() {
        bridgeJob.cancel()
    }

    private fun extractText(message: M0): String? {
        return message.itemList?.firstNotNullOfOrNull { item ->
            item.textItem?.text
        }
    }

    private fun extractStickerTags(text: String): Pair<String, List<StickerInfo>> {
        val stickerManager = StickerManager.getInstance(context)
        val systemTags = setOf("语音", "图片", "视频", "文件", "位置", "红包", "转账")
        val stickerRegex = Regex("\\[([^\\[\\]]+?)\\]")
        val fileNamePattern = Regex("^[a-zA-Z0-9_\\-]+\\.(png|jpg|jpeg|gif|webp)$", RegexOption.IGNORE_CASE)
        val matches = stickerRegex.findAll(text).toList()

        val stickers = mutableListOf<StickerInfo>()
        val sentStickerDescs = mutableSetOf<String>()
        var cleanText = text

        val rolePrefixRegex = Regex("(?m)^\\s*\\[(?:角色\\d+|[^\\[\\]]+?)\\]\\s*")
        cleanText = rolePrefixRegex.replace(cleanText, "")

        val thinkRegex = Regex("(?is)<think[^>]*>[\\s\\S]*?</think\\s*>")
        cleanText = thinkRegex.replace(cleanText, "")

        val encRegex = Regex("(?m)^enc:\\S+$")
        cleanText = encRegex.replace(cleanText, "")

        for (match in matches) {
            val description = match.groupValues[1].trim()
            if (description in systemTags) continue

            var found = false
            val sticker = stickerManager.findStickerByDescriptionExact(description)
                ?: stickerManager.findStickerByDescription(description)
            if (sticker != null) {
                // 防止同一回合重复添加相同的表情包
                if (stickers.none { it.name == sticker.name }) {
                    stickers.add(sticker)
                    found = true
                } else {
                    android.util.Log.d("WeChatBridge", "Duplicate sticker skipped: ${sticker.name}")
                }
            }
            // 无论是否成功匹配，都记录描述用于后续清理
            sentStickerDescs.add(description)
            sticker?.description?.let { sentStickerDescs.add(it) }
            cleanText = cleanText.replace(match.value, "")
            if (!found && fileNamePattern.matches(description)) {
                android.util.Log.d("WeChatBridge", "Removed unmatched sticker file tag: [$description]")
            }
            if (!found && !fileNamePattern.matches(description)) {
                android.util.Log.w("WeChatBridge", "Removed unmatched sticker tag: [$description]")
            }
        }

        // 同时清理 cleanText 中可能残留的已发送表情包描述文字（如AI同时输出 [爱你] 和 爱你）
        for (desc in sentStickerDescs) {
            if (desc.length >= 2 && cleanText.contains(desc)) {
                cleanText = cleanText.replace(desc, "")
                android.util.Log.d("WeChatBridge", "Removed residual sticker desc from text: $desc")
            }
        }

        // 清理孤立的 ] 和 [（防止格式异常的残留）
        cleanText = cleanText.replace("]", "").replace("[", "")
        // 清理残留的 sticker 文件名
        cleanText = Regex("\\bsticker_\\w+\\.png\\b", RegexOption.IGNORE_CASE).replace(cleanText, "")

        // 额外清理：如果已发送的表情包的 description/name 还残留在 cleanText 中，也移除
        for (sticker in stickers) {
            val desc = sticker.description
            if (!desc.isNullOrBlank() && desc.length >= 2 && cleanText.contains(desc)) {
                cleanText = cleanText.replace(desc, "")
                android.util.Log.d("WeChatBridge", "Removed sticker desc from text (extra): $desc")
            }
            val name = sticker.name
            if (name.length >= 2 && cleanText.contains(name)) {
                cleanText = cleanText.replace(name, "")
                android.util.Log.d("WeChatBridge", "Removed sticker name from text (extra): $name")
            }
        }

        var result = cleanText.trim()
            .replace(Regex("\\r\\n|\\r|\\n+"), "，")
            .replace(Regex("，{2,}"), "，")
            .trim()
            .trimStart('，', ',', '.', '。', ' ')

        // 模式2：AI文字中提到表情包但没有输出标记 → 智能匹配（与App端保持一致）
        if (stickers.isEmpty()) {
            val allRules = stickerManager.getAllRules()
            if (allRules.isNotEmpty()) {
                val matchedStickers = mutableListOf<Pair<StickerInfo, String>>()
                for (rule in allRules.shuffled()) {
                    val desc = rule.description
                    if (desc.length >= 2 && text.contains(desc)) {
                        val sticker = stickerManager.findStickerByDescription(desc)
                        if (sticker != null) matchedStickers.add(sticker to desc)
                    }
                }
                if (matchedStickers.isNotEmpty()) {
                    val (picked, matchedDesc) = matchedStickers.random()
                    // 防止同一回合重复添加相同的表情包
                    if (stickers.none { it.name == picked.name }) {
                        stickers.add(picked)
                        android.util.Log.d("WeChatBridge", "Matched sticker from text: ${picked.name}")
                    } else {
                        android.util.Log.d("WeChatBridge", "Duplicate sticker skipped: ${picked.name}")
                    }
                    // 从 result 中移除描述文字：优先用AI文字中实际匹配到的desc，其次用sticker.description/name
                    val descToRemove = if (result.contains(matchedDesc)) matchedDesc else (picked.description ?: picked.name)
                    if (descToRemove.length >= 2) {
                        result = result.replace(descToRemove, "")
                        android.util.Log.d("WeChatBridge", "Removed sticker desc from text: $descToRemove (matched: $matchedDesc)")
                    }
                    result = removeLocalRepetition(result)
                }
            }
        }

        // 去除局部重复子串（与App端保持一致）
        result = removeLocalRepetition(result)

        return result to stickers
    }

    /**
     * 去除文本中的局部重复子串（与ChatViewModel保持一致）
     */
    private fun removeLocalRepetition(text: String): String {
        if (text.length < 4) return text
        var result = text

        // 模式1：X + Y + Y（Y是X的后缀）
        for (len in result.length / 2 downTo 2) {
            val suffix = result.takeLast(len)
            val beforeSuffix = result.dropLast(len)
            if (beforeSuffix.endsWith(suffix)) {
                result = beforeSuffix
                return removeLocalRepetition(result)
            }
        }

        // 模式2：检测子串重复（非后缀）
        for (len in result.length / 2 downTo 4) {
            val suffix = result.takeLast(len)
            val beforeSuffix = result.dropLast(len)
            if (beforeSuffix.contains(suffix)) {
                result = beforeSuffix
                return removeLocalRepetition(result)
            }
            val suffixCleaned = suffix.trimEnd('。', '！', '？', '，', '.', '!', '?', ',', ' ')
            val beforeSuffixCleaned = beforeSuffix.trimEnd('。', '！', '？', '，', '.', '!', '?', ',', ' ')
            if (suffixCleaned.length >= 4 && beforeSuffixCleaned.endsWith(suffixCleaned)) {
                result = beforeSuffix
                return removeLocalRepetition(result)
            }
        }

        // 模式3：句子级重复检测
        val sentenceDelimiters = Regex("(?<=[。！？.!?])")
        val sentences = result.split(sentenceDelimiters)
        if (sentences.size >= 2) {
            val deduped = mutableListOf<String>()
            for (sentence in sentences) {
                val trimmed = sentence.trim()
                if (trimmed.isEmpty()) continue
                val currentClean = trimmed.trimEnd('。', '！', '？', '，', '.', '!', '?', ',', ' ')
                var isDuplicate = false
                for (prev in deduped) {
                    val prevClean = prev.trimEnd('。', '！', '？', '，', '.', '!', '?', ',', ' ')
                    if (currentClean == prevClean ||
                        (currentClean.length >= 4 && prevClean.endsWith(currentClean)) ||
                        (prevClean.length >= 4 && currentClean.endsWith(prevClean))
                    ) {
                        isDuplicate = true
                        break
                    }
                }
                if (!isDuplicate) {
                    deduped.add(trimmed)
                }
            }
            val joined = deduped.joinToString("")
            if (joined.length < result.length) {
                result = joined
                return removeLocalRepetition(result)
            }
        }

        return result
    }

    private fun loadStickerBytes(sticker: StickerInfo): ByteArray? {
        return try {
            when {
                sticker.path.startsWith("asset://") -> {
                    val assetPath = sticker.path.removePrefix("asset://")
                    context.assets.open(assetPath).use { it.readBytes() }
                }
                else -> File(sticker.path).takeIf { it.exists() }?.readBytes()
            }
        } catch (e: Exception) {
            android.util.Log.e("WeChatBridge", "Failed to load sticker bytes: ${sticker.path}", e)
            null
        }
    }

    private fun com.ref_b.ai.database.model.CompanionEntity.toAiCompanionInfo() = AiCompanionInfo(
        id = id, name = name, personality = personality,
        age = age, backstory = backstory, speakingStyle = speakingStyle,
        systemPrompt = systemPrompt
    )

    private fun com.ref_b.ai.database.model.ChatMessage.toAiChatMessage() = AiChatMessage(
        isFromUser = isFromUser, content = content, timestamp = timestamp,
        type = when (type) {
            com.ref_b.ai.database.model.MessageType.IMAGE -> AiMessageType.IMAGE
            else -> AiMessageType.TEXT
        },
        companionId = companionId
    )

    private fun List<com.ref_b.ai.database.model.ChatMessage>.toAiChatMessages() = map { it.toAiChatMessage() }
}`

// SourceName: feature__wechat__src__main__java__com__ref_b__ai__feature__wechat__service__WeChatAiReplyWorker.kt
// SourceSet: ref_b
const RawFeatureWechatServiceWeChatAiReplyWorker = `package com.ref_b.ai.feature.wechat.service

import android.content.Context
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.Data
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import com.ref_b.ai.feature.wechat.data.WeChatChatBridge
import com.ref_b.ai.feature.wechat.data.WeChatMessageRepository
import com.ref_b.ai.feature.wechat.data.WeChatTokenStore
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * 后台处理微信消息并触发 AI 回复的 Worker。
 *
 * 当 App 不在前台时，收到微信消息会 enqueue 此 Worker，
 * 在后台完成 AI 生成并发送回复到微信。
 */
class WeChatAiReplyWorker(
    context: Context,
    params: WorkerParameters
) : CoroutineWorker(context, params) {

    override suspend fun doWork(): Result = withContext(Dispatchers.IO) {
        try {
            val wechatUserId = inputData.getString(KEY_WECHAT_USER_ID) ?: ""
            val messageText = inputData.getString(KEY_MESSAGE_TEXT) ?: ""

            if (wechatUserId.isBlank() || messageText.isBlank()) {
                return@withContext Result.failure()
            }

            val tokenStore = WeChatTokenStore(applicationContext)

            // 检查自动回复是否开启
            if (!tokenStore.getAutoReply()) {
                return@withContext Result.success()
            }

            val repository = WeChatServiceLocator.messageRepository(applicationContext)
            val bridge = WeChatChatBridge(applicationContext, repository)

            try {
                bridge.handleTextMessage(wechatUserId, messageText)
            } finally {
                bridge.close()
            }

            Result.success()
        } catch (e: Exception) {
            android.util.Log.e("WeChatAiReplyWorker", "Error processing message", e)
            Result.retry()
        }
    }

    companion object {
        private const val WORK_NAME_PREFIX = "wechat_ai_reply_"
        const val KEY_WECHAT_USER_ID = "wechat_user_id"
        const val KEY_MESSAGE_TEXT = "message_text"

        private val networkConstraints = Constraints.Builder()
            .setRequiredNetworkType(NetworkType.CONNECTED)
            .build()

        fun enqueue(context: Context, wechatUserId: String, messageText: String) {
            val inputData = Data.Builder()
                .putString(KEY_WECHAT_USER_ID, wechatUserId)
                .putString(KEY_MESSAGE_TEXT, messageText)
                .build()

            val workRequest = OneTimeWorkRequestBuilder<WeChatAiReplyWorker>()
                .setInputData(inputData)
                .setConstraints(networkConstraints)
                .build()

            WorkManager.getInstance(context).enqueueUniqueWork(
                "$WORK_NAME_PREFIX$wechatUserId",
                ExistingWorkPolicy.APPEND_OR_REPLACE,
                workRequest
            )
        }
    }
}`

// SourceName: feature__wechat__src__main__java__com__ref_b__ai__feature__wechat__service__WeChatProactiveMessageReceiver.kt
// SourceSet: ref_b
const RawFeatureWechatServiceWeChatProactiveMessageReceiver = `package com.ref_b.ai.feature.wechat.service

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log
import com.ref_b.ai.common.StickerManager
import com.ref_b.ai.common.wechat.WeChatBroadcast
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeoutOrNull
import java.io.File

/**【已脱敏】接收来自 feature:notification 的广播，将 App 主动生成的消息同步到微信。**/
class WeChatProactiveMessageReceiver : BroadcastReceiver() {

    override fun onReceive(context: Context, intent: Intent) {
        val pendingResult = goAsync()
        handleReceive(context, intent, pendingResult)
    }

    companion object {
        private const val TAG = "WeChatProactiveReceiver"
        private val STICKER_PATTERN = Regex("^\\[([^\\[\\]]+)\\]$")
        private val ROLE_PREFIX_REGEX = Regex("(?m)^\\s*\\[(?:角色\\d+|[^\\[\\]]+?)\\]\\s*")
        private val ENC_LEAK_REGEX = Regex("(?m)^enc:\\S+$")
        private val THINK_REGEX = Regex("(?is)<think[^>]*>[\\s\\S]*?</think\\s*>")
        private val FILE_NAME_PATTERN = Regex("^[a-zA-Z0-9_\\-]+\\.(png|jpg|jpeg|gif|webp)$", RegexOption.IGNORE_CASE)

        fun isStickerContent(content: String): Boolean {
            return STICKER_PATTERN.matches(content.trim())
        }

        fun cleanAiContentForWechat(raw: String): String {
            var text = raw
            text = ROLE_PREFIX_REGEX.replace(text, "")
            text = THINK_REGEX.replace(text, "")
            text = ENC_LEAK_REGEX.replace(text, "")
            val bracketRegex = Regex("\\[([^\\[\\]]+?)\\]")
            val systemTags = setOf("语音", "图片", "视频", "文件", "位置", "红包", "转账")
            val cleanedStickerDescs = mutableSetOf<String>()
            text = bracketRegex.replace(text) { match: MatchResult ->
                val inner = match.groupValues[1].trim()
                if (inner in systemTags) {
                    match.value
                } else {
                    cleanedStickerDescs.add(inner)
                    Log.d(TAG, "Cleaned sticker/non-system tag: [$inner]")
                    ""
                }
            }
            // 清理孤立的 ] 和 [（防止格式异常的残留）
            text = text.replace("]", "").replace("[", "")
            // 清理残留的 sticker 文件名
            text = Regex("\\bsticker_\\w+\\.png\\b", RegexOption.IGNORE_CASE).replace(text, "")
            // 清理已知的表情包描述文字残留
            for (desc in cleanedStickerDescs) {
                if (desc.length >= 2 && text.contains(desc)) {
                    text = text.replace(desc, "")
                    Log.d(TAG, "Removed residual sticker desc: $desc")
                }
            }
            // 换行处理：前面是标点的直接删除，否则替换成逗号
            text = text.replace(Regex("(?<=[。！？])\\s*\\n+\\s*"), "")
            text = text.replace(Regex("(?<![。！？])\\s*\\n+\\s*"), "，")
            text = text.replace(Regex("，{2,}"), "，")
                .trim()
                .trimStart('，', ',', '.', '。', ' ')

            // 去除连续重复的短句（如"打算晚上吃什么呀。打算晚上吃什么呀。"）
            val sentences = text.split(Regex("(?<=[。！？])"))
            val deduped = mutableListOf<String>()
            for (sentence in sentences) {
                val trimmed = sentence.trim()
                if (trimmed.isEmpty()) continue
                val currentClean = trimmed.trimEnd('。', '！', '？', '，', '.', '!', '?', ',', ' ')
                // 检查当前句子是否与之前任何句子重复（支持子串匹配）
                var isDuplicate = false
                for (prev in deduped) {
                    val prevClean = prev.trimEnd('。', '！', '？', '，', '.', '!', '?', ',', ' ')
                    if (currentClean == prevClean ||
                        (currentClean.length >= 4 && prevClean.endsWith(currentClean)) ||
                        (prevClean.length >= 4 && currentClean.endsWith(prevClean))
                    ) {
                        isDuplicate = true
                        Log.d(TAG, "Removed duplicate sentence: $trimmed (matches previous: $prevClean)")
                        break
                    }
                }
                if (!isDuplicate) {
                    deduped.add(sentence)
                }
            }
            var result = deduped.joinToString("").trim()

            // 额外检测：子串级重复（如"能正常看到了嘛～现在好了吧？正常看到了嘛～现在好了吧？"）
            for (len in result.length / 2 downTo 4) {
                val suffix = result.takeLast(len)
                val beforeSuffix = result.dropLast(len)
                if (beforeSuffix.contains(suffix)) {
                    result = beforeSuffix
                    break
                }
                val suffixCleaned = suffix.trimEnd('。', '！', '？', '，', '.', '!', '?', ',', ' ')
                val beforeSuffixCleaned = beforeSuffix.trimEnd('。', '！', '？', '，', '.', '!', '?', ',', ' ')
                if (suffixCleaned.length >= 4 && beforeSuffixCleaned.endsWith(suffixCleaned)) {
                    result = beforeSuffix
                    break
                }
            }

            return result
        }

        private fun extractStickerName(content: String): String? {
            return STICKER_PATTERN.find(content.trim())?.groupValues?.get(1)
        }

        fun handleReceive(
            context: Context,
            intent: Intent,
            pendingResult: BroadcastReceiver.PendingResult
        ) {
            val companionId = intent.getLongExtra(WeChatBroadcast.EXTRA_COMPANION_ID, -1L)
            if (companionId == -1L) {
                pendingResult.finish()
                return
            }

            val messageId = intent.getLongExtra(WeChatBroadcast.EXTRA_MESSAGE_ID, -1L)
            val directContent = intent.getStringExtra(WeChatBroadcast.EXTRA_CONTENT)
            val finalContent = intent.getStringExtra(WeChatBroadcast.EXTRA_FINAL_CONTENT)

            // 使用标志位防止 finish() 重复调用：
            // goAsync() 有 10 秒超时，系统可能自动 finish()，
            // 协程 finally 中需检查是否已完成，避免 "Broadcast already finished" 闪退。
            var finishedByCoroutine = false

            CoroutineScope(SupervisorJob() + Dispatchers.IO).launch {
                try {
                    // goAsync() 限制 10 秒，预留 500ms 余量避免系统先 finish
                    withTimeoutOrNull(9500L) {
                        val content: String = if (!finalContent.isNullOrBlank()) {
                            Log.d(TAG, "Using final content from App, skipping DB lookup")
                            finalContent
                        } else if (messageId != -1L) {
                            val db = com.ref_b.ai.database.AppDatabase.getDatabase(context)
                            val msg = db.chatMessageDao().getMessageById(messageId)
                            msg?.content ?: return@withTimeoutOrNull
                        } else {
                            directContent ?: return@withTimeoutOrNull
                        }

                        val tokenStore = WeChatServiceLocator.tokenStore(context)
                        if (!tokenStore.isLoggedIn()) {
                            Log.d(TAG, "Not logged in, skip proactive sync")
                            return@withTimeoutOrNull
                        }
                        if (!tokenStore.getForwardEnabled()) {
                            Log.d(TAG, "Forwarding disabled, skip proactive sync")
                            return@withTimeoutOrNull
                        }

                        val repository = WeChatServiceLocator.messageRepository(context)
                        val wechatUserIds = tokenStore.getWechatUserIdsForCompanionId(companionId)

                        if (wechatUserIds.isEmpty()) {
                            Log.d(TAG, "No WeChat user mapped to companion $companionId")
                            return@withTimeoutOrNull
                        }

                        val account = tokenStore.getAccount()
                        val isSticker = isStickerContent(content)

                        for (wechatUserId in wechatUserIds) {
                            val contextToken = account?.let { tokenStore.getContextToken(it.accountId, wechatUserId) }
                            if (isSticker) {
                                sendStickerToWechat(context, repository, wechatUserId, content, contextToken)
                            } else {
                                val cleanedContent = cleanAiContentForWechat(content)
                                if (cleanedContent.isBlank()) {
                                    Log.d(TAG, "Skipping empty message after cleaning")
                                    continue
                                }
                                val result = repository.sendTextMessage(wechatUserId, cleanedContent, contextToken)
                                result.onSuccess {
                                    Log.d(TAG, "Proactive text sent to $wechatUserId")
                                }.onFailure { error ->
                                    Log.w(TAG, "Failed to send proactive text to $wechatUserId: ${error.message}")
                                }
                            }
                        }
                    } ?: run {
                        Log.w(TAG, "Proactive message sync timed out (9.5s)")
                    }
                } catch (e: Exception) {
                    Log.e(TAG, "Error sending proactive message", e)
                } finally {
                    // 先取消协程 scope，再 finish 广播
                    cancel()
                    // 仅在协程未超时（系统未自动 finish）时调用 finish()
                    if (!finishedByCoroutine) {
                        finishedByCoroutine = true
                        pendingResult.finish()
                    }
                }
            }
        }

        private suspend fun sendStickerToWechat(
            context: Context,
            repository: com.ref_b.ai.feature.wechat.data.WeChatMessageRepository,
            wechatUserId: String,
            content: String,
            contextToken: String?
        ) {
            val stickerName = extractStickerName(content) ?: run {
                Log.w(TAG, "Invalid sticker format: $content")
                return
            }
            val stickerManager = StickerManager.getInstance(context)
            var sticker = stickerManager.findStickerByDescriptionExact(stickerName)
            if (sticker == null) {
                sticker = stickerManager.findStickerByDescription(stickerName)
            }
            if (sticker == null && !stickerName.endsWith(".png")) {
                sticker = stickerManager.findStickerByDescription("$stickerName.png")
            }
            if (sticker == null) {
                val allStickers = stickerManager.getAllStickers()
                sticker = allStickers.find { it.fileName == stickerName || it.name == stickerName }
            }
            if (sticker == null) {
                Log.w(TAG, "Sticker not found: $stickerName, skipping (total stickers: ${StickerManager.getInstance(context).getAllStickers().size})")
                return
            }
            val bytes = try {
                when {
                    sticker.path.startsWith("asset://") -> {
                        val assetPath = sticker.path.removePrefix("asset://")
                        context.assets.open(assetPath).use { it.readBytes() }
                    }
                    else -> File(sticker.path).takeIf { it.exists() }?.readBytes()
                }
            } catch (e: Exception) {
                Log.e(TAG, "Failed to load sticker bytes: ${sticker.path}", e)
                null
            }
            if (bytes == null) {
                Log.w(TAG, "Sticker bytes null for: $stickerName")
                return
            }
            val result = repository.sendImageMessage(
                toUserId = wechatUserId,
                imageBytes = bytes,
                fileName = sticker.fileName ?: "sticker.png",
                description = sticker.description ?: sticker.name,
                contextToken = contextToken
            )
            result.onSuccess {
                Log.d(TAG, "Proactive sticker image sent to $wechatUserId: $stickerName")
            }.onFailure { error ->
                Log.w(TAG, "Failed to send proactive sticker to $wechatUserId: ${error.message}")
            }
        }
    }
}`
