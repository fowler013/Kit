#!/bin/bash
# Kit Bot Service Management Script

BOT_NAME="slack-ai-bot"
BOT_PATH="/Users/tevinfowler/Documents/Kit"
LOG_FILE="$BOT_PATH/bot.log"

case "$1" in
    start)
        echo "Starting Kit AI Bot..."
        cd "$BOT_PATH"
        nohup ./$BOT_NAME > "$LOG_FILE" 2>&1 &
        echo $! > "$BOT_PATH/bot.pid"
        echo "Bot started with PID $(cat $BOT_PATH/bot.pid)"
        ;;
    stop)
        if [ -f "$BOT_PATH/bot.pid" ]; then
            PID=$(cat "$BOT_PATH/bot.pid")
            echo "Stopping Kit AI Bot (PID: $PID)..."
            kill $PID
            rm "$BOT_PATH/bot.pid"
            echo "Bot stopped"
        else
            echo "Bot is not running"
        fi
        ;;
    restart)
        $0 stop
        sleep 2
        $0 start
        ;;
    status)
        if [ -f "$BOT_PATH/bot.pid" ]; then
            PID=$(cat "$BOT_PATH/bot.pid")
            if ps -p $PID > /dev/null; then
                echo "Kit AI Bot is running (PID: $PID)"
            else
                echo "Bot PID file exists but process is not running"
                rm "$BOT_PATH/bot.pid"
            fi
        else
            echo "Kit AI Bot is not running"
        fi
        ;;
    logs)
        tail -f "$LOG_FILE"
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|logs}"
        exit 1
        ;;
esac
