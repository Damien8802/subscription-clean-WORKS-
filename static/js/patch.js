// patch.js - добавляем скрипт исправления кнопок на страницу
(function() {
    console.log('Patching page with button fix...');
    
    // Создаем элемент script
    const script = document.createElement('script');
    script.src = '/static/js/fix-buttons.js';
    script.onload = function() {
        console.log('Button fix script loaded');
    };
    script.onerror = function() {
        console.log('Failed to load button fix, trying inline...');
        // Если не загрузился, добавляем inline
        addInlineFix();
    };
    
    // Добавляем в head
    document.head.appendChild(script);
    
    function addInlineFix() {
        const inlineScript = document.createElement('script');
        inlineScript.innerHTML = `
            // Простой inline fix
            setTimeout(function() {
                document.querySelectorAll('*').forEach(el => {
                    const text = (el.textContent || '').toUpperCase().trim();
                    if (text.includes('НАЧАТЬ ИСПОЛЬЗОВАТЬ')) {
                        el.onclick = function() { window.location.href = '/admin'; };
                        el.style.cursor = 'pointer';
                    }
                    if (text.includes('ПРОВЕРИТЬ СТАТУС API')) {
                        el.onclick = function() {
                            fetch('/api/health')
                                .then(r => r.json())
                                .then(data => alert('API работает!'));
                        };
                        el.style.cursor = 'pointer';
                    }
                });
            }, 1000);
        `;
        document.head.appendChild(inlineScript);
    }
})();
