# Redis Cluster Configuration

## Overview

Этот проект поддерживает два режима работы с Redis:

1. **Single Redis Instance** (`docker-compose.yml`) - для разработки
2. **Redis Cluster** (`docker-compose.cluster.yml`) - для production-like тестирования

## Redis Cluster Architecture

### Topology

Redis кластер состоит из 6 узлов:
- **3 Master ноды**: `redis-node-1`, `redis-node-2`, `redis-node-3`
- **3 Replica ноды**: `redis-node-4`, `redis-node-5`, `redis-node-6`

Каждая Master нода имеет одну Replica для обеспечения высокой доступности.

### Port Mapping

- `redis-node-1`: `7001:6379`
- `redis-node-2`: `7002:6379`
- `redis-node-3`: `7003:6379`
- `redis-node-4`: `7004:6379`
- `redis-node-5`: `7005:6379`
- `redis-node-6`: `7006:6379`

### Configuration

Каждый узел кластера настроен с следующими параметрами:
```
--cluster-enabled yes
--cluster-config-file nodes.conf
--cluster-node-timeout 5000
--appendonly yes
```

### Initialization

Кластер автоматически инициализируется с помощью сервиса `redis-cluster-init`:
```bash
redis-cli --cluster create \
  redis-node-1:6379 redis-node-2:6379 redis-node-3:6379 \
  redis-node-4:6379 redis-node-5:6379 redis-node-6:6379 \
  --cluster-replicas 1 --cluster-yes
```

## Application Configuration

Приложение автоматически определяет тип Redis deployment по количеству адресов в `REDIS_ADDRESSES`:

**Single Instance:**
```env
REDIS_ADDRESSES=redis:6379
```

**Cluster:**
```env
REDIS_ADDRESSES=redis-node-1:6379,redis-node-2:6379,redis-node-3:6379,redis-node-4:6379,redis-node-5:6379,redis-node-6:6379
```

## Data Distribution

В Redis кластере данные автоматически распределяются по hash slots:
- Всего: 16384 hash slots
- Master-1: slots 0-5460
- Master-2: slots 5461-10922  
- Master-3: slots 10923-16383

## Monitoring and Management

### Health Checks

Каждый узел имеет health check:
```yaml
healthcheck:
  test: ["CMD", "redis-cli", "ping"]
  interval: 5s
  timeout: 3s
  retries: 5
```

### Management Script

Используйте `redis-cluster.sh` для управления кластером:

```bash
# Запуск кластера
./redis-cluster.sh start

# Проверка статуса
./redis-cluster.sh status

# Тестирование
./redis-cluster.sh test

# Просмотр логов
./redis-cluster.sh logs

# Остановка
./redis-cluster.sh stop

# Очистка данных
./redis-cluster.sh clean
```

### Manual Cluster Commands

Для ручного управления кластером:

```bash
# Подключение к узлу
docker-compose -f docker-compose.cluster.yml exec redis-node-1 redis-cli

# Информация о кластере
docker-compose -f docker-compose.cluster.yml exec redis-node-1 redis-cli cluster info

# Список узлов
docker-compose -f docker-compose.cluster.yml exec redis-node-1 redis-cli cluster nodes

# Состояние slots
docker-compose -f docker-compose.cluster.yml exec redis-node-1 redis-cli cluster slots
```

## Failover Testing

Для тестирования failover:

```bash
# Остановить master узел
docker-compose -f docker-compose.cluster.yml stop redis-node-1

# Проверить статус кластера
./redis-cluster.sh status

# Запустить узел обратно
docker-compose -f docker-compose.cluster.yml start redis-node-1
```

## Performance Considerations

### Преимущества кластера:
- Горизонтальное масштабирование
- Высокая доступность
- Автоматическое распределение данных
- Failover при отказе узлов

### Особенности:
- Не все Redis команды поддерживаются в кластере
- Транзакции ограничены одним hash slot
- Lua скрипты должны работать с ключами из одного slot

## Migration from Single to Cluster

При переходе с single instance на cluster:

1. Экспортировать данные из single instance
2. Запустить кластер
3. Импортировать данные в кластер
4. Обновить конфигурацию приложения

```bash
# Экспорт из single instance
docker-compose exec redis redis-cli --rdb dump.rdb

# Импорт в кластер (требует дополнительной настройки)
# Рекомендуется миграция через приложение
```
