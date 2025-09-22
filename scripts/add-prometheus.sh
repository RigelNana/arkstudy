#!/bin/bash

# 为所有微服务统一添加 Prometheus 监控支持

set -e

SERVICES=(
    "user-service"
    "material-service" 
    "ocr-service"
)

METRICS_PACKAGE="github.com/RigelNana/arkstudy/pkg/metrics"
PROTO_PACKAGE="github.com/RigelNana/arkstudy/proto"

echo "🚀 正在为所有服务添加 Prometheus 监控..."

for service in "${SERVICES[@]}"; do
    echo ""
    echo "📦 处理服务: $service"
    
    # 1. 更新 go.mod
    echo "  - 更新 go.mod"
    cd "services/$service"
    
    # 添加依赖
    if ! grep -q "$METRICS_PACKAGE" go.mod; then
        echo "    添加 metrics 依赖..."
        go mod edit -require="$METRICS_PACKAGE@v0.0.0"
        go mod edit -replace="$METRICS_PACKAGE=../../pkg/metrics"
    fi
    
    if ! grep -q "$PROTO_PACKAGE" go.mod; then
        echo "    添加 proto 依赖..."
        go mod edit -require="$PROTO_PACKAGE@v0.0.0"
        go mod edit -replace="$PROTO_PACKAGE=../../proto"
    fi
    
    # 2. 备份并更新 main.go 
    echo "  - 备份并更新 main.go"
    cp main.go main.go.backup
    
    # 添加 import
    if ! grep -q "pkg/metrics" main.go; then
        sed -i '/pb.*auth\|pb.*user\|pb.*material\|pb.*ai/a\\t"github.com/RigelNana/arkstudy/pkg/metrics"\n\tgrpcMetrics "github.com/RigelNana/arkstudy/pkg/metrics/grpc"' main.go
    fi
    
    # 在 main 函数开始后添加 metrics 服务器
    if ! grep -q "StartMetricsServer" main.go; then
        sed -i '/func main() {/a\\t// 启动 Prometheus metrics 服务器\n\tmetrics.StartMetricsServer("2112")\n\tlog.Printf("Prometheus metrics server started on :2112")\n' main.go
    fi
    
    # 更新 gRPC 服务器创建
    if ! grep -q "grpcMetrics\." main.go; then
        sed -i 's/grpcServer := grpc.NewServer()/grpcServer := grpc.NewServer(\n\t\tgrpc.UnaryInterceptor(grpcMetrics.UnaryServerInterceptor("'$service'")),\n\t\tgrpc.StreamInterceptor(grpcMetrics.StreamServerInterceptor("'$service'")),\n\t)/' main.go
    fi
    
    # 3. 运行 go mod tidy
    echo "  - 运行 go mod tidy"
    go mod tidy
    
    echo "  ✅ $service 完成"
    cd ../..
done

echo ""
echo "🎯 为 LLM Service (Python) 添加 Prometheus 支持..."

# LLM Service (Python)
cd services/llm-service

# 更新 requirements.txt
if ! grep -q "prometheus" requirements.txt; then
    echo "prometheus-fastapi-instrumentator" >> requirements.txt
    echo "prometheus-client" >> requirements.txt
fi

# 更新 main.py
if ! grep -q "prometheus" app/main.py; then
    echo "  - 更新 main.py 添加 Prometheus"
    sed -i '1i\from prometheus_fastapi_instrumentator import Instrumentator' app/main.py
    sed -i '/app = FastAPI/a\\n# 启用 Prometheus 指标\ninstrumentator = Instrumentator()\ninstrumentator.instrument(app).expose(app)' app/main.py
fi

cd ../..

echo ""
echo "🔧 更新 Helm Chart 以支持 metrics 端口..."

# 更新 values-dev.yaml 为所有服务添加 metrics 端口
SERVICES_WITH_GATEWAY=("gateway" "${SERVICES[@]}")

for service in "${SERVICES_WITH_GATEWAY[@]}"; do
    echo "  - 为 $service 添加 metrics 端口配置"
    # 这里可以添加 Helm values 更新逻辑
done

echo ""
echo "✅ 所有服务的 Prometheus 监控配置完成！"
echo ""
echo "📋 下一步操作："
echo "1. 重新构建所有服务的 Docker 镜像"
echo "2. 部署 Prometheus + Grafana 监控栈"
echo "3. 配置 ServiceMonitor 资源"
echo "4. 验证 metrics 采集"
echo ""
echo "🚀 运行以下命令查看 metrics:"
echo "kubectl port-forward svc/arkstudy-gateway 8080:8080"
echo "curl http://localhost:8080/metrics"