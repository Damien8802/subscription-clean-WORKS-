// МОБИЛЬНЫЙ ДЕТЕКТОР И ФУНКЦИИ
document.addEventListener('DOMContentLoaded', function() {
    // Определяем мобильное устройство
    const isMobile = /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent);
    
    if (isMobile) {
        // Добавляем класс к body
        document.body.classList.add('mobile-device');
        
        // Скрываем/показываем элементы
        const desktopElements = document.querySelectorAll('.desktop-only');
        desktopElements.forEach(el => el.style.display = 'none');
        
        const mobileElements = document.querySelectorAll('.mobile-only');
        mobileElements.forEach(el => el.style.display = 'block');
        
        // Улучшаем тапы
        document.querySelectorAll('.btn, a').forEach(link => {
            link.style.cursor = 'pointer';
            link.addEventListener('touchstart', function() {
                this.style.opacity = '0.7';
            });
            link.addEventListener('touchend', function() {
                this.style.opacity = '1';
            });
        });
        
        // Предотвращаем масштабирование на дабл-тап
        let lastTouchEnd = 0;
        document.addEventListener('touchend', function(event) {
            const now = (new Date()).getTime();
            if (now - lastTouchEnd <= 300) {
                event.preventDefault();
            }
            lastTouchEnd = now;
        }, false);
        
        // Улучшаем скролл для iOS
        document.body.style.WebkitOverflowScrolling = 'touch';
        
        // Мобильное меню (если есть)
        const mobileMenuBtn = document.querySelector('.mobile-menu-btn');
        const mobileMenu = document.querySelector('.mobile-menu');
        
        if (mobileMenuBtn && mobileMenu) {
            mobileMenuBtn.addEventListener('click', function() {
                mobileMenu.classList.toggle('active');
            });
        }
        
        // Закрытие меню при клике вне его
        document.addEventListener('click', function(event) {
            if (mobileMenu && !mobileMenu.contains(event.target) && 
                mobileMenuBtn && !mobileMenuBtn.contains(event.target)) {
                mobileMenu.classList.remove('active');
            }
        });
        
        // Сохраняем в localStorage что это мобильное устройство
        localStorage.setItem('isMobileDevice', 'true');
        
        // Показываем уведомление (опционально)
        if (!localStorage.getItem('mobileWelcomeShown')) {
            console.log('📱 Добро пожаловать в мобильную версию!');
            localStorage.setItem('mobileWelcomeShown', 'true');
        }
    } else {
        document.body.classList.add('desktop-device');
        localStorage.setItem('isMobileDevice', 'false');
    }
    
    // Определяем тип устройства
    const isTablet = window.innerWidth >= 768 && window.innerWidth <= 1024;
    if (isTablet) {
        document.body.classList.add('tablet-device');
    }
    
    // Адаптация размера шрифта
    function adjustFontSize() {
        const width = window.innerWidth;
        const baseSize = 16;
        let scale = 1;
        
        if (width < 480) scale = 0.9;
        if (width < 360) scale = 0.85;
        if (width > 1200) scale = 1.1;
        
        document.documentElement.style.fontSize = (baseSize * scale) + 'px';
    }
    
    adjustFontSize();
    window.addEventListener('resize', adjustFontSize);
    
    // Улучшаем работу форм на мобильных
    const inputs = document.querySelectorAll('input, textarea, select');
    inputs.forEach(input => {
        input.addEventListener('focus', function() {
            // Прокручиваем к полю ввода на мобильных
            if (isMobile) {
                setTimeout(() => {
                    this.scrollIntoView({ behavior: 'smooth', block: 'center' });
                }, 300);
            }
        });
    });
});
