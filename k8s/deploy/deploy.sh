#!/bin/bash
# Copyright 2026 watchTower
# ================================================================
# watchTower Kubernetes 部署脚本
# 适用环境: Kind (Kubernetes in Docker)
# ================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DEPLOY_DIR="$SCRIPT_DIR"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-watchtower}"
NAMESPACE="watchtower"
TIMEOUT=300

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step()  { echo -e "${BLUE}[STEP]${NC} $1"; }

# ================================================================
# 等待 Pod Ready
# ================================================================
wait_for_pods() {
    local label=$1
    local expected=$2
    local name=$3

    log_info "等待 $name Pod 就绪 (expected: $expected)..."
    count=0
    while true; do
        ready=$(kubectl get pods -n "$NAMESPACE" -l "$label" \
            -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "")
        ready_count=$(echo "$ready" | tr ' ' '\n' | grep -c "True" || echo 0)

        echo "  [$count/${TIMEOUT}s] Ready: $ready_count/$expected"

        if [[ "$ready_count" -ge "$expected" ]]; then
            log_info "$name 就绪 ✓"
            return 0
        fi

        if [[ $count -ge $TIMEOUT ]]; then
            log_error "$name 部署超时!"
            kubectl get pods -n "$NAMESPACE" -l "$label"
            return 1
        fi

        sleep 5
        count=$((count + 5))
    done
}

# ================================================================
# 等待 Service Ready（检查 endpoints）
# ================================================================
wait_for_service() {
    local svc=$1
    local name=$2

    log_info "等待 $name Service 就绪..."
    count=0
    while true; do
        endpoints=$(kubectl get endpoints "$svc" -n "$NAMESPACE" \
            -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null || echo "")
        if [[ -n "$endpoints" ]]; then
            log_info "$name Service 就绪 ✓"
            return 0
        fi
        if [[ $count -ge $TIMEOUT ]]; then
            log_error "$name Service 超时!"
            return 1
        fi
        sleep 5
        count=$((count + 5))
    done
}

# ================================================================
# Step 0: 检查依赖工具
# ================================================================
check_dependencies() {
    log_step "检查依赖工具..."

    for cmd in kind kubectl docker; do
        if ! command -v "$cmd" &> /dev/null; then
            log_error "$cmd 未安装，请先安装"
            exit 1
        fi
    done
    log_info "所有依赖工具已就绪 ✓"
}

# ================================================================
# Step 1: 创建/更新 Kind 集群
# ================================================================
create_cluster() {
    log_step "========== 创建/更新 Kind 集群 =========="

    if kind get clusters 2>/dev/null | grep -q "^${KIND_CLUSTER_NAME}$"; then
        log_warn "集群 '$KIND_CLUSTER_NAME' 已存在，跳过创建"
        return 0
    fi

    kubectl config use-context kind-${KIND_CLUSTER_NAME} 2>/dev/null || true

    kind create cluster \
        --name "$KIND_CLUSTER_NAME" \
        --config "$DEPLOY_DIR/00-kind-cluster.yml" \
        --wait 120s

    log_info "Kind 集群 '$KIND_CLUSTER_NAME' 创建成功 ✓"
}

# ================================================================
# Step 2: 为 control-plane 节点打标签（Ingress 调度）
# ================================================================
label_nodes() {
    log_step "========== 标记 control-plane 节点 =========="

    kubectl label node ${KIND_CLUSTER_NAME}-control-plane \
        ingress-ready=true --overwrite 2>/dev/null || true
    log_info "control-plane 节点标记完成 ✓"
}

# ================================================================
# Step 3: 构建并加载镜像到 Kind
# ================================================================
build_and_load_images() {
    log_step "========== 构建并加载镜像到 Kind =========="

    local root_dir
    root_dir="$(cd "$SCRIPT_DIR/../.." && pwd)"

    # 3.1 mock-prometheus
    if kubectl get deployment mock-prometheus -n "$NAMESPACE" &>/dev/null; then
        log_warn "mock-prometheus 镜像已存在，跳过构建"
    else
        log_info "构建 watchtower/mock-prometheus:v1.0.0..."
        docker build \
            -f "$root_dir/docker/Dockerfile.mock-prometheus" \
            -t watchtower/mock-prometheus:v1.0.0 \
            "$root_dir" \
            --no-cache
        kind load docker-image watchtower/mock-prometheus:v1.0.0 \
            --name "$KIND_CLUSTER_NAME"
    fi

    # 3.2 backend
    if kubectl get deployment watchtower-backend -n "$NAMESPACE" &>/dev/null; then
        log_warn "watchtower/backend 镜像已存在，跳过构建"
    else
        log_info "构建 watchtower/backend:v1.0.0..."
        docker build \
            -f "$SCRIPT_DIR/../backend/Dockerfile" \
            -t watchtower/backend:v1.0.0 \
            "$root_dir" \
            --no-cache
        kind load docker-image watchtower/backend:v1.0.0 \
            --name "$KIND_CLUSTER_NAME"
    fi

    # 3.3 frontend（复用 docker/ 目录下的 Dockerfile）
    # K8s 环境：前端通过 Ingress 访问后端 API，使用相对路径 /api
    log_info "构建 watchtower/frontend:v1.0.0..."
    docker build \
        -f "$root_dir/docker/Dockerfile.frontend" \
        -t watchtower/frontend:v1.0.0 \
        --build-arg NGINX_API_BASE=/api \
        "$root_dir" \
        --no-cache
    kind load docker-image watchtower/frontend:v1.0.0 \
        --name "$KIND_CLUSTER_NAME"

    log_info "所有镜像加载完成 ✓"
}

