// fix-buttons.js - исправляем кнопки без изменения проекта
console.log('=== FIX BUTTONS LOADED ===');

// Ждем загрузки страницы
document.addEventListener('DOMContentLoaded', function() {
    console.log('Page loaded, fixing buttons...');
    
    // 1. Ищем кнопки по тексту
    function findAndFixButtons() {
        const allElements = document.querySelectorAll('*');
        let fixedCount = 0;
        
        allElements.forEach(element => {
            const text = (element.textContent || '').toUpperCase().trim();
            
            // Кнопка "НАЧАТЬ ИСПОЛЬЗОВАТЬ"
            if (text.includes('НАЧАТЬ ИСПОЛЬЗОВАТЬ') || 
                text.includes('НАЧАТЬ') && text.includes('ИСПОЛЬЗОВАТЬ')) {
                console.log('Found START button:', element);
                
                // Удаляем все старые обработчики
                const newElement = element.cloneNode(true);
                element.parentNode.replaceChild(newElement, element);
                
                // Добавляем новый обработчик
                newElement.style.cursor = 'pointer';
                newElement.addEventListener('click', function(e) {
                    e.preventDefault();
                    e.stopPropagation();
                    console.log('START button clicked');
                    alert('🚀 Начинаем использование!\nПереходим в админ панель...');
                    window.location.href = '/admin';
                    return false;
                });
                
                fixedCount++;
            }
            
            // Кнопка "ПРОВЕРИТЬ СТАТУС API"
            if (text.includes('ПРОВЕРИТЬ СТАТУС API') || 
                text.includes('ПРОВЕРИТЬ') && text.includes('API')) {
                console.log('Found API button:', element);
                
                // Удаляем все старые обработчики
                const newElement = element.cloneNode(true);
                element.parentNode.replaceChild(newElement, element);
                
                // Добавляем новый обработчик
                newElement.style.cursor = 'pointer';
                newElement.addEventListener('click', function(e) {
                    e.preventDefault();
                    e.stopPropagation();
                    console.log('API button clicked');
                    
                    // Показываем загрузку
                    const originalText = newElement.innerHTML;
                    newElement.innerHTML = '⏳ Проверка API...';
                    
                    fetch('/api/health')
                        .then(response => response.json())
                        .then(data => {
                            alert('✅ API работает!\nСтатус: ' + data.status + '\nБаза: ' + data.database);
                            newElement.innerHTML = originalText;
                            window.open('/api/health', '_blank');
                        })
                        .catch(error => {
                            alert('❌ Ошибка API: ' + error.message);
                            newElement.innerHTML = originalText;
                        });
                    
                    return false;
                });
                
                fixedCount++;
            }
        });
        
        return fixedCount;
    }
    
    // 2. Запускаем исправление
    let fixedButtons = findAndFixButtons();
    
    // 3. Если не нашли кнопок, пробуем через 1 секунду (для динамического контента)
    if (fixedButtons === 0) {
        setTimeout(() => {
            fixedButtons = findAndFixButtons();
            if (fixedButtons === 0) {
                console.log('No buttons found, creating new ones...');
                createFallbackButtons();
            }
        }, 1000);
    }
    
    console.log('Fixed ' + fixedButtons + ' buttons');
});

// 4. Создаем запасные кнопки если не нашли существующие
function createFallbackButtons() {
    console.log('Creating fallback buttons...');
    
    // Кнопка "НАЧАТЬ ИСПОЛЬЗОВАТЬ"
    const startBtn = document.createElement('button');
    startBtn.id = 'fallback-start-btn';
    startBtn.innerHTML = '🚀 НАЧАТЬ ИСПОЛЬЗОВАТЬ';
    startBtn.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        z-index: 9999;
        background: linear-gradient(135deg, #ff6b6b, #ee5a52);
        color: white;
        padding: 12px 24px;
        border: none;
        border-radius: 8px;
        font-size: 16px;
        font-weight: bold;
        cursor: pointer;
        box-shadow: 0 4px 15px rgba(255, 107, 107, 0.3);
    `;
    
    startBtn.addEventListener('click', function() {
        alert('🚀 Начинаем использование!');
        window.location.href = '/admin';
    });
    
    // Кнопка "ПРОВЕРИТЬ СТАТУС API"
    const apiBtn = document.createElement('button');
    apiBtn.id = 'fallback-api-btn';
    apiBtn.innerHTML = '🔍 ПРОВЕРИТЬ СТАТУС API';
    apiBtn.style.cssText = `
        position: fixed;
        top: 70px;
        right: 20px;
        z-index: 9999;
        background: linear-gradient(135deg, #2ed573, #1dd1a1);
        color: white;
        padding: 12px 24px;
        border: none;
        border-radius: 8px;
        font-size: 16px;
        font-weight: bold;
        cursor: pointer;
        box-shadow: 0 4px 15px rgba(46, 213, 115, 0.3);
    `;
    
    apiBtn.addEventListener('click', function() {
        alert('🔍 Проверяем API...');
        
        fetch('/api/health')
            .then(response => response.json())
            .then(data => {
                alert('✅ API работает!\nСтатус: ' + data.status);
                window.open('/api/health', '_blank');
            })
            .catch(error => {
                alert('❌ Ошибка API: ' + error.message);
            });
    });
    
    // Добавляем кнопки на страницу
    document.body.appendChild(startBtn);
    document.body.appendChild(apiBtn);
    
    console.log('Fallback buttons created');
}

// 5. Также добавляем глобальные функции для кнопок
window.fixButtons = function() {
    const allElements = document.querySelectorAll('*');
    
    allElements.forEach(element => {
        const text = (element.textContent || '').toUpperCase().trim();
        
        if (text.includes('НАЧАТЬ ИСПОЛЬЗОВАТЬ')) {
            element.onclick = function() {
                window.location.href = '/admin';
            };
            element.style.cursor = 'pointer';
        }
        
        if (text.includes('ПРОВЕРИТЬ СТАТУС API')) {
            element.onclick = function() {
                fetch('/api/health')
                    .then(r => r.json())
                    .then(data => {
                        alert('API работает! Статус: ' + data.status);
                    });
            };
            element.style.cursor = 'pointer';
        }
    });
};

console.log('Button fix script loaded successfully');
