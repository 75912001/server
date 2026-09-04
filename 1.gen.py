import os
import re
import shutil
import subprocess
import sys

# ─── 脚本功能说明 ───────────────────────────────────────────────────────────────
# 
# gen.py: Protocol Buffers 代码生成脚本
#
# 功能概述:
#   从 server/proto 目录中的 .proto 文件生成 Go Protocol Buffers 代码和 gRPC 服务桩。
#   这是一个两阶段生成器：
#   1. 首先解析源 proto 文件中的 CMD 注释，校验唯一性，生成统一的 cmd.proto 枚举
#   2. 然后执行 protoc 编译所有 proto 文件，输出到 proto/pb/ 目录
#
# 执行前置条件:
#   - Python 3 环境
#   - Go 工具链（go list 用于查询 xlib 依赖位置）
#   - protoc 编译器（Protocol Buffers 3+）
#   - xlib 模块可用（github.com/75912001/xlib）
#   - protoc-gen-go-grpc-x 插件（位于 xlib/grpc/proto/bin/）
#
# 生成输出:
#   - proto/cmd.proto          生成的 CMD 枚举定义文件（自动）
#   - proto/pb/*.pb.go         Protocol Buffers 消息代码（所有 proto）
#   - proto/pb/*_grpc.pb.go    gRPC 服务客户端和服务器代码（*.grpc.proto）
#   - proto/pb/*_grpc.x.pb.go  gRPC-X 扩展代码（*.grpc.proto）
#
# 调用方式:
#   python 1.gen.py

XLIB_MODULE = "github.com/75912001/xlib"

# ANSI 颜色代码用于终端输出
COLOR_GREEN = "\033[32m"
COLOR_RED = "\033[31m"
COLOR_RESET = "\033[0m"


# ─── 工具函数 ─────────────────────────────────────────────────────────────────

def print_ok(message):
    """打印成功信息（绿色）"""
    print(f"{COLOR_GREEN}{message}{COLOR_RESET}")


def print_error(message):
    """打印错误信息（红色）到标准错误流"""
    print(f"{COLOR_RED}{message}{COLOR_RESET}", file=sys.stderr)


def run_cmd(cmd, cwd=None):
    """
    执行系统命令，若失败则打印错误并退出
    参数:
      cmd: 命令列表（如 ["protoc", "--version"]）
      cwd: 工作目录
    返回: 命令的标准输出字符串
    """
    print(f"Executing: {subprocess.list2cmdline(cmd)}")
    res = subprocess.run(cmd, capture_output=True, text=True, cwd=cwd)
    if res.returncode != 0:
        print_error(f"Error: {res.stderr}")
        sys.exit(res.returncode)
    return res.stdout


def check_tool(name):
    """检查系统 PATH 中是否存在指定工具，不存在则退出"""
    if shutil.which(name) is None:
        print_error(f"Error: '{name}' not found in PATH. Please install it first.")
        sys.exit(1)


def check_dir(path, desc):
    """检查目录是否存在，不存在则打印错误信息并退出"""
    if not os.path.isdir(path):
        print_error(f"Error: {desc} not found: {path}")
        sys.exit(1)


def check_file(path, desc):
    """检查文件是否存在，不存在则打印错误信息并退出"""
    if not os.path.isfile(path):
        print_error(f"Error: {desc} not found: {path}")
        sys.exit(1)


def get_go_module_dir(module, cwd):
    """
    查询 Go 模块的本地磁盘位置
    参数:
      module: Go 模块名（如 "github.com/75912001/xlib"）
      cwd: 查询上下文目录
    返回: 模块的绝对路径
    """
    out = run_cmd(["go", "list", "-m", "-f", "{{.Dir}}", module], cwd=cwd).strip()
    if not out:
        print_error(f"Error: go module dir is empty: {module}")
        sys.exit(1)
    return out


def get_grpc_x_plugin_path(xlib_dir):
    """
    根据操作系统返回 protoc-gen-go-grpc-x 插件的路径
    支持平台: Windows (protoc-gen-go-grpc-x)、macOS (protoc-gen-go-grpc-x-mac)、Linux (protoc-gen-go-grpc-x-linux)
    """
    if sys.platform.startswith("win"):
        name = "protoc-gen-go-grpc-x"
    elif sys.platform == "darwin":
        name = "protoc-gen-go-grpc-x-mac"
    else:
        name = "protoc-gen-go-grpc-x-linux"

    return os.path.join(xlib_dir, "grpc", "proto", "bin", name)


def discover_protos(proto_dir):
    """
    扫描 proto 目录，返回两个列表：
      - all_protos  : 所有 .proto 文件（排除 options.proto，它只作 import 用）
      - grpc_protos : 仅 *.grpc.proto 文件（需要 gRPC 插件生成服务桩代码）
    """
    all_protos, grpc_protos = [], []
    for fname in sorted(os.listdir(proto_dir)):
        if not fname.endswith(".proto") or fname == "options.proto":
            continue
        all_protos.append(fname)
        if ".grpc." in fname:
            grpc_protos.append(fname)
    return all_protos, grpc_protos