# ================================================================
# Step 4: 部署 Ingress Controller
# ================================================================
deploy_ingress_controller() {
    log_step "========== 部署 Ingress Controller =========="

    if kubectl get deployment -n ingress-nginx ingress-nginx-controller &>/dev/null; then
        log_warn "Ingress Controller 已部署，跳过"
    else
        kubectl apply -f "$DEPLOY_DIR/01-ingress-controller.yml"
        log_info "Ingress Controller 部署中，等待就绪..."

        count=0
        while true; do
            ready=$(kubectl get deployment -n ingress-nginx ingress-nginx-controller \
                -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
            echo "  [$count/120s] Ingress Controller readyReplicas: $ready"
            if [[ "$ready" == "1" ]]; then
                log_info "Ingress Controller 就绪 ✓"
                break
            fi
            if [[ $count -ge 120 ]]; then
                log_error "Ingress Controller 部署超时!"
                exit 1
            fi
            sleep 5
            count=$((count + 5))
        done
    fi
}

# ================================================================
# Step 5: 部署基础设施（etcd, minio, milvus）
# ================================================================
deploy_infrastructure() {
    log_step "========== 部署基础设施服务 =========="

    kubectl apply -f "$DEPLOY_DIR/02-namespace-configmap.yml"

    # etcd
    kubectl apply -f "$DEPLOY_DIR/03-statefulset-etcd.yml"
    wait_for_pods "app=milvus-etcd" 1 "etcd"

    # minio
    kubectl apply -f "$DEPLOY_DIR/04-statefulset-minio.yml"
    wait_for_pods "app=milvus-minio" 1 "MinIO"

    # milvus（依赖 etcd + minio）
    kubectl apply -f "$DEPLOY_DIR/05-statefulset-milvus.yml"
    wait_for_pods "app=milvus-standalone" 1 "Milvus"

    log_info "基础设施服务全部就绪 ✓"
}

# ================================================================
# Step 6: 部署 mock-prometheus
# ================================================================
deploy_mock_prometheus() {
    log_step "========== 部署 mock Prometheus =========="

    kubectl apply -f "$DEPLOY_DIR/06-mock-prometheus.yml"
    wait_for_pods "app=mock-prometheus" 1 "mock-prometheus"
    wait_for_service "mock-prometheus" "mock-prometheus"

    log_info "mock-prometheus 就绪 ✓"
}

# ================================================================
# Step 7: 部署 watchTower Backend + Frontend
# ================================================================
deploy_watchtower() {
    log_step "========== 部署 watchTower Backend + Frontend =========="

    kubectl apply -f "$DEPLOY_DIR/07-watchtower.yml"
    wait_for_pods "app=watchtower-backend" 2 "watchTower Backend"
    wait_for_service "watchtower-backend" "watchTower Backend"
    wait_for_pods "app=watchtower-frontend" 2 "watchTower Frontend"
    wait_for_service "watchtower-frontend" "watchTower Frontend"

    log_info "watchTower Backend + Frontend 就绪 ✓"
}

# ================================================================
# Step 8: 部署 Ingress 路由
# ================================================================
deploy_ingress() {
    log_step "========== 部署 Ingress 路由 =========="

    kubectl apply -f "$DEPLOY_DIR/09-ingress.yml"
    log_info "Ingress 路由已应用 ✓"
}

# ================================================================
# Step 9: 部署 HPA（依赖 metrics-server）
# ================================================================
deploy_hpa() {
    log_step "========== 部署 Metrics Server + HPA =========="

    kubectl apply -f "$DEPLOY_DIR/10-metrics-server.yml"

    # 等待 metrics-server 就绪
    count=0
    while true; do
        ready=$(kubectl get deployment -n kube-system metrics-server \
            -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        echo "  [$count/60s] metrics-server readyReplicas: $ready"
        if [[ "$ready" == "1" ]]; then
            log_info "metrics-server 就绪 ✓"
            break
        fi
        if [[ $count -ge 60 ]]; then
            log_warn "metrics-server 部署超时，继续部署 HPA..."
            break
        fi
        sleep 5
        count=$((count + 5))
    done

    kubectl apply -f "$DEPLOY_DIR/08-hpa.yml"
    log_info "HPA 已部署 ✓"
}

# ================================================================
# 状态查看
# ================================================================
status() {
    echo ""
    echo "=============================================="
    echo "  watchTower 集群状态"
    echo "=============================================="
    echo ""

    echo "--- Pods ---"
    kubectl get pods -n "$NAMESPACE" -o wide
    echo ""

    echo "--- Services ---"
    kubectl get svc -n "$NAMESPACE"
    echo ""

    echo "--- Deployments ---"
    kubectl get deployment -n "$NAMESPACE"
    echo ""

    echo "--- HPA ---"
    kubectl get hpa -n "$NAMESPACE" || echo "  无 HPA"
    echo ""

    echo "--- Ingress ---"
    kubectl get ingress -n "$NAMESPACE"
    echo ""

    echo "--- StatefulSets ---"
    kubectl get statefulset -n "$NAMESPACE"
    echo ""

    echo "--- PVC ---"
    kubectl get pvc -n "$NAMESPACE"
    echo ""

    echo "=============================================="
    echo "  访问地址（Kind 环境）"
    echo "=============================================="
    echo "  前端页面: http://localhost/"
    echo "  Backend API: http://localhost/api/chat"
    echo "  Milvus Attu: http://localhost:8000  (需单独部署 Attu)"
    echo "=============================================="
}

# ================================================================
# 清理
# ================================================================
cleanup() {
    echo ""
    echo "=============================================="
    echo "  清理 watchTower 部署"
    echo "=============================================="
    read -p "  确认删除 watchtower namespace 下的所有资源？(y/N): " confirm
    if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
        echo "  取消清理"
        return
    fi

    kubectl delete -f "$DEPLOY_DIR/09-ingress.yml" --ignore-not-found
    kubectl delete -f "$DEPLOY_DIR/08-hpa.yml" --ignore-not-found
    kubectl delete -f "$DEPLOY_DIR/10-metrics-server.yml" --ignore-not-found
    kubectl delete -f "$DEPLOY_DIR/07-watchtower.yml" --ignore-not-found
    kubectl delete -f "$DEPLOY_DIR/06-mock-prometheus.yml" --ignore-not-found
    kubectl delete -f "$DEPLOY_DIR/05-statefulset-milvus.yml" --ignore-not-found
    kubectl delete -f "$DEPLOY_DIR/04-statefulset-minio.yml" --ignore-not-found
    kubectl delete -f "$DEPLOY_DIR/03-statefulset-etcd.yml" --ignore-not-found
    kubectl delete -f "$DEPLOY_DIR/02-namespace-configmap.yml" --ignore-not-found
    kubectl delete -f "$DEPLOY_DIR/01-ingress-controller.yml" --ignore-not-found

    echo ""
    echo "  所有 watchtower 资源已删除"
    echo "  如需删除 Kind 集群: kind delete cluster --name $KIND_CLUSTER_NAME"
}

# ================================================================
# 完整部署流程
# ================================================================
deploy() {
    log_step "=============================================="
    log_step "  watchTower Kubernetes 部署"
    log_step "  Cluster: $KIND_CLUSTER_NAME"
    log_step "  Namespace: $NAMESPACE"
    log_step "=============================================="
    echo ""

    check_dependencies
    create_cluster
    label_nodes
    build_and_load_images
    deploy_ingress_controller
    deploy_infrastructure
    deploy_mock_prometheus
    deploy_watchtower
    deploy_ingress
    deploy_hpa

    echo ""
    log_step "=============================================="
    log_step "  部署完成！"
    log_step "=============================================="
    echo ""
    status
}

# ================================================================
# 主流程
# ================================================================
case "${1:-deploy}" in
    deploy)
        deploy
        ;;
    status)
        status
        ;;
    cleanup)
        cleanup
        ;;
    infra)
        deploy_infrastructure
        ;;
    *)
        echo ""
        echo "用法: $0 {deploy|status|cleanup|infra}"
        echo ""
        echo "  deploy  - 完整部署流程（默认）"
        echo "  status  - 查看集群状态"
        echo "  cleanup - 删除所有 watchtower 资源"
        echo "  infra  - 仅部署基础设施（etcd/minio/milvus）"
        echo ""
        echo "环境变量:"
        echo "  KIND_CLUSTER_NAME   Kind 集群名称（默认: watchtower）"
        echo "  NAMESPACE           K8s 命名空间（默认: watchtower）"
        echo ""
        echo "注意事项:"
        echo "  1. 首次部署前请修改 deploy/02-namespace-configmap.yml 中的 API Key"
        echo "  2. HPA 需要 metrics-server 支持，首次部署可能需要等待 metrics-server 就绪"
        echo "  3. Kind 环境访问: http://localhost/（需确保 80 端口未被占用）"
        echo ""
        exit 1
        ;;
esac
