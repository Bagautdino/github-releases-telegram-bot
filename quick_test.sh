#!/bin/bash

echo "🧪 Быстрый тест уведомлений о релизах"
echo "======================================"

echo "1️⃣ Останавливаем бота..."
docker-compose down > /dev/null 2>&1

echo "2️⃣ Переключаемся на тестовую конфигурацию (без LLM)..."
cp .env .env.backup 2>/dev/null || true
cp env.test .env

echo "3️⃣ Запускаем бота с тестовой конфигурацией..."
docker-compose up -d

echo "4️⃣ Ждём 5 секунд для инициализации..."
sleep 5

echo "5️⃣ Показываем логи последних 10 строк..."
docker-compose logs --tail=10

echo ""
echo "✅ Готово! Теперь:"
echo "   • Отправьте боту: /setchat -4820464052"
echo "   • Затем: /addtestrepo"  
echo "   • И наконец: /forcecheck"
echo ""
echo "📱 Вы должны получить уведомления о релизах!"
echo ""
echo "📊 Для просмотра логов в реальном времени:"
echo "   docker-compose logs -f"
echo ""
echo "🔄 Для восстановления оригинальной конфигурации:"
echo "   cp .env.backup .env && docker-compose restart"
