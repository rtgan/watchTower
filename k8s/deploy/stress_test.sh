#!/bin/bash
# Copyright 2026 watchTower
# ================================================================
# 压测脚本 — 用于测试 watchTower HPA 自动扩缩容
# 原理: 无限循环向 backend API 发送请求，触发 CPU 飙高
#       HPA 检测到 CPU > 50% 后自动扩容 Pod
# ================================================================

set -e

KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-watchtower}"
NAMESPACE="watchtower"
BACKEND_URL="http://localhost/api/chat"
DURATION="${DURATION:-}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $(date '+%H:%M:%S') $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $(date '+%H:%M:%S') $1"; }
log_step()  { echo -e "${BLUE}[STEP]${NC} $(date '+%H:%M:%S') $1"; }

# ================================================================
# 发送一个 POST 请求到 /api/chat
# ================================================================
send_request() {
    local idx=$1
    local response
    local http_code

    response=$(curl -s -w "\n%{http_code}" \
        -X POST "$BACKEND_URL" \
        -H "Content-Type: application/json" \
        -d "{\"query\":\"压测请求 #$idx\",\"history\":[]}" \
        -o /dev/null 2>&1) || response="error"

    http_code=$(echo "$response" | tail -1)
    echo "  [$idx] HTTP: $http_code"
}

# ================================================================
# 查看 HPA 状态
# ================================================================
check_hpa() {
    echo ""
    echo "--- HPA 状态 ---"
    kubectl get hpa -n "$NAMESPACE" 2>/dev/null || echo "  无 HPA"
    echo ""
    echo "--- Backend Pods ---"
    kubectl get pods -n "$NAMESPACE" -l app=watchtower-backend \
        -o wide 2>/dev/null || echo "  无 Pod"
    echo ""
    echo "--- Backend CPU/Memory ---"
    kubectl top pods -n "$NAMESPACE" -l app=watchtower-backend 2>/dev/null \
        || echo "  metrics-server 尚未收集到数据，稍后重试"
    echo ""
}

# ================================================================
# 启动压测
# ================================================================
start_stress() {
    log_step "=============================================="
    log_step "  watchTower HPA 压测"
    log_step "  Target: $BACKEND_URL"
    log_step "=============================================="
    echo ""

    log_info "检查 Ingress / Service 是否就绪..."
    if ! kubectl get ingress -n "$NAMESPACE" &>/dev/null; then
        log_warn "Ingress 未就绪，尝试直接访问 Service..."
        BACKEND_URL="http://watchtower-backend.$NAMESPACE.svc.cluster.local:6872/api/chat"
    fi
    log_info "Backend URL: $BACKEND_URL"
    echo ""

    log_warn "开始压测... 按 Ctrl+C 停止"
    log_info "建议观察另一个终端: kubectl get hpa -n $NAMESPACE -w"
    echo ""

    count=0
    total_ok=0
    total_fail=0

    # 使用并发请求加速压测
    if command -v parallel &>/dev/null; then
        log_info "使用 GNU parallel 并发压测..."
        seq 1 9999999 | parallel -j 20 "curl -s -X POST '$BACKEND_URL' \
            -H 'Content-Type: application/json' \
            -d '{\"query\":\"压测 {}\",\"history\":[]}' \
            -o /dev/null -w '%{http_code}\n' 2>/dev/null | tail -1"
    else
        log_info "使用默认并发模式（安装 parallel 可提升压测效率）..."
        while true; do
            count=$((count + 1))

            # 同时发起 5 个并发请求
            for i in 1 2 3 4 5; do
                send_request "$count-$i" &
            done
            wait

            if [[ $((count % 10)) -eq 0 ]]; then
                echo ""
                log_info "已发送 $((count * 5)) 个请求，检查 HPA 状态:"
                check_hpa
            fi

            if [[ -n "$DURATION" && $count -ge "$DURATION" ]]; then
                log_info "达到指定时长 $DURATION 秒，停止压测"
                break
            fi

            sleep 0.5
        done
    fi
}

# ================================================================
# 停止压测（只停止脚本，压测进程由 wait 托管）
# ================================================================
stop_stress() {
    log_warn "停止压测（请等待后台任务结束）..."
    pkill -P $$ 2>/dev/null || true
}

trap 'stop_stress; exit 0' INT TERM

case "${1:-start}" in
    start)
        start_stress
        ;;
    check)
        check_hpa
        ;;
    *)
        echo "用法: $0 {start|check}"
        echo ""
        echo "  start - 启动压测（默认）"
        echo "  check - 仅查看 HPA 状态"
        echo ""
        echo "环境变量:"
        echo "  BACKEND_URL  压测目标 URL（默认: http://localhost/api/chat）"
        echo "  DURATION     压测持续秒数（默认: 无限）"
        echo ""
        exit 1
        ;;
esac
