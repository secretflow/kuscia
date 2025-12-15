#!/bin/bash

# 创建大量测试 Job 的脚本
# 用途: 为 GC 功能测试创建 10000 个已完成的 KusciaJob

set -e

# ============ 配置参数 ============
TOTAL_JOBS=10000
BATCH_SIZE=100
NAMESPACE="cross-domain"
COMPLETION_DAYS_AGO=35  # 完成时间为 35 天前 (超过默认 30 天保留期)

# ============ 辅助函数 ============
log_info() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] INFO: $*"
}

log_error() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $*" >&2
}

# 生成 RFC3339 格式的时间戳 (N 天前)
generate_completion_time() {
    local days_ago=$1
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        date -u -v-${days_ago}d +"%Y-%m-%dT%H:%M:%SZ"
    else
        # Linux
        date -u -d "${days_ago} days ago" +"%Y-%m-%dT%H:%M:%SZ"
    fi
}

# 创建单个 KusciaJob
create_kuscia_job() {
    local job_id=$1
    local completion_time=$2
    local job_name="test-gc-job-${job_id}"

    cat <<EOF | kubectl apply -f - > /dev/null 2>&1
apiVersion: kuscia.secretflow/v1alpha1
kind: KusciaJob
metadata:
  name: ${job_name}
  namespace: ${NAMESPACE}
  labels:
    test-purpose: gc-test
    batch: "$(($job_id / $BATCH_SIZE))"
spec:
  initiator: alice
  maxParallelism: 1
  tasks:
    - taskID: task-1
      alias: task-1
      appImage: secretflow/secretflow-lite-anolis8:latest
      parties:
        - domainID: alice
          role: default
status:
  phase: Succeeded
  completionTime: "${completion_time}"
  conditions:
    - type: Complete
      status: "True"
      lastTransitionTime: "${completion_time}"
  taskStatus:
    task-1:
      phase: Succeeded
EOF

    if [ $? -eq 0 ]; then
        return 0
    else
        log_error "Failed to create job ${job_name}"
        return 1
    fi
}

# ============ 主逻辑 ============
main() {
    log_info "Starting to create ${TOTAL_JOBS} test KusciaJobs"
    log_info "Namespace: ${NAMESPACE}"
    log_info "Batch size: ${BATCH_SIZE}"
    log_info "Completion time: ${COMPLETION_DAYS_AGO} days ago"

    # 检查命名空间是否存在
    if ! kubectl get namespace ${NAMESPACE} > /dev/null 2>&1; then
        log_error "Namespace ${NAMESPACE} does not exist"
        exit 1
    fi

    # 生成完成时间
    completion_time=$(generate_completion_time ${COMPLETION_DAYS_AGO})
    log_info "Completion time: ${completion_time}"

    # 创建 Jobs
    local created_count=0
    local failed_count=0
    local start_time=$(date +%s)

    for ((i=1; i<=TOTAL_JOBS; i++)); do
        if create_kuscia_job $i "$completion_time"; then
            ((created_count++))
        else
            ((failed_count++))
        fi

        # 每批次打印进度
        if [ $((i % BATCH_SIZE)) -eq 0 ]; then
            local current_time=$(date +%s)
            local elapsed=$((current_time - start_time))
            local rate=$((i / elapsed))
            local remaining=$((TOTAL_JOBS - i))
            local eta=$((remaining / rate))

            log_info "Progress: ${i}/${TOTAL_JOBS} (${created_count} created, ${failed_count} failed)"
            log_info "Rate: ${rate} jobs/sec, ETA: ${eta} seconds"

            # 避免 API Server 过载,批次间休眠
            sleep 2
        fi
    done

    local end_time=$(date +%s)
    local total_time=$((end_time - start_time))

    log_info "=========================================="
    log_info "Job creation completed!"
    log_info "Total jobs: ${TOTAL_JOBS}"
    log_info "Created: ${created_count}"
    log_info "Failed: ${failed_count}"
    log_info "Total time: ${total_time} seconds"
    log_info "Average rate: $((TOTAL_JOBS / total_time)) jobs/sec"
    log_info "=========================================="
}

# 执行
main "$@"
