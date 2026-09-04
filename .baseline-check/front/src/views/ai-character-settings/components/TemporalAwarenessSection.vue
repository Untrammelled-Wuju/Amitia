<template>
  <div class="section-card temporal-section" v-loading="loading">
    <div class="section-heading">
      <div>
        <div class="section-title">时间感知</div>
        <div class="section-subtitle">
          角色使用自己的当地时间理解生活状态，实际投递仍遵守用户安静时段。
        </div>
      </div>
      <el-button
        type="primary"
        plain
        :loading="saving"
        :disabled="!characterId"
        @click="save"
        >保存时间设置</el-button
      >
    </div>
    <div class="temporal-grid">
      <div class="temporal-item">
        <label>启用时间感知</label><el-switch v-model="profile.enabled" />
      </div>
      <div class="temporal-item">
        <label>时区模式</label
        ><el-radio-group v-model="profile.timezoneMode"
          ><el-radio-button value="follow_user">跟随用户</el-radio-button
          ><el-radio-button value="fixed"
            >固定时区</el-radio-button
          ></el-radio-group
        >
      </div>
      <div class="temporal-item">
        <label>IANA 时区</label
        ><el-select
          v-model="profile.timezone"
          filterable
          allow-create
          :disabled="profile.timezoneMode === 'follow_user'"
          class="full-width"
          ><el-option
            v-for="zone in timezoneOptions"
            :key="zone"
            :label="zone"
            :value="zone" /></el-select
        ><span class="item-help">固定时区会正确处理当地夏令时。</span>
      </div>
      <div class="temporal-item">
        <label>角色生日</label
        ><el-input
          v-model="birthday"
          placeholder="MM-DD，例如 08-16"
          maxlength="5"
        /><span class="item-help"
          >生日以当地日期保存，不转换成某一年的 UTC 零点。</span
        >
      </div>
      <div class="temporal-item">
        <label>时段感知</label><el-switch v-model="profile.daypartAwareness" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import {
  createTemporalAnchor,
  deleteTemporalAnchor,
  getCharacterTemporalProfile,
  getTemporalProfile,
  listTemporalAnchors,
  updateCharacterTemporalProfile,
  updateTemporalAnchor,
  type TemporalAnchor,
  type TemporalProfile,
} from "@/api/temporal";

const props = defineProps<{ characterId: string }>();
const loading = ref(false);
const saving = ref(false);
const birthday = ref("");
const birthdayAnchor = ref<TemporalAnchor | null>(null);
const userTimezone = ref("Asia/Shanghai");
const timezoneOptions = [
  "Asia/Shanghai",
  "Asia/Tokyo",
  "Asia/Hong_Kong",
  "Asia/Singapore",
  "Europe/London",
  "Europe/Paris",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "Australia/Sydney",
  "UTC",
];
const profile = reactive<TemporalProfile>({
  id: "",
  ownerType: "character",
  ownerId: "",
  timezoneMode: "follow_user",
  timezone: "Asia/Shanghai",
  locale: "zh-CN",
  calendarSystem: "gregorian",
  weekStart: 1,
  holidayRegion: "",
  hemisphere: "unknown",
  daypartConfigJson: "{}",
  quietHoursJson: "{}",
  autoDetectTimezone: false,
  travelMode: false,
  awarenessLevel: 70,
  source: "fallback",
  confidence: 30,
  enabled: true,
  holidayAwareness: true,
  daypartAwareness: true,
  anniversaryAwareness: true,
  memoryResonance: true,
  allowSharedDateMention: true,
  version: 1,
  createdAtUtc: "",
  updatedAtUtc: "",
});

async function load() {
  if (!props.characterId) return;
  loading.value = true;
  try {
    const [characterProfile, userProfile, anchors] = await Promise.all([
      getCharacterTemporalProfile(props.characterId),
      getTemporalProfile(),
      listTemporalAnchors(props.characterId),
    ]);
    Object.assign(profile, characterProfile);
    userTimezone.value = userProfile.timezone;
    birthdayAnchor.value =
      anchors.find(
        (item) =>
          item.characterId === props.characterId &&
          item.anchorType === "birthday",
      ) || null;
    birthday.value = birthdayAnchor.value?.localDate || "";
  } catch {
    ElMessage.error("加载角色时间设置失败");
  } finally {
    loading.value = false;
  }
}
async function save() {
  if (!props.characterId) return;
  if (profile.timezoneMode === "fixed" && !profile.timezone.trim()) {
    ElMessage.warning("请输入角色 IANA 时区");
    return;
  }
  if (birthday.value && !/^\d{2}-\d{2}$/.test(birthday.value)) {
    ElMessage.warning("角色生日格式应为 MM-DD");
    return;
  }
  saving.value = true;
  try {
    profile.source = "explicit";
    profile.confidence = 100;
    Object.assign(
      profile,
      await updateCharacterTemporalProfile(props.characterId, profile),
    );
    if (!birthday.value && birthdayAnchor.value) {
      await deleteTemporalAnchor(birthdayAnchor.value.id, props.characterId);
      birthdayAnchor.value = null;
    } else if (birthday.value) {
      if (profile.timezoneMode === "follow_user") {
        const currentUserProfile = await getTemporalProfile();
        userTimezone.value = currentUserProfile.timezone;
      }
      const payload = {
        ...(birthdayAnchor.value || {}),
        scopeType: "character" as const,
        characterId: props.characterId,
        anchorType: "birthday",
        title: "角色生日",
        description: "",
        timeKind: "annual_date" as const,
        localDate: birthday.value,
        localTime: "09:00",
        timezone:
          profile.timezoneMode === "follow_user"
            ? userTimezone.value
            : profile.timezone,
        importance: 90,
        confidence: 100,
        sensitivityLevel: "internal",
        allowPromptMention: true,
        allowProactiveMention: true,
        requiresConfirmation: false,
        source: "manual",
        status: "active" as const,
      };
      birthdayAnchor.value = birthdayAnchor.value
        ? await updateTemporalAnchor(birthdayAnchor.value.id, payload)
        : await createTemporalAnchor(payload);
    }
    ElMessage.success("角色时间设置已保存");
  } catch {
    ElMessage.error("保存角色时间设置失败");
  } finally {
    saving.value = false;
  }
}
watch(
  () => props.characterId,
  () => load(),
  { immediate: true },
);
</script>

<style scoped>
.temporal-section {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}
.section-title {
  margin-bottom: 4px;
}
.section-subtitle {
  font-size: 12px;
  line-height: 1.5;
  color: var(--ac-color-text-muted);
}
.temporal-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px 20px;
}
.temporal-item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 7px;
}
.temporal-item label {
  font-size: 12px;
  font-weight: 500;
  color: var(--ac-color-text-secondary);
}
.full-item {
  grid-column: 1/-1;
}
.full-width,
.el-slider {
  width: 100%;
}
.item-help {
  font-size: 11px;
  line-height: 1.45;
  color: var(--ac-color-text-muted);
}
@media (max-width: 700px) {
  .section-heading {
    flex-direction: column;
  }
  .temporal-grid {
    grid-template-columns: 1fr;
  }
  .full-item {
    grid-column: auto;
  }
}
</style>
