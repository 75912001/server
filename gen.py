import os
import shutil
import subprocess
import sys


def run_cmd(cmd, cwd=None):
    print(f"Executing: {cmd}")
    res = subprocess.run(cmd, shell=True, capture_output=True, text=True, cwd=cwd)
    if res.returncode != 0:
        print(f"Error: {res.stderr}")
        sys.exit(res.returncode)
    return res.stdout


def check_tool(name):
    if shutil.which(name) is None:
        print(f"Error: '{name}' not found in PATH. Please install it first.")
        sys.exit(1)


def check_dir(path, desc):
    if not os.path.isdir(path):
        print(f"Error: {desc} not found: {path}")
        sys.exit(1)


def check_file(path, desc):
    if not os.path.isfile(path):
        print(f"Error: {desc} not found: {path}")
        sys.exit(1)


def main():
    # 当前 server 仓库根目录
    server_dir = os.path.dirname(os.path.abspath(__file__))
    # xlib 仓库与 server 同级
    xlib_dir = os.path.join(os.path.dirname(server_dir), "xlib")

    # --- 前置校验 ---
    check_tool("go")
    check_tool("protoc")
    check_dir(xlib_dir,                                              "xlib 仓库目录")
    check_dir(os.path.join(xlib_dir, "grpc", "proto", "gen"),       "xlib/grpc/proto/gen (插件源码)")
    check_dir(os.path.join(xlib_dir, "grpc", "proto"),              "xlib/grpc/proto (options.proto)")
    check_dir(os.path.join(xlib_dir, "thirdparty"),                 "xlib/thirdparty")
    check_file(os.path.join(server_dir, "proto", "online.grpc.proto"), "proto/online.grpc.proto")

    plugin_exe = os.path.join(xlib_dir, "protoc-gen-go-grpc-x.exe")

    # 1. 在 xlib 仓库中编译自研 protoc 插件
    run_cmd("go build -o protoc-gen-go-grpc-x.exe ./grpc/proto/gen/", cwd=xlib_dir)
    check_file(plugin_exe, "编译后的 protoc-gen-go-grpc-x.exe")

    # 2. 调用 protoc 生成全套 Stub 代码
    cmd = (
        f"protoc"
        f" --proto_path={os.path.join(server_dir, 'proto')}"
        f" --proto_path={os.path.join(xlib_dir, 'grpc', 'proto')}"
        f" --proto_path={os.path.join(xlib_dir, 'thirdparty')}"
        f" --plugin=protoc-gen-go-grpc-x={plugin_exe}"
        f" --go_out={xlib_dir} --go-grpc_out={xlib_dir} --go-grpc-x_out={xlib_dir}"
        f" --go_opt=Moptions.proto=github.com/75912001/xlib/grpc/proto,Monline.grpc.proto=github.com/75912001/xlib/grpc/template/proto/pb"
        f" --go-grpc_opt=Moptions.proto=github.com/75912001/xlib/grpc/proto,Monline.grpc.proto=github.com/75912001/xlib/grpc/template/proto/pb"
        f" --go-grpc-x_opt=Monline.grpc.proto=github.com/75912001/xlib/grpc/template/proto/pb"
        f" online.grpc.proto"
    )
    run_cmd(cmd, cwd=server_dir)

    # 3. 将生成文件移至 server 仓库的 proto/gen 目录
    gen_dst = os.path.join(server_dir, "proto", "pb")
    os.makedirs(gen_dst, exist_ok=True)

    gen_src = os.path.join(xlib_dir, "github.com", "75912001", "xlib", "grpc", "template", "proto", "pb")
    if not os.path.isdir(gen_src):
        print(f"Error: protoc 未生成预期输出目录: {gen_src}")
        sys.exit(1)

    for f in os.listdir(gen_src):
        shutil.move(os.path.join(gen_src, f), os.path.join(gen_dst, f))
    shutil.rmtree(os.path.join(xlib_dir, "github.com"))

    # 4. 清理编译产物
    if os.path.exists(plugin_exe):
        os.remove(plugin_exe)

    print("Done. 文件已输出至 proto/gen/")


if __name__ == "__main__":
    main()
