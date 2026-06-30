<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <teleport to="body">
    <transition name="pwa-fade">
      <div v-if="showPrompt" class="pwa-install-overlay" @click.self="dismiss">
        <div class="pwa-install-card">
          <div class="pwa-card-header">
            <div class="pwa-icon">
              <img src="/icons/icon-192.png" alt="AI-Amitia" width="48" height="48" />
            </div>
            <div class="pwa-title-group">
              <h3 class="pwa-title">Install AI-Amitia</h3>
              <p class="pwa-subtitle">Add to home screen for quick access</p>
            </div>
            <button class="pwa-close" @click="dismiss" aria-label="Close">&times;</button>
          </div>

          <div class="pwa-card-body">
            <div class="pwa-features">
              <div class="pwa-feature">
                <span class="pf-icon">&#9889;</span>
                <span>Fast access from home screen</span>
              </div>
              <div class="pwa-feature">
                <span class="pf-icon">&#128274;</span>
                <span>Data stays on your device</span>
              </div>
              <div class="pwa-feature">
                <span class="pf-icon">&#128241;</span>
                <span>Full-screen standalone mode</span>
              </div>
            </div>

            <!-- iOS install instructions -->
            <div v-if="isIOS" class="pwa-ios-hint">
              <p>Tap <strong>Share</strong> <span class="ios-icon">&#9650;</span> then <strong>Add to Home Screen</strong></p>
            </div>
          </div>

          <div class="pwa-card-footer">
            <button class="pwa-btn-secondary" @click="dismissLater">Later</button>
            <button v-if="!isIOS" class="pwa-btn-primary" @click="install">
              Install
            </button>
            <button v-else class="pwa-btn-primary" @click="dismiss">
              Got it
            </button>
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const showPrompt = ref(false)
const isIOS = ref(false)

// Store the beforeinstallprompt event
let deferredPrompt: any = null

// Check if already installed or recently dismissed
function wasRecentlyDismissed(): boolean {
  const dismissed = localStorage.getItem('pwa-install-dismissed')
  if (!dismissed) return false
  // Re-prompt after 7 days
  const dismissedTime = parseInt(dismissed, 10)
  return Date.now() - dismissedTime < 7 * 24 * 60 * 60 * 1000
}

function isStandalone(): boolean {
  return window.matchMedia('(display-mode: standalone)').matches ||
    (window.navigator as any).standalone === true
}

function detectIOS(): boolean {
  return /iphone|ipad|ipod/.test(navigator.userAgent.toLowerCase())
}

onMounted(() => {
  if (isStandalone()) return
  isIOS.value = detectIOS()

  // Listen for the install prompt event
  window.addEventListener('beforeinstallprompt', (e: Event) => {
    e.preventDefault()
    deferredPrompt = e

    if (!wasRecentlyDismissed()) {
      // Show prompt after a short delay
      setTimeout(() => {
        showPrompt.value = true
      }, 3000)
    }
  })

  // For iOS: show prompt after delay since beforeinstallprompt doesn't fire
  if (isIOS.value && !wasRecentlyDismissed()) {
    setTimeout(() => {
      showPrompt.value = true
    }, 5000)
  }

  // Listen for app installed event
  window.addEventListener('appinstalled', () => {
    showPrompt.value = false
    deferredPrompt = null
    localStorage.removeItem('pwa-install-dismissed')
  })
})

onUnmounted(() => {
  // Clean up listeners handled by Vue
})

async function install() {
  if (!deferredPrompt) return

  deferredPrompt.prompt()
  const { outcome } = await deferredPrompt.userChoice

  if (outcome === 'accepted') {
    showPrompt.value = false
    localStorage.removeItem('pwa-install-dismissed')
  } else {
    dismissLater()
  }

  deferredPrompt = null
}

function dismiss() {
  showPrompt.value = false
}

function dismissLater() {
  showPrompt.value = false
  localStorage.setItem('pwa-install-dismissed', String(Date.now()))
}
</script>

<style scoped>
.pwa-install-overlay {
  position: fixed; inset: 0; z-index: 9999;
  background: rgba(0, 0, 0, 0.45);
  display: flex; align-items: flex-end; justify-content: center;
  padding: 16px;
}

.pwa-install-card {
  background: #fff;
  border-radius: 16px 16px 0 0;
  max-width: 420px; width: 100%;
  box-shadow: 0 -4px 24px rgba(0, 0, 0, 0.15);
  overflow: hidden;
  animation: slideUp 0.3s ease-out;
}

@keyframes slideUp {
  from { transform: translateY(100%); }
  to { transform: translateY(0); }
}

.pwa-card-header {
  display: flex; align-items: center; gap: 12px;
  padding: 20px 20px 12px;
  position: relative;
}

.pwa-icon img {
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.3);
}

.pwa-title-group { flex: 1; }
.pwa-title { margin: 0; font-size: 17px; font-weight: 600; color: #1a1a2e; }
.pwa-subtitle { margin: 2px 0 0; font-size: 13px; color: #666; }

.pwa-close {
  position: absolute; top: 12px; right: 12px;
  background: none; border: none; font-size: 24px;
  color: #999; cursor: pointer; padding: 4px 8px;
  line-height: 1;
}
.pwa-close:hover { color: #333; }

.pwa-card-body {
  padding: 0 20px 16px;
}

.pwa-features {
  display: flex; flex-direction: column; gap: 8px;
  margin-bottom: 12px;
}

.pwa-feature {
  display: flex; align-items: center; gap: 10px;
  font-size: 14px; color: #444;
}

.pf-icon { font-size: 18px; width: 24px; text-align: center; }

.pwa-ios-hint {
  background: #f0f7ff;
  border-radius: 8px;
  padding: 10px 14px;
  font-size: 13px;
  color: #3B82F6;
  text-align: center;
}

.ios-icon {
  display: inline-block;
  border: 1.5px solid #3B82F6;
  border-radius: 4px;
  padding: 1px 6px;
  font-size: 11px;
  margin: 0 2px;
}

.pwa-card-footer {
  display: flex; gap: 10px;
  padding: 0 20px 20px;
}

.pwa-btn-secondary {
  flex: 1;
  padding: 12px;
  border: 1.5px solid #ddd;
  border-radius: 12px;
  background: #fff;
  font-size: 15px; font-weight: 500;
  color: #666;
  cursor: pointer;
}
.pwa-btn-secondary:hover { background: #f5f5f5; }

.pwa-btn-primary {
  flex: 2;
  padding: 12px;
  border: none;
  border-radius: 12px;
  background: #3B82F6;
  font-size: 15px; font-weight: 600;
  color: #fff;
  cursor: pointer;
}
.pwa-btn-primary:hover { background: #2563EB; }

.pwa-fade-enter-active, .pwa-fade-leave-active {
  transition: opacity 0.25s ease;
}
.pwa-fade-enter-from, .pwa-fade-leave-to {
  opacity: 0;
}

@media (display-mode: standalone) {
  .pwa-install-overlay { display: none !important; }
}
</style>
