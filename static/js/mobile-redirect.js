// static/js/mobile-redirect.js - Safe mobile detection
(function() {
    'use strict';
    
    // Проверяем только на главной странице
    if (window.location.pathname === '/' || window.location.pathname === '') {
        // Проверка на мобильное устройство
        const isMobile = /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i
            .test(navigator.userAgent);
        
        // Проверка ширины экрана
        const isSmallScreen = window.innerWidth <= 768;
        
        if (isMobile || isSmallScreen) {
            // Создаем баннер предложения
            const banner = document.createElement('div');
            banner.className = 'mobile-redirect-banner desktop-only';
            banner.innerHTML = \
                <div style="display: flex; justify-content: space-between; align-items: center;">
                    <span>📱 Доступна мобильная версия</span>
                    <div>
                        <button onclick="window.location.href='/mobile'" 
                                style="background: white; color: #4f46e5; border: none; 
                                       padding: 8px 16px; border-radius: 8px; margin-right: 8px;">
                            Перейти
                        </button>
                        <button onclick="this.parentElement.parentElement.style.display='none'" 
                                style="background: transparent; color: white; border: 1px solid white; 
                                       padding: 8px 16px; border-radius: 8px;">
                            Позже
                        </button>
                    </div>
                </div>
            \;
            
            // Добавляем после 3 секунд
            setTimeout(() => {
                document.body.appendChild(banner);
            }, 3000);
        }
    }
})();
