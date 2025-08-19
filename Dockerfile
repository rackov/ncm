# Этап 1: Сборка приложения
FROM golang:1.24-alpine AS builder

# Установка рабочих директорий
WORKDIR /app

# Копирование go.mod и go.sum
COPY go.mod go.sum ./

# Загрузка зависимостей
RUN go mod download

# Копирование исходного кода
COPY . .

# Сборка приложения
RUN CGO_ENABLED=0 GOOS=linux go build -o proxy .

# Этап 2: Создание финального образа
FROM alpine:latest

# Установка ca-certificates для HTTPS
RUN apk --no-cache add ca-certificates

# Создание директорий для логов и конфигурации
RUN mkdir -p /app/logs /app/config /app/templates

# Создание примера файла конфигурации
RUN echo '# Example configuration file\n# Copy this file to setup.toml and modify as needed\n[server]\nLocalPort = 8888\nPortControl = 8080\n\n[[servers]]\nName = "example.com"\nPort = 80' > /app/config/setup.toml.example

# Рабочая директория
WORKDIR /app

# Копирование бинарного файла из этапа сборки
COPY --from=builder /app/proxy .

# Копирование папки templates
COPY --from=builder /app/templates ./templates

# Создание томов для логов и конфигурации
VOLUME ["/app/logs", "/app/config"]

# Запуск приложения
CMD ["./proxy"]