# ─── CMD proto 生成 ───────────────────────────────────────────────────────────
# 从源 proto 文件中解析 CMD 注释和消息定义，生成统一的 cmd.proto 枚举文件

# 匹配 CMD 注释格式: //0xHEX#direction#description
_RE_CMD_COMMENT = re.compile(r'^\s*//\s*(0x[0-9A-Fa-f]+)#([^#]+)#(.+)')
# 匹配 proto 消息定义: message MessageName
_RE_MESSAGE     = re.compile(r'^\s*message\s+(\w+)')
# 匹配 proto package: package xyz
_RE_PACKAGE     = re.compile(r'^\s*package\s+(\w+)')
# 匹配 proto go_package option: option go_package = "path/to/package"
_RE_GO_PACKAGE  = re.compile(r'^\s*option\s+go_package\s*=\s*"([^"]+)"')


def _parse_source_proto(path):
    """
    解析源 proto 文件，返回：
      - package: proto package 名称
      - go_package: Go 包路径
      - entries: 列表，每项为 (hex_val, direction, desc, msg_name)
        其中 hex_val 为十六进制 CMD 值、direction 为消息方向、desc 为描述、msg_name 为消息名
    扫描逻辑：找到 //0xHEX#...# 注释后，查找其后首个 message 定义，配对为一条 entry
    """
    package, go_package = "", ""
    entries = []
    pending = None

    with open(path, encoding="utf-8") as f:
        for line in f:
            m = _RE_PACKAGE.match(line)
            if m:
                package = m.group(1)
                continue
            m = _RE_GO_PACKAGE.match(line)
            if m:
                go_package = m.group(1)
                continue
            m = _RE_CMD_COMMENT.match(line)
            if m:
                pending = (m.group(1), m.group(2), m.group(3).strip())
                continue
            if pending:
                m = _RE_MESSAGE.match(line)
                if m:
                    entries.append((*pending, m.group(1)))
                pending = None

    return package, go_package, entries


def _is_cmd_proto_filename(fname):
    return fname == "cmd.proto" or fname.endswith(".cmd.proto")


def _remove_generated_file(path, expected_header):
    if not os.path.isfile(path):
        return
    with open(path, encoding="utf-8") as f:
        first_line = f.readline().strip()
    if first_line != expected_header:
        print_error(f"Error: refuse to remove non-generated CMD file: {path}")
        sys.exit(1)
    os.remove(path)


def _clean_legacy_cmd_outputs(proto_dir):
    """
    清理旧的已生成的 CMD 相关文件，防止残留的过期代码。
    清理范围：
      - proto 目录中带生成标记的 cmd.proto 和 *.cmd.proto 文件
      - proto/pb 目录中的 cmd.pb.go 和 *.cmd.pb.go 文件
    只会删除自动生成标记的文件（首行为 "// Code generated by gen.py. DO NOT EDIT."），保护手写代码。
    """
    for fname in sorted(os.listdir(proto_dir)):
        if _is_cmd_proto_filename(fname):
            _remove_generated_file(
                os.path.join(proto_dir, fname),
                "// Code generated by gen.py. DO NOT EDIT.",
            )

    pb_dir = os.path.join(proto_dir, "pb")
    if not os.path.isdir(pb_dir):
        return
    for fname in sorted(os.listdir(pb_dir)):
        if fname == "cmd.pb.go" or fname.endswith(".cmd.pb.go"):
            _remove_generated_file(
                os.path.join(pb_dir, fname),
                "// Code generated by gen.py. DO NOT EDIT.",
            )


