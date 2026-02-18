// static/sw-mobile.js - Mobile Service Worker
const CACHE_NAME = 'subscription-mobile-v1';
const STATIC_CACHE = [
    '/mobile',
    '/static/css/mobile/main.css',
    '/static/js/mobile/main.js',
    '/static/manifest-mobile.json'
];

self.addEventListener('install', event => {
    console.log('[Mobile SW] Installing...');
    event.waitUntil(
        caches.open(CACHE_NAME)
            .then(cache => cache.addAll(STATIC_CACHE))
            .then(() => self.skipWaiting())
    );
});

self.addEventListener('fetch', event => {
    // Только для мобильных запросов
    if (event.request.url.includes('/mobile')) {
        event.respondWith(
            caches.match(event.request)
                .then(response => {
                    if (response) {
                        return response;
                    }
                    return fetch(event.request);
                })
        );
    }
    // Для остальных запросов - стандартное поведение
});
