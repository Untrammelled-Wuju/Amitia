// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
// Service Worker for AI Companion PWA
// Version: 1.0.0

const CACHE_NAME = 'ai-companion-v2'

// Static assets to cache on install
const STATIC_ASSETS = [
  '/',
  '/index.html',
  '/manifest.webmanifest',
  '/icons/icon-192.png',
  '/icons/icon-512.png',
]

// ============================================================
// Install: cache static assets
// ============================================================
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(STATIC_ASSETS).catch((err) => {
        console.warn('[SW] Cache addAll failed:', err)
        // Continue even if some assets fail
      })
    })
  )
  // Activate immediately
  self.skipWaiting()
})

// ============================================================
// Activate: clean old caches
// ============================================================
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) => {
      return Promise.all(
        keys
          .filter((key) => key !== CACHE_NAME)
          .map((key) => caches.delete(key))
      )
    })
  )
  // Take control of all clients
  self.clients.claim()
})

// ============================================================
// Fetch: cache-first for static, network-only for API
// ============================================================
self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url)

  // Skip non-http/https requests (e.g. chrome-extension://)
  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    return
  }

  // NEVER cache API requests - they may contain sensitive data
  if (url.pathname.startsWith('/api/')) {
    // Network-only: do not cache API responses
    return
  }

  // NEVER cache auth-related paths
  if (url.pathname.includes('/auth/') || url.pathname.includes('/login') || url.pathname.includes('/setup')) {
    return
  }

  // Only cache GET requests for static resources
  if (event.request.method !== 'GET') {
    return
  }

  // Skip caching on dev server (localhost)
  if (url.hostname === 'localhost' || url.hostname === '127.0.0.1') {
    return
  }

  // Cache-first strategy for static assets (JS, CSS, images, fonts)
  const isStaticAsset =
    url.pathname.match(/\.(js|css|png|jpg|jpeg|gif|svg|ico|woff|woff2|ttf|eot|webp)$/) ||
    url.pathname === '/' ||
    url.pathname.endsWith('/index.html') ||
    url.pathname === '/manifest.webmanifest' ||
    url.pathname.startsWith('/icons/') ||
    url.pathname.startsWith('/assets/')

  if (isStaticAsset) {
    event.respondWith(
      caches.match(event.request).then((cached) => {
        if (cached) {
          // Return cached, update cache in background
          const fetchPromise = fetch(event.request).then((response) => {
            if (response && response.status === 200 && response.type === 'basic') {
              const clone = response.clone()
              caches.open(CACHE_NAME).then((cache) => {
                cache.put(event.request, clone)
              })
            }
            return response
          }).catch(() => cached)
          
          // Use cached immediately
          return cached
        }
        
        // Not in cache, fetch from network
        return fetch(event.request).then((response) => {
          if (response && response.status === 200 && response.type === 'basic') {
            const clone = response.clone()
            caches.open(CACHE_NAME).then((cache) => {
              cache.put(event.request, clone)
            })
          }
          return response
        })
      })
    )
  }
  // For everything else, let the browser handle it normally
})


// ============================================================
// Push Notifications (Step 69)
// ============================================================

self.addEventListener("push", (event) => {
  let data = {}
  try {
    if (event.data) {
      data = event.data.json()
    }
  } catch {
    // If not JSON, try text
    try {
      const text = event.data?.text() || ""
      data = { title: "通知", body: text.slice(0, 200) }
    } catch {
      data = { title: "通知", body: "" }
    }
  }

  const title = data.title || "AI Companion"
  const options = {
    body: (data.body || "").slice(0, 300),
    icon: data.icon || "/icons/icon-192.png",
    badge: data.badge || "/icons/icon-192.png",
    tag: data.tag || "default",
    data: data.data || {},
    requireInteraction: false,
    silent: false,
  }

  event.waitUntil(
    self.registration.showNotification(title, options).catch((err) => {
      console.warn("[SW] showNotification failed:", err)
    })
  )
})

self.addEventListener("notificationclick", (event) => {
  event.notification.close()

  event.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((clients) => {
      // Focus existing window if any
      for (const client of clients) {
        if ("focus" in client) {
          client.focus()
          return
        }
      }
      // Otherwise open new window
      if (self.clients.openWindow) {
        return self.clients.openWindow("/")
      }
    })
  )
})
