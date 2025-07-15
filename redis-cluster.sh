#!/bin/bash

# Redis Cluster Management Script

set -e

COMPOSE_FILE="docker-compose.cluster.yml"

case "$1" in
    "start")
        echo "🚀 Starting Redis cluster..."
        docker-compose -f $COMPOSE_FILE up -d
        echo "✅ Redis cluster started!"
        echo "📊 Cluster info:"
        sleep 15  # Wait for cluster to initialize
        docker-compose -f $COMPOSE_FILE exec redis-node-1 redis-cli cluster info
        ;;
    
    "stop")
        echo "🛑 Stopping Redis cluster..."
        docker-compose -f $COMPOSE_FILE down
        echo "✅ Redis cluster stopped!"
        ;;
    
    "restart")
        echo "🔄 Restarting Redis cluster..."
        docker-compose -f $COMPOSE_FILE down
        docker-compose -f $COMPOSE_FILE up -d
        echo "✅ Redis cluster restarted!"
        ;;
    
    "status")
        echo "📊 Redis cluster status:"
        docker-compose -f $COMPOSE_FILE exec redis-node-1 redis-cli cluster info
        echo ""
        echo "🔗 Cluster nodes:"
        docker-compose -f $COMPOSE_FILE exec redis-node-1 redis-cli cluster nodes
        ;;
    
    "logs")
        echo "📝 Redis cluster logs:"
        docker-compose -f $COMPOSE_FILE logs -f
        ;;
    
    "clean")
        echo "🧹 Cleaning up Redis cluster data..."
        docker-compose -f $COMPOSE_FILE down -v
        echo "✅ Redis cluster data cleaned!"
        ;;
    
    "test")
        echo "🧪 Testing Redis cluster..."
        docker-compose -f $COMPOSE_FILE exec redis-node-1 redis-cli set test-key "Hello Redis Cluster"
        result=$(docker-compose -f $COMPOSE_FILE exec redis-node-2 redis-cli get test-key)
        if [ "$result" = "Hello Redis Cluster" ]; then
            echo "✅ Redis cluster is working correctly!"
        else
            echo "❌ Redis cluster test failed!"
            exit 1
        fi
        docker-compose -f $COMPOSE_FILE exec redis-node-1 redis-cli del test-key
        ;;
    
    *)
        echo "Redis Cluster Management Script"
        echo ""
        echo "Usage: $0 {start|stop|restart|status|logs|clean|test}"
        echo ""
        echo "Commands:"
        echo "  start    - Start Redis cluster"
        echo "  stop     - Stop Redis cluster"
        echo "  restart  - Restart Redis cluster"
        echo "  status   - Show cluster status and nodes"
        echo "  logs     - Show cluster logs"
        echo "  clean    - Stop cluster and remove all data"
        echo "  test     - Test cluster functionality"
        exit 1
        ;;
esac
