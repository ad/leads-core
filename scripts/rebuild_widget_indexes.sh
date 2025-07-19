#!/bin/bash

# Script to rebuild Redis widget indexes
# This should be run when widget indexing is inconsistent

echo "Rebuilding Redis widget indexes..."

REDIS_HOST=${REDIS_HOST:-localhost}
REDIS_PORT=${REDIS_PORT:-6379}

# Clear existing index keys
echo "Clearing existing indexes..."
redis-cli -h $REDIS_HOST -p $REDIS_PORT DEL "widgets:isVisible:0"
redis-cli -h $REDIS_HOST -p $REDIS_PORT DEL "widgets:isVisible:1"
redis-cli -h $REDIS_HOST -p $REDIS_PORT DEL "widgets:type:banner"
redis-cli -h $REDIS_HOST -p $REDIS_PORT DEL "widgets:type:action"
redis-cli -h $REDIS_HOST -p $REDIS_PORT DEL "widgets:type:lead-form"
redis-cli -h $REDIS_HOST -p $REDIS_PORT DEL "widgets:type:quiz"
redis-cli -h $REDIS_HOST -p $REDIS_PORT DEL "widgets:type:social-proof"
redis-cli -h $REDIS_HOST -p $REDIS_PORT DEL "widgets:type:live-interest"
redis-cli -h $REDIS_HOST -p $REDIS_PORT DEL "widgets:type:widget-tab"
redis-cli -h $REDIS_HOST -p $REDIS_PORT DEL "widgets:type:sticky-bar"
redis-cli -h $REDIS_HOST -p $REDIS_PORT DEL "widgets:type:wheelOfFortune"
redis-cli -h $REDIS_HOST -p $REDIS_PORT DEL "widgets:type:survey"
redis-cli -h $REDIS_HOST -p $REDIS_PORT DEL "widgets:type:coupon"

echo "Rebuilding indexes from existing widgets..."

# Get all widget keys
WIDGET_KEYS=$(redis-cli -h $REDIS_HOST -p $REDIS_PORT KEYS "*:widget" | tr '\n' ' ')

for key in $WIDGET_KEYS; do
    echo "Processing $key..."
    
    # Extract widget data
    ID=$(redis-cli -h $REDIS_HOST -p $REDIS_PORT HGET "$key" "id")
    TYPE=$(redis-cli -h $REDIS_HOST -p $REDIS_PORT HGET "$key" "type")
    IS_VISIBLE=$(redis-cli -h $REDIS_HOST -p $REDIS_PORT HGET "$key" "isVisible")
    
    if [ -n "$ID" ] && [ -n "$TYPE" ] && [ -n "$IS_VISIBLE" ]; then
        # Add to type index
        redis-cli -h $REDIS_HOST -p $REDIS_PORT SADD "widgets:type:$TYPE" "$ID"
        
        # Add to visibility index
        if [ "$IS_VISIBLE" = "true" ]; then
            redis-cli -h $REDIS_HOST -p $REDIS_PORT SADD "widgets:isVisible:1" "$ID"
        else
            redis-cli -h $REDIS_HOST -p $REDIS_PORT SADD "widgets:isVisible:0" "$ID"
        fi
        
        echo "  Added widget $ID (type: $TYPE, visible: $IS_VISIBLE)"
    fi
done

echo "Index rebuild complete!"
echo
echo "Index statistics:"
redis-cli -h $REDIS_HOST -p $REDIS_PORT SCARD "widgets:isVisible:1"
echo "visible widgets"
redis-cli -h $REDIS_HOST -p $REDIS_PORT SCARD "widgets:isVisible:0" 
echo "hidden widgets"
