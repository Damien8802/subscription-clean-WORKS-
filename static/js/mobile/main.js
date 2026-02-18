// static/js/mobile/main.js - Mobile functionality
class MobileApp {
    constructor() {
        this.init();
    }
    
    init() {
        console.log('Mobile app initialized');
        this.setupServiceWorker();
        this.setupOfflineDetection();
        this.loadStats();
    }
    
    setupServiceWorker() {
        if ('serviceWorker' in navigator) {
            navigator.serviceWorker.register('/sw-mobile.js')
                .then(reg => console.log('Mobile SW registered:', reg))
                .catch(err => console.log('Mobile SW failed:', err));
        }
    }
    
    setupOfflineDetection() {
        window.addEventListener('online', () => {
            this.showToast('Соединение восстановлено', 'success');
        });
        
        window.addEventListener('offline', () => {
            this.showToast('Нет соединения', 'warning');
        });
    }
    
    async loadStats() {
        try {
            const response = await fetch('/mobile/api/stats');
            const data = await response.json();
            this.updateDashboard(data);
        } catch (error) {
            console.error('Failed to load stats:', error);
        }
    }
    
    updateDashboard(data) {
        // Обновляем мобильные карточки
        const elements = {
            'revenue': document.querySelector('.stat-revenue'),
            'users': document.querySelector('.stat-users'),
            'active': document.querySelector('.stat-active'),
            'churn': document.querySelector('.stat-churn')
        };
        
        for (const [key, element] of Object.entries(elements)) {
            if (element && data[key]) {
                element.textContent = data[key];
            }
        }
    }
    
    showToast(message, type = 'info') {
        // Простой toast для мобильных
        const toast = document.createElement('div');
        toast.style.cssText = \
            position: fixed;
            bottom: 80px;
            left: 50%;
            transform: translateX(-50%);
            background: \;
            color: white;
            padding: 12px 24px;
            border-radius: 8px;
            z-index: 9999;
            animation: slideUp 0.3s ease;
        \;
        toast.textContent = message;
        document.body.appendChild(toast);
        
        setTimeout(() => {
            toast.remove();
        }, 3000);
    }
}

// Инициализация при загрузке
document.addEventListener('DOMContentLoaded', () => {
    window.mobileApp = new MobileApp();
});
