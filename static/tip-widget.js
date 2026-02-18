(function() {
    const script = document.currentScript;
    const widgetId = new URL(script.src).searchParams.get('widget');
    
    if (!widgetId) {
        console.error('Tip widget: No widget ID provided');
        return;
    }
    
    // Создаем контейнер если его нет
    const containerId = `tip-container-${widgetId}`;
    let container = document.getElementById(containerId);
    
    if (!container) {
        container = document.createElement('div');
        container.id = containerId;
        document.body.appendChild(container);
    }
    
    // Создаем iframe с виджетом
    const iframe = document.createElement('iframe');
    iframe.src = `/tip-widget/${widgetId}`;
    iframe.style.border = 'none';
    iframe.style.width = '100%';
    iframe.style.maxWidth = '350px';
    iframe.style.height = '500px';
    iframe.style.borderRadius = '16px';
    iframe.style.boxShadow = '0 10px 30px rgba(0,0,0,0.1)';
    iframe.style.overflow = 'hidden';
    
    container.appendChild(iframe);
    
    // Отправка сообщения о размере
    window.addEventListener('message', (event) => {
        if (event.data && event.data.type === 'tip_widget_height') {
            iframe.style.height = event.data.height + 'px';
        }
    });
    
    // Логирование для отладки
    console.log(`Tip widget ${widgetId} loaded`);
})();