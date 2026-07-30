#!/usr/bin/env bash
#
# 功能:
#   一键重建本地 Docker 部署的 gateway/cache/online/login 四个 .1 服务实例.
#
# 用法:
#   在 server 仓库根目录使用 Git Bash 执行:
#     ./deploy.sh
#
# 执行流程:
#   1. 检查 python/docker 和 Docker daemon.
#   2. 执行 python gen.py 生成 protobuf 代码.
#   3. 按 cache -> online -> gateway -> login 顺序处理各服务:
#      清理 deploy/<service>/log, 停止 server.<service>.1,
#      删除 server.<service>.1, 删除 server.<service>:dev,
#      重新构建 server.<service>:dev, 启动 server.<service>.1.
#
# 影响范围:
#   会清理四个服务的部署日志, 停止并删除四个 .1 容器, 删除并重建本地 dev 镜像.
#   不处理 .2 实例, 不处理 Redis/etcd, 不拉取 GHCR 镜像.
set -u

COLOR_GREEN='\033[92m'
COLOR_RED='\033[91m'
COLOR_RESET='\033[0m'

print_error() {
    printf "${COLOR_RED}%s${COLOR_RESET}\n" "$1" >&2
}

print_success() {
    printf "${COLOR_GREEN}%s${COLOR_RESET}\n" "$1"
}

check_tool() {
    local name="$1"

    if ! command -v "$name" >/dev/null 2>&1; then
        print_error "check required tool failed: $name not found."
        exit 1
    fi
}

run_required() {
    local desc="$1"
    shift

    if ! "$@"; then
        print_error "${desc} failed."
        exit 1
    fi

    print_success "${desc} successfully."
}

service_ports() {
    local service="$1"

    case "$service" in
        cache)
            printf '%s\n' "-p" "20301:20301"
            ;;
        online)
            printf '%s\n' "-p" "20201:20201"
            ;;
        gateway)
            printf '%s\n' "-p" "10101:10101" "-p" "20101:20101"
            ;;
        login)
            printf '%s\n' "-p" "30401:30401"
            ;;
        *)
            print_error "get service ports failed: unsupported service $service."
            exit 1
            ;;
    esac
}

clean_service_log() {
    local service="$1"
    local log_dir="$SERVER_DIR/deploy/$service/log"

    run_required "prepare $service log directory" mkdir -p "$log_dir"

    # 只清理当前服务部署日志目录, 避免误删 deploy 之外的文件.
    shopt -s nullglob
    local log_files=("$log_dir"/*)
    shopt -u nullglob

    if ((${#log_files[@]} == 0)); then
        print_success "clean $service log skipped: no log files."
        return
    fi

    run_required "clean $service log" rm -rf -- "${log_files[@]}"
}

stop_service_container() {
    local service="$1"
    local container="server.$service.1"

    if ! docker container inspect "$container" >/dev/null 2>&1; then
        print_success "stop $container skipped: container not found."
        return
    fi

    run_required "stop $container" docker stop "$container"
}

remove_service_container() {
    local service="$1"
    local container="server.$service.1"

    if ! docker container inspect "$container" >/dev/null 2>&1; then
        print_success "remove $container skipped: container not found."
        return
    fi

    run_required "remove $container" docker rm "$container"
}

remove_service_image() {
    local service="$1"
    local image="server.$service:dev"

    if ! docker image inspect "$image" >/dev/null 2>&1; then
        print_success "remove $image skipped: image not found."
        return
    fi

    run_required "remove $image" docker rmi "$image"
}

build_service_image() {
    local service="$1"
    local image="server.$service:dev"
    local dockerfile="deploy/$service/Dockerfile"

    run_required "build $image" docker build -f "$dockerfile" -t "$image" .
}

start_service_container() {
    local service="$1"
    local container="server.$service.1"
    local image="server.$service:dev"
    local config_file="$SERVER_DIR/deploy/$service/$service.1.yaml"
    local log_dir="$SERVER_DIR/deploy/$service/log"
    local project_root_win
    local ports=()

    if [[ ! -f "$config_file" ]]; then
        print_error "start $container failed: config file not found: $config_file."
        exit 1
    fi

    project_root_win="$(pwd -W)"
    mapfile -t ports < <(service_ports "$service")

    if ! MSYS_NO_PATHCONV=1 docker run -d --name "$container" \
        "${ports[@]}" \
        -v "$project_root_win/deploy/$service/$service.1.yaml:/app/config/$service.yaml" \
        -v "$project_root_win/deploy/$service/log:/app/log" \
        "$image"; then
        print_error "start $container failed."
        exit 1
    fi

    print_success "start $container successfully."
}

deploy_service() {
    local service="$1"

    printf '\n==> deploy %s.1\n' "$service"
    clean_service_log "$service"
    stop_service_container "$service"
    remove_service_container "$service"
    remove_service_image "$service"
    build_service_image "$service"
    start_service_container "$service"
}

main() {
    # deploy.sh 位于 server 仓库根目录, 直接以脚本位置作为部署根目录, 避免依赖 git safe.directory.
    SERVER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    cd "$SERVER_DIR" || exit 1

    check_tool python
    check_tool docker

    if ! docker version >/dev/null 2>&1; then
        print_error "check Docker daemon failed: please make sure Docker is running."
        exit 1
    fi
    print_success "check Docker daemon successfully."

    run_required "generate protobuf code" python 1.gen.py

    # 先启动依赖服务, 再启动入口服务.
    local services=(cache online gateway login)
    local service
    for service in "${services[@]}"; do
        deploy_service "$service"
    done

    printf '\n'
    run_required "show deployed containers" docker ps --filter "name=server." --format "table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"
}

main "$@"