def gen_cmd_proto(proto_dir):
    """
    生成统一的 cmd.proto 文件，步骤：
    1. 清理旧的已生成的 CMD 相关文件（带生成标记的 cmd.proto 和 .pb.go 文件）
    2. 扫描全部源 proto 文件（排除 options.proto 和 cmd.proto），解析 CMD 注释和消息
    3. 校验 package/go_package 一致性、CMD 值全局唯一、消息名全局唯一
    4. 生成 cmd.proto，包含 MsgID 枚举，十进制数值，十六进制注释
    """
    _clean_legacy_cmd_outputs(proto_dir)

    package = ""
    go_package = ""
    all_entries = []
    for fname in sorted(os.listdir(proto_dir)):
        if not fname.endswith(".proto") or fname == "options.proto" or _is_cmd_proto_filename(fname):
            continue

        source_package, source_go_package, entries = _parse_source_proto(os.path.join(proto_dir, fname))
        if not entries:
            continue
        if not source_package or not source_go_package:
            print_error(f"Error: CMD source proto package is incomplete: {fname}")
            sys.exit(1)
        if not package:
            package = source_package
            go_package = source_go_package
        elif package != source_package or go_package != source_go_package:
            print_error(
                "Error: all CMD source protos must use the same package and go_package: "
                f"{fname} package={source_package} go_package={source_go_package}"
            )
            sys.exit(1)
        all_entries.extend((*entry, fname) for entry in entries)

    if not all_entries:
        print_error("Error: no CMD annotations found in proto sources")
        sys.exit(1)

    seen_values = {}
    seen_messages = {}
    for hex_val, _, _, msg_name, source_name in all_entries:
        decimal_val = int(hex_val, 16)
        if decimal_val == 0:
            print_error(f"Error: CMD value 0 is reserved: {source_name}:{msg_name}")
            sys.exit(1)
        if decimal_val in seen_values:
            print_error(
                f"Error: duplicated CMD value {hex_val}: "
                f"{seen_values[decimal_val]} and {source_name}:{msg_name}"
            )
            sys.exit(1)
        if msg_name in seen_messages:
            print_error(
                f"Error: duplicated CMD message {msg_name}: "
                f"{seen_messages[msg_name]} and {source_name}"
            )
            sys.exit(1)
        seen_values[decimal_val] = f"{source_name}:{msg_name}"
        seen_messages[msg_name] = source_name

    out_path = os.path.join(proto_dir, "cmd.proto")
    with open(out_path, "w", encoding="utf-8") as f:
        f.write("// Code generated by gen.py. DO NOT EDIT.\n\n")
        f.write('syntax = "proto3";\n')
        f.write(f"package {package};\n")
        f.write(f'option go_package = "{go_package}";\n')
        f.write("\nenum MsgID {\n")
        f.write("  MsgIDUnknown_CMD = 0; // proto3 首值必须为 0\n")
        for hex_val, direction, desc, msg_name, _ in all_entries:
            comment = f"//{hex_val}#{direction}#{desc}"
            f.write(f"  {msg_name}_CMD = {int(hex_val, 16)}; {comment}\n")
        f.write("}\n")

    print_ok(f"生成 cmd.proto  ({len(all_entries)} 条 CMD)")


# ─── 主流程 ───────────────────────────────────────────────────────────────────

def main():
    # 当前 server 仓库根目录
    server_dir = os.path.dirname(os.path.abspath(__file__))
    # xlib 仓库与 server 同级
    xlib_dir = get_go_module_dir(XLIB_MODULE, server_dir)

    # --- 前置校验 ---
    check_tool("go")
    check_tool("protoc")
    check_dir(xlib_dir,                                              "xlib 仓库目录")
    check_dir(os.path.join(xlib_dir, "grpc", "proto"),              "xlib/grpc/proto (options.proto)")
    check_file(os.path.join(xlib_dir, "grpc", "proto", "options.proto"), "xlib/grpc/proto/options.proto")
    check_dir(os.path.join(xlib_dir, "thirdparty"),                 "xlib/thirdparty")

    proto_dir = os.path.join(server_dir, "proto")
    check_dir(proto_dir, "proto 目录")

    plugin_exe = get_grpc_x_plugin_path(xlib_dir)
    check_file(plugin_exe, "protoc-gen-go-grpc-x plugin")

    # 2. 根据全部源 proto 的 //0xHEX#... 注释生成统一 cmd.proto 枚举文件
    gen_cmd_proto(proto_dir)

    # 3. 扫描 proto 文件, 分组(cmd.proto 已写入 proto_dir, 会被发现)
    all_protos, grpc_protos = discover_protos(proto_dir)
    if not all_protos:
        print_error("Error: proto 目录中没有找到任何 .proto 文件")
        sys.exit(1)
    print(f"发现 proto 文件: {all_protos}")
    print(f"gRPC proto 文件: {grpc_protos}")

    out_dir = server_dir
    proto_path_args = [
        f"--proto_path={proto_dir}",
        f"--proto_path={os.path.join(xlib_dir, 'grpc', 'proto')}",
        f"--proto_path={os.path.join(xlib_dir, 'thirdparty')}",
    ]
    m_options = "Moptions.proto=github.com/75912001/xlib/grpc/proto"

    # 4a. 所有 proto → --go_out（普通消息、枚举）
    cmd_go = [
        "protoc",
        *proto_path_args,
        f"--go_out={out_dir}",
        f"--go_opt=module=server,{m_options}",
        *all_protos,
    ]
    run_cmd(cmd_go, cwd=server_dir)
    print_ok(f"[go_out] 生成完毕: {all_protos}")

    # 4b. *.grpc.proto → --go-grpc_out + --go-grpc-x_out（服务桩代码）
    if grpc_protos:
        cmd_grpc = [
            "protoc",
            *proto_path_args,
            f"--plugin=protoc-gen-go-grpc-x={plugin_exe}",
            f"--go-grpc_out={out_dir}",
            f"--go-grpc_opt=module=server,{m_options}",
            f"--go-grpc-x_out={out_dir}",
            "--go-grpc-x_opt=module=server",
            *grpc_protos,
        ]
        run_cmd(cmd_grpc, cwd=server_dir)
        print_ok(f"[go-grpc_out / go-grpc-x_out] 生成完毕: {grpc_protos}")

    print_ok("Done. 文件已输出至 proto/pb/")


if __name__ == "__main__":
    main()
