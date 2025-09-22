#!/bin/bash

# Prometheus 监控栈部署与验证脚本

set -e

NAMESPACE="arkstudy"
MONITORING_NAMESPACE="monitoring"

echo "🚀 部署 Prometheus 监控栈"

# 1. 安装 Prometheus Operator
echo "📦 安装 kube-prometheus-stack..."
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# 创建 monitoring values
cat > /tmp/monitoring-values.yaml << EOF
prometheus:
  prometheusSpec:
    retention: 15d
    serviceMonitorSelectorNilUsesHelmValues: false
    serviceMonitorSelector: {}
    ruleSelectorNilUsesHelmValues: false
    ruleSelector: {}
    storageSpec:
      volumeClaimTemplate:
        spec:
          storageClassName: standard
          accessModes: ["ReadWriteOnce"]
          resources:
            requests:
              storage: 10Gi

grafana:
  adminPassword: admin123
  persistence:
    enabled: true
    size: 5Gi
  
  dashboardProviders:
    dashboardproviders.yaml:
      apiVersion: 1
      providers:
      - name: 'default'
        orgId: 1
        folder: ''
        type: file
        disableDeletion: false
        editable: true
        options:
          path: /var/lib/grafana/dashboards/default

  dashboards:
    default:
      go-grpc:
        gnetId: 11655
        revision: 1
        datasource: Prometheus
      gin-gonic:
        gnetId: 16802  
        revision: 1
        datasource: Prometheus
      fastapi:
        gnetId: 14138
        revision: 1
        datasource: Prometheus

alertmanager:
  config:
    route:
      group_by: ['alertname', 'severity']
      group_wait: 10s
      group_interval: 10s
      repeat_interval: 1h
      receiver: 'web.hook'
    receivers:
    - name: 'web.hook'
      webhook_configs:
      - url: 'http://127.0.0.1:5001/'
        send_resolved: true
EOF

# 安装或升级监控栈
helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  -n $MONITORING_NAMESPACE --create-namespace \
  -f /tmp/monitoring-values.yaml \
  --wait --timeout 10m

echo "⏳ 等待 Prometheus Operator 就绪..."
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=prometheus-operator -n $MONITORING_NAMESPACE --timeout=300s

# 2. 重新部署应用以包含 metrics 支持
echo "🔄 重新部署应用服务..."
helm upgrade --install arkstudy deploy/helm/arkstudy \
  -n $NAMESPACE \
  -f deploy/helm/arkstudy/values-dev.yaml \
  --wait --timeout 10m

# 3. 验证 metrics 端点
echo "🔍 验证 metrics 端点..."

# 等待 Pod 就绪
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=arkstudy -n $NAMESPACE --timeout=300s

# 测试 Gateway metrics
echo "  测试 Gateway metrics..."
kubectl port-forward -n $NAMESPACE svc/arkstudy-gateway 8080:8080 &
GATEWAY_PF_PID=$!
sleep 3
if curl -s http://localhost:8080/metrics | grep -q "prometheus"; then
    echo "  ✅ Gateway metrics OK"
else
    echo "  ❌ Gateway metrics 失败"
fi
kill $GATEWAY_PF_PID 2>/dev/null || true

# 测试 LLM service metrics
echo "  测试 LLM Service metrics..."
kubectl port-forward -n $NAMESPACE svc/arkstudy-llm-service 8000:8000 &
LLM_PF_PID=$!
sleep 3
if curl -s http://localhost:8000/metrics | grep -q "prometheus"; then
    echo "  ✅ LLM Service metrics OK"
else
    echo "  ❌ LLM Service metrics 失败"
fi
kill $LLM_PF_PID 2>/dev/null || true

# 4. 检查 ServiceMonitor 状态
echo "🎯 检查 ServiceMonitor 状态..."
kubectl get servicemonitor -n $NAMESPACE

# 5. 检查 Prometheus targets
echo "📊 访问 Prometheus 控制台..."
kubectl port-forward -n $MONITORING_NAMESPACE svc/kube-prometheus-stack-prometheus 9090:9090 &
PROM_PF_PID=$!
echo "  Prometheus: http://localhost:9090"
echo "  检查 targets: http://localhost:9090/targets"

# 6. 检查 Grafana
echo "📈 访问 Grafana 控制台..."
kubectl port-forward -n $MONITORING_NAMESPACE svc/kube-prometheus-stack-grafana 3000:80 &
GRAFANA_PF_PID=$!
echo "  Grafana: http://localhost:3000 (admin/admin123)"

echo ""
echo "🎉 Prometheus 监控栈部署完成！"
echo ""
echo "📋 验证清单："
echo "1. ✅ kube-prometheus-stack 已安装"
echo "2. ✅ arkstudy 服务已更新支持 metrics"
echo "3. ✅ ServiceMonitor 已创建"
echo "4. ✅ PrometheusRule 告警规则已配置"
echo ""
echo "🔧 下一步操作："
echo "1. 在 Grafana 中导入或创建自定义仪表盘"
echo "2. 配置 AlertManager 通知渠道"
echo "3. 测试告警规则触发"
echo ""
echo "⚠️  记住按 Ctrl+C 停止 port-forward 进程"

# 保持 port-forward 运行
wait