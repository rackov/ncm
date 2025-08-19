#!/bin/bash

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

# Проверяем, существует ли контейнер с таким именем, и останавливаем/удаляем его
if docker ps -a --format '{{.Names}}' | grep -q "^proxy-container$"; then
    echo "Останавливаем и удаляем существующий контейнер proxy-container..."
    docker stop proxy-container
    docker rm proxy-container
fi

# Проверяем, существует ли образ, и удаляем его перед пересборкой
if docker images --format '{{.Repository}}:{{.Tag}}' | grep -q "^proxy-app:latest$"; then
    echo "Удаляем существующий образ proxy-app:latest..."
    docker rmi proxy-app:latest
fi

# Пересобираем образ
echo "Собираем Docker-образ..."
docker build -t proxy-app .

# Проверяем, что образ успешно собран
if ! docker images --format '{{.Repository}}:{{.Tag}}' | grep -q "^proxy-app:latest$"; then
    echo "Ошибка при сборке Docker-образа. Пожалуйста, проверьте вывод выше."
    exit 1
fi

# Запускаем контейнер с пробросом всех портов на хост-систему
echo "Запускаем контейнер..."
CONTAINER_ID=$(docker run -d \
  --name proxy-container \
  --network host \
  -v $(pwd)/logs:/app/logs \
  -v $(pwd)/config/setup.toml:/app/config/setup.toml \
  -v $(pwd)/templates:/app/templates \
  proxy-app)

# Проверяем, что контейнер успешно запущен
if [ -z "$CONTAINER_ID" ]; then
    echo "Ошибка при запуске контейнера. Пожалуйста, проверьте вывод выше."
    exit 1
fi

# Получаем информацию о контейнере
echo "Контейнер запущен с ID: $CONTAINER_ID"
echo "Приложение использует порты из файла конфигурации: прокси = $LOCAL_PORT, управление = $CONTROL_PORT"

# Создаем скрипт для перезапуска контейнера
cat > restart-proxy.sh << 'EOF'
#!/bin/bash

# Останавливаем контейнер
echo "Останавливаем контейнер..."
docker stop proxy-container

# Запускаем контейнер
echo "Запускаем контейнер..."
docker start proxy-container

echo "Контейнер перезапущен. Приложение использует порты из файла конфигурации."
EOF

# Делаем скрипт перезапуска исполняемым
chmod +x restart-proxy.sh

echo "Для перезапуска контейнера после изменения файла конфигурации используйте скрипт ./restart-proxy.sh"
echo "Приложение будет использовать порты, указанные в файле ./config/setup.toml"
