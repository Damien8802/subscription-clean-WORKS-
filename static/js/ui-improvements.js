// ui-improvements/js/main.js

class SubscriptionDashboard {
    constructor() {
        this.theme = localStorage.getItem('theme') || 'light';
        this.widgets = [];
        this.init();
    }
    
    init() {
        this.setupTheme();
        this.setupWidgets();
        this.setupEventListeners();
        this.loadData();
    }
    
    setupTheme() {
        document.documentElement.setAttribute('data-theme', this.theme);
        this.updateThemeButton();
    }
    
    toggleTheme() {
        this.theme = this.theme === 'light' ? 'dark' : 'light';
        document.documentElement.setAttribute('data-theme', this.theme);
        localStorage.setItem('theme', this.theme);
        this.updateThemeButton();
    }
    
    updateThemeButton() {
        const btn = document.getElementById('themeToggle');
        if (btn) {
            btn.innerHTML = this.theme === 'light' ? '🌙' : '☀️';
        }
    }
    
    setupWidgets() {
        // Виджеты по умолчанию
        this.widgets = [
            { id: 'stats', title: 'Статистика', type: 'stats', enabled: true },
            { id: 'revenue', title: 'Доходы', type: 'chart', enabled: true },
            { id: 'users', title: 'Пользователи', type: 'list', enabled: true },
            { id: 'subscriptions', title: 'Подписки', type: 'table', enabled: true }
        ];
        
        this.renderWidgets();
    }
    
    renderWidgets() {
        const container = document.getElementById('widgetsContainer');
        if (!container) return;
        
        container.innerHTML = this.widgets
            .filter(w => w.enabled)
            .map(widget => this.createWidgetHTML(widget))
            .join('');
    }
    
    createWidgetHTML(widget) {
        return `
            <div class="widget" id="widget-${widget.id}" data-widget="${widget.id}">
                <div class="widget-header">
                    <h3 class="widget-title">${widget.title}</h3>
                    <button class="btn btn-small" onclick="dashboard.toggleWidget('${widget.id}')">
                        ✕
                    </button>
                </div>
                <div class="widget-content">
                    ${this.getWidgetContent(widget)}
                </div>
            </div>
        `;
    }
    
    getWidgetContent(widget) {
        switch(widget.type) {
            case 'stats':
                return '<div class="stats-widget">Загрузка статистики...</div>';
            case 'chart':
                return '<canvas class="chart-container"></canvas>';
            case 'list':
                return '<ul class="user-list"></ul>';
            case 'table':
                return '<table class="data-table"></table>';
            default:
                return '<p>Виджет</p>';
        }
    }
    
    toggleWidget(widgetId) {
        const widget = this.widgets.find(w => w.id === widgetId);
        if (widget) {
            widget.enabled = !widget.enabled;
            this.renderWidgets();
            this.saveWidgetsConfig();
        }
    }
    
    saveWidgetsConfig() {
        localStorage.setItem('dashboardWidgets', JSON.stringify(this.widgets));
    }
    
    loadWidgetsConfig() {
        const saved = localStorage.getItem('dashboardWidgets');
        if (saved) {
            this.widgets = JSON.parse(saved);
            this.renderWidgets();
        }
    }
    
    async loadData() {
        try {
            const response = await fetch('/api/stats');
            const data = await response.json();
            this.updateStats(data);
        } catch (error) {
            console.error('Ошибка загрузки данных:', error);
        }
    }
    
    updateStats(data) {
        // Обновляем карточки статистики
        const cards = document.querySelectorAll('[data-stat]');
        cards.forEach(card => {
            const stat = card.getAttribute('data-stat');
            if (data[stat] !== undefined) {
                const valueEl = card.querySelector('.card-value');
                if (valueEl) {
                    valueEl.textContent = data[stat];
                }
            }
        });
    }
    
    exportToCSV() {
        const data = this.getExportData();
        const csv = this.convertToCSV(data);
        this.downloadCSV(csv, 'subscriptions-export.csv');
    }
    
    getExportData() {
        // Здесь будет реальная логика экспорта
        return [
            { id: 1, name: 'Netflix', price: 599, user: 'user1' },
            { id: 2, name: 'Spotify', price: 299, user: 'user1' },
            { id: 3, name: 'YouTube', price: 349, user: 'user2' }
        ];
    }
    
    convertToCSV(data) {
        const headers = Object.keys(data[0]).join(',');
        const rows = data.map(row => Object.values(row).join(','));
        return [headers, ...rows].join('\n');
    }
    
    downloadCSV(content, filename) {
        const blob = new Blob([content], { type: 'text/csv' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        a.click();
        window.URL.revokeObjectURL(url);
    }
    
    setupEventListeners() {
        // Кнопка темы
        document.getElementById('themeToggle')?.addEventListener('click', () => this.toggleTheme());
        
        // Кнопка экспорта
        document.getElementById('exportBtn')?.addEventListener('click', () => this.exportToCSV());
        
        // Поиск
        document.getElementById('searchInput')?.addEventListener('input', (e) => this.handleSearch(e));
    }
    
    handleSearch(event) {
        const searchTerm = event.target.value.toLowerCase();
        // Логика поиска
        console.log('Поиск:', searchTerm);
    }
}

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', () => {
    window.dashboard = new SubscriptionDashboard();
});
