#!/bin/bash

# Функция для проверки доступности порта
check_port() {
    local port=$1
    if netstat -tuln | grep -q ":$port "; then
        echo "Порт $port уже занят. Пожалуйста, выберите другой порт."
        return 1
    fi
    return 0
}

# Проверяем наличие файла конфигурации
if [ ! -f ./config/setup.toml ]; then
    echo "Файл конфигурации не найден. Создаем пример конфигурации..."
    mkdir -p ./config
    docker run --rm proxy-app cat /app/config/setup.toml.example > ./config/setup.toml
    echo "Пожалуйста, отредактируйте файл ./config/setup.toml и запустите скрипт снова."
    exit 1
fi

# Проверяем наличие папки templates
if [ ! -d ./templates ]; then
    echo "Папка templates не найдена. Создаем пустую папку..."
    mkdir -p ./templates
    echo "Пожалуйста, поместите файлы шаблонов в папку ./templates и перезапустите скрипт."
    exit 1
fi

# Создаем директорию для логов, если она не существует
mkdir -p ./logs

# Выводим содержимое файла для отладки
echo "Содержимое файла конфигурации:"
cat ./config/setup.toml

# Извлекаем значения портов из файла конфигурации
LOCAL_PORT=$(grep -i 'LocalPort' ./config/setup.toml | grep -o '[0-9]\+' | head -1)
CONTROL_PORT=$(grep -i 'PortControl' ./config/setup.toml | grep -o '[0-9]\+' | head -1)

# Если не найдено, пробуем альтернативные имена параметров
if [ -z "$LOCAL_PORT" ]; then
    LOCAL_PORT=$(grep -i 'localport' ./config/setup.toml | grep -o '[0-9]\+' | head -1)
fi

if [ -z "$CONTROL_PORT" ]; then
    CONTROL_PORT=$(grep -i 'portcontrol' ./config/setup.toml | grep -o '[0-9]\+' | head -1)
fi

# Проверяем, что порты были найдены
if [ -z "$LOCAL_PORT" ] || [ -z "$CONTROL_PORT" ]; then
    echo "Не удалось определить значения портов из файла конфигурации."
    echo "Убедитесь, что в файле setup.toml указаны параметры LocalPort и PortControl."
    echo "Найденные значения: LocalPort = '$LOCAL_PORT', PortControl = '$CONTROL_PORT'"
    exit 1
fi

echo "Используются порты: прокси = $LOCAL_PORT, управление = $CONTROL_PORT"

# Проверяем доступность портов
if ! check_port $LOCAL_PORT; then
    echo "Порт прокси ($LOCAL_PORT) уже занят. Пожалуйста, измените его в файле конфигурации."
    exit 1
fi

if ! check_port $CONTROL_PORT; then
    echo "Порт управления ($CONTROL_PORT) уже занят. Пожалуйста, измените его в файле конфигурации."
    exit 1
fi

# Проверяем, существует ли контейнер с таким именем, и останавливаем/удаляем его
if docker ps -a --format '{{.Names}}' | grep -q "^proxy-container$"; then
    echo "Останавливаем и удаляем существующий контейнер proxy-container..."
    docker stop proxy-container
    docker rm proxy-container
fi

# Удаляем старый контейнер и образ, если они существуют
docker rmi proxy-app 2>/dev/null || true

# Пересобираем образ с правильной структурой
docker build -t proxy-app .

# Запускаем контейнер с монтированием папки templates
# Удалите эту строку из скрипта запуска, если не хотите монтировать папку templates
#   -v $(pwd)/templates:/app/templates \

docker run -d \
  --name proxy-container \
  -p $LOCAL_PORT:$LOCAL_PORT \
  -p $CONTROL_PORT:$CONTROL_PORT \
  -v $(pwd)/logs:/app/logs \
  -v $(pwd)/config/setup.toml:/app/config/setup.toml \
  proxy-app

echo "Контейнер запущен."
