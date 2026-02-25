#!/bin/bash
# Проверка: запуск MCP, вызов cie_reindex, проверка .cie/index.log
set -e
cd "$(dirname "$0")/.."
CIEBIN="./cie"
[ -x "$CIEBIN" ] || CIEBIN=cie

echo "=== Запуск MCP (stdin — JSON-RPC, stderr в /tmp/cie_stderr2.log)"
rm -f .cie/index.log
(
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'
  sleep 1
  echo '{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}'
  sleep 0.3
  echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"cie_reindex","arguments":{"force_full":false}}}'
  sleep 50
) | $CIEBIN --mcp 2>/tmp/cie_stderr2.log &
PID=$!
echo "PID=$PID, ждём 55 сек..."
sleep 55
kill $PID 2>/dev/null; wait $PID 2>/dev/null || true
echo ""
echo "--- stderr (последние 50 строк) ---"
tail -50 /tmp/cie_stderr2.log
echo ""
echo "--- .cie/index.log ---"
cat .cie/index.log 2>/dev/null || echo "(нет файла)"
echo ""
echo "--- проверка: reindex в логе? ---"
grep -E "reindex started|reindex completed|reindex failed" .cie/index.log 2>/dev/null || echo "не найдено"
