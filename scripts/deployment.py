#!/usr/bin/env python3
"""One-command application releases of the integrated main_temp snapshot.

The same standard-library-only module runs on the workstation and through SSH.
Remote secrets stay in remote process memory / root-only recovery files.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
from urllib.parse import urlsplit
import uuid


PUBLIC_KEYS = ("VITE_OIDC_AUTHORITY", "VITE_OIDC_CLIENT_ID", "VITE_OIDC_REDIRECT_URI",
               "VITE_OIDC_POST_LOGOUT_REDIRECT_URI")
MIGRATIONS = "backend/internal/infrastructure/gormdb/migrations"
GROUPS = {
    "backend": ("backend", "deploy/platform/Dockerfile.service", ".dockerignore"),
    "web": ("frontend",),
    "infrastructure": ("deploy/platform/compose.yaml", "deploy/platform/compose.execution.yaml",
                       "deploy/platform/compose.https.yaml", "deploy/platform/Caddyfile",
                       "deploy/sandbox", "deploy/runtimes", "deploy/platform/themes",
                       "scripts/build-identity-theme.py", "frontend/src/design-tokens.css"),
}
IGNORED = {"node_modules", "dist", "coverage", "__pycache__", ".git"}


class DeploymentError(RuntimeError):
    pass


def run(args, *, cwd=None, env=None, input=None, log=None):
    result = subprocess.run(args, cwd=cwd, env=env, input=input, text=True,
                            stdout=log or subprocess.PIPE, stderr=log or subprocess.PIPE)
    if result.returncode:
        # Remote output may contain interpolated configuration or credentials.
        raise DeploymentError(f"{Path(args[0]).name} 执行失败；" +
                              (f"详情保存在 {log.name}" if log else "请检查命令环境与连接"))
    return result.stdout or ""


def validate_profile(profile):
    if set(profile) != {"host", "root"}:
        raise DeploymentError("部署配置只接受 host 和 root")
    host, root = profile["host"], profile["root"]
    if not isinstance(host, str) or not re.fullmatch(r"[A-Za-z0-9_][A-Za-z0-9_.@:-]*", host):
        raise DeploymentError("服务器应为 SSH 别名或 user@host，不能包含命令或选项")
    if not isinstance(root, str) or not re.fullmatch(r"/[A-Za-z0-9_./-]+", root):
        raise DeploymentError("安装目录必须是绝对路径")
    if root == "/" or "//" in root or any(p in (".", "..") for p in root.split("/")):
        raise DeploymentError("安装目录不能是根目录或包含路径跳转")
    return {"host": host, "root": root.rstrip("/")}


def profile_path():
    base = Path(os.environ.get("XDG_CONFIG_HOME", str(Path.home() / ".config")))
    return base / "agent-workspace" / "deploy.json"


def save_profile(profile, path):
    profile = validate_profile(profile)
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, delete=False) as output:
        temporary = Path(output.name)
        os.chmod(temporary, 0o600)
        json.dump(profile, output, ensure_ascii=False)
        output.write("\n")
    os.replace(temporary, path)
    return profile


def load_profile(args):
    path = profile_path()
    if args.host:
        return validate_profile({"host": args.host, "root": args.root or "/srv/agent-workspace"})
    if args.root:
        raise DeploymentError("--root 需要同时提供 --host")
    if args.configure or not path.exists():
        if not sys.stdin.isatty():
            raise DeploymentError("首次运行：make deploy-setup；也可提供 --host 和 --root")
        profile = validate_profile({"host": input("服务器（SSH 别名或 user@host）：").strip(),
                                    "root": input("安装目录 [/srv/agent-workspace]：").strip()
                                    or "/srv/agent-workspace"})
        return save_profile(profile, path)
    return validate_profile(json.loads(path.read_text()))


def file_manifest(root):
    files = {}
    paths = []
    for directory, dirs, names in os.walk(root, followlinks=False):
        dirs[:] = sorted(d for d in dirs if d not in IGNORED)
        for name in dirs + names:
            path = Path(directory) / name
            if path.is_symlink():
                raise DeploymentError(f"发布目录包含符号链接：{path.relative_to(root)}")
        paths.extend(Path(directory) / name for name in names)
    for path in sorted(paths):
        relative = path.relative_to(root)
        if not path.is_file() or relative.name.endswith(".tsbuildinfo"):
            continue
        if relative.name.startswith(".env") and relative.name != ".env.example":
            raise DeploymentError(f"发布目录包含私有配置：{relative}")
        files[relative.as_posix()] = hashlib.sha256(path.read_bytes()).hexdigest()
    return files


def group_hashes(files):
    return {group: hashlib.sha256(json.dumps({name: digest for name, digest in files.items()
                       if any(name == prefix or name.startswith(prefix + "/") for prefix in prefixes)},
                       sort_keys=True).encode()).hexdigest() for group, prefixes in GROUPS.items()}


def make_plan(candidate, deployed):
    new, old = group_hashes(candidate), group_hashes(deployed)
    if new["infrastructure"] != old["infrastructure"]:
        raise DeploymentError("本次包含基础设施、主题或 Runtime 变更，需按运维说明发布；日常部署不改变这些配置")
    old_migrations = {p: h for p, h in deployed.items() if p.startswith(MIGRATIONS + "/")}
    if any(candidate.get(p) != h for p, h in old_migrations.items()):
        raise DeploymentError("已部署 Migration 不能修改或删除")
    return {"backend": new["backend"] != old["backend"], "web": new["web"] != old["web"],
            "migration": any(p.startswith(MIGRATIONS + "/") and p not in deployed for p in candidate)}


def validate_public(values):
    for key in PUBLIC_KEYS:
        if not values.get(key):
            raise DeploymentError(f"服务器缺少 {key}，请补齐远端配置")
        if key == "VITE_OIDC_CLIENT_ID":
            if not re.fullmatch(r"[A-Za-z0-9_.-]+", values[key]):
                raise DeploymentError("OIDC client ID 无效")
            continue
        parsed = urlsplit(values[key])
        if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment:
            raise DeploymentError(f"{key} 必须是无账号、query、fragment 的 HTTPS 地址")
    return values


def snapshot(repo, destination):
    run(["git", "fetch", "origin", "main_temp"], cwd=repo)
    revision = run(["git", "rev-parse", "origin/main_temp"], cwd=repo).strip()
    archive = destination.parent / "source.tar"
    run(["git", "archive", "--format=tar", "-o", str(archive), revision], cwd=repo)
    destination.mkdir()
    resolved_destination = destination.resolve()
    with tarfile.open(archive) as bundle:
        # A committed snapshot cannot upload ignored workstation credentials or .git.
        for member in bundle.getmembers():
            if not (member.isfile() or member.isdir()) or not (destination / member.name).resolve().is_relative_to(resolved_destination):
                raise DeploymentError("集成源码包含不安全的归档路径或链接")
        bundle.extractall(destination)
    return revision


def remote(profile, action, *, capture=True):
    payload = {**action, "root": profile["root"]}
    # Carry manifests on stdin: a single SSH argument can exceed Linux's 128 KiB limit.
    bootstrap = "import sys; payload=sys.stdin.readline(); sys.argv=['deployment','--remote',payload]; exec(compile(sys.stdin.read(),'<deployment>','exec'))"
    command = "python3 -c " + shlex.quote(bootstrap)
    source = json.dumps(payload) + "\n" + Path(__file__).read_text()
    if capture:
        result = subprocess.run(["ssh", "-o", "BatchMode=yes", profile["host"], command], input=source,
                                text=True, capture_output=True)
        if result.returncode:
            message = next((line for line in result.stderr.splitlines() if line.startswith("部署停止：")),
                           "SSH 连接失败，请检查服务器地址和 SSH Key")
            raise DeploymentError(message.removeprefix("部署停止："))
        return json.loads(result.stdout)
    result = subprocess.run(["ssh", "-o", "BatchMode=yes", profile["host"], command], input=source, text=True)
    if result.returncode:
        raise DeploymentError("服务器部署未完成；请按上方提示处理")


def gates(source, plan, public, log):
    run(["make", "deploy-test"], cwd=source, log=log)
    if plan["backend"]:
        for target in ("test", "build"):
            run(["make", target], cwd=source, log=log)
    if plan["web"]:
        run(["pnpm", "--dir", "frontend", "install", "--frozen-lockfile"], cwd=source, log=log)
        run(["pnpm", "--dir", "frontend", "test"], cwd=source, log=log)
        run(["make", "web-typecheck"], cwd=source, log=log)
        run(["make", "web-build"], cwd=source, env={**os.environ, **public}, log=log)


def require_build_space(source, plan):
    required = 4 * 1024**3 if plan["backend"] else 1024**3 if plan["web"] else 0
    if shutil.disk_usage(source).free < required:
        raise DeploymentError(f"发布工作站构建空间不足 {required // 1024**3} GiB，请释放过期构建缓存后重试；服务器尚未更新")


def local_main(args):
    profile = load_profile(args)
    if args.configure:
        save_profile(profile, profile_path())
        remote(profile, {"action": "inspect"})
        print(f"已保存 {profile['host']}:{profile['root']}。以后运行 make deploy。")
        return
    repo = Path(__file__).resolve().parent.parent
    print("读取 main_temp 和服务器配置…", flush=True)
    with tempfile.TemporaryDirectory(prefix="aw-deploy-") as temporary:
        source = Path(temporary) / "source"
        revision = snapshot(repo, source)
        manifest = file_manifest(source)
        state = remote(profile, {"action": "inspect"})
        plan = make_plan(manifest, state["files"])
        # A previous partial/manual release does not prove both running binaries match src.
        plan["backend"] = plan["backend"] or not state["backend_verified"]
        print(f"目标 {profile['host']} · main_temp {revision[:12]} · " +
              ("更新后端和前端" if plan["backend"] and plan["web"] else
               "更新后端" if plan["backend"] else "更新前端" if plan["web"] else "应用无改动，无需重启"), flush=True)
        if args.check:
            print("配置和发布范围检查通过。")
            return
        if plan["backend"] and state["busy"]:
            raise DeploymentError("还有任务运行或等待处理，稍后直接重试 make deploy")
        # Run immutable integration code rather than an uncommitted local deploy helper.
        integrated = source / "scripts/deployment.py"
        if not integrated.exists() or integrated.read_bytes() != Path(__file__).read_bytes():
            raise DeploymentError("部署入口尚未集成到 main_temp；先完成集成，再运行 make deploy")
        require_build_space(source, plan)
        with tempfile.NamedTemporaryFile(mode="w", prefix="aw-deploy-gates-", suffix=".log", delete=False) as log:
            print("自动执行适用检查…", flush=True)
            gates(source, plan, state["public"], log)
        Path(log.name).unlink()
        if not plan["backend"] and not plan["web"]:
            remote(profile, {"action": "verify"}, capture=False)
            print("当前应用已是最新，服务健康。")
            return
        release = "app-" + time.strftime("%Y%m%dT%H%M%SZ", time.gmtime()) + "-" + uuid.uuid4().hex[:8]
        remote(profile, {"action": "prepare", "release": release})
        remote_dir = profile["root"] + "/src.release-" + release
        print("上传不可变发布并自动备份、更新、检查…", flush=True)
        run(["rsync", "--archive", "--checksum", "--exclude=node_modules", "--exclude=dist",
             "--exclude=__pycache__", str(source) + "/", profile["host"] + ":" + remote_dir + "/"])
        web_files = {}
        if plan["web"]:
            dist = source / "frontend/dist"
            web_files = file_manifest(dist)
            run(["rsync", "--archive", "--checksum", str(dist) + "/",
                 profile["host"] + ":" + remote_dir + "/.web/"])
        remote(profile, {"action": "release", "release": release, "revision": revision,
                         "files": manifest, "web_files": web_files, "previous": state["source"]}, capture=False)
        print("部署完成。", flush=True)


def load_remote_env(root):
    path = root / "config/platform.env"
    if not path.is_file() or path.stat().st_mode & 0o077:
        raise DeploymentError("服务器 config/platform.env 必须存在且仅所有者可读")
    # The protected deployment-owned shell file is trusted, never copied to the workstation.
    script = 'set -a; . "$1" >/dev/null; python3 -c "import os,json; print(json.dumps(dict(os.environ)))"'
    return json.loads(run(["bash", "-eu", "-c", script, "deploy", str(path)]))


def container_value(container, expression):
    return run(["docker", "inspect", "--format", expression, "agent-platform-" + container + "-1"]).strip()


def inspect_server(root):
    if not (root / "src").is_symlink() or not (root / "web/current").is_symlink():
        raise DeploymentError("目标还未完成首次安装，请使用首次安装说明；日常部署只接管已有安装")
    source = (root / "src").resolve(strict=True)
    if not source.is_relative_to(root) or not (root / "web/current").resolve(strict=True).is_relative_to(root):
        raise DeploymentError("现有发布指针不在安装目录内")
    env = load_remote_env(root)
    if not (root / "config/platform.https.yaml").is_file():
        raise DeploymentError("服务器缺少 config/platform.https.yaml")
    if Path(env.get("WEB_RELEASE_ROOT", "")) != root / "web":
        raise DeploymentError("WEB_RELEASE_ROOT 应为安装目录/web")
    run(["docker", "compose", "version"])
    if shutil.disk_usage(root).free < 2 * 1024**3:
        raise DeploymentError("服务器可用空间不足 2 GiB，请保留备份并释放无用构建缓存后重试")
    images = {}
    for service in ("api", "worker"):
        if container_value(service, "{{.State.Health.Status}}") != "healthy":
            raise DeploymentError(f"当前 {service} 不健康，请先恢复现有服务")
        images[service] = container_value(service, "{{.Image}}")
    files = file_manifest(source)
    verified = False
    marker = source / ".deployment.json"
    if marker.is_file():
        record = json.loads(marker.read_text())
        verified = record.get("images") == images and record.get("backend_hash") == group_hashes(files)["backend"]
    return {"source": str(source), "files": files, "backend_verified": verified, "busy": active_work(),
            "public": validate_public({k: env.get(k, "") for k in PUBLIC_KEYS})}


def verify_server(root, env):
    for service in ("api", "worker"):
        if container_value(service, "{{.State.Health.Status}}") != "healthy":
            raise DeploymentError(f"{service} 健康检查未通过")
    origin = env["VITE_OIDC_POST_LOGOUT_REDIRECT_URI"].rstrip("/")
    for url in (origin + "/api/healthz", origin + "/api/readyz", origin + "/",
                env["VITE_OIDC_AUTHORITY"].rstrip("/") + "/.well-known/openid-configuration"):
        run(["curl", "--fail", "--silent", "--show-error", "--max-time", "20", "-o", "/dev/null", url])
    web = (root / "web/current").resolve(strict=True)
    entry = web / "index.html"
    paths = ["index.html", *sorted(set(re.findall(r'(?:src|href)="/(assets/[A-Za-z0-9._-]+)"', entry.read_text())))]
    with tempfile.TemporaryDirectory(prefix="aw-public-verify-") as directory:
        for path in paths:
            response = Path(directory) / "response"
            url = origin + ("/" if path == "index.html" else "/" + path)
            run(["curl", "--fail", "--silent", "--show-error", "--max-time", "20", "-o", str(response), url])
            if response.read_bytes() != (web / path).read_bytes():
                raise DeploymentError("公网 Web 内容与当前发布不一致")


def atomic_link(target, link):
    temporary = link.with_name("." + link.name + "-" + uuid.uuid4().hex)
    temporary.symlink_to(target)
    os.replace(temporary, link)


def database(query):
    return run(["docker", "exec", "-i", "agent-platform-postgres-1", "sh", "-c",
                'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At'], input=query).strip()


def active_work():
    return database("SELECT (SELECT count(*) FROM runs WHERE state IN ('queued','running','waiting_for_user'))"
                    "+(SELECT count(*) FROM session_messages WHERE state IN ('queued','generating','waiting_for_user'))"
                    "+(SELECT count(*) FROM image_generation_records WHERE state IN ('pending','running'))"
                    "+(SELECT count(*) FROM assistant_conversation_turns WHERE state = 'generating')"
                    "+(SELECT count(*) FROM knowledge_ingestion_jobs WHERE state IN ('queued','running'))"
                    "+(SELECT count(*) FROM ai_application_knowledge_jobs WHERE state IN ('queued','running'))"
                    "+(SELECT count(*) FROM cli_connector_definitions WHERE state IN ('building','testing'))"
                    "+(SELECT count(*) FROM message_channel_deliveries WHERE state = 'sending');\n") != "0"


def wait_healthy(service):
    for _ in range(60):
        state = container_value(service, "{{.State.Health.Status}}")
        if state == "healthy":
            return
        if state == "unhealthy":
            break
        time.sleep(2)
    raise DeploymentError(f"新 {service} 未通过健康检查")


def cutover(compose, rollback, plan, verify, migration_changed, promote):
    """Rollback binaries only when the actual migration ledger is unchanged."""
    stopped = False
    try:
        if plan["backend"]:
            if active_work():
                raise DeploymentError("还有任务运行或等待处理，稍后直接重试 make deploy")
            stopped = True
            compose(["stop", "api", "worker"])
            if active_work():
                raise DeploymentError("切换前有新任务进入，已取消本次更新")
            compose(["up", "-d", "--no-deps", "api"])
            wait_healthy("api")
            compose(["up", "-d", "--no-deps", "worker"])
            wait_healthy("worker")
        verify()
        promote()
        verify()
    except BaseException:
        if stopped:
            try:
                changed = migration_changed()
            except BaseException:
                changed = True
            if changed:
                compose(["stop", "worker"])
                raise DeploymentError("新 Migration 已应用，Worker 已停止；请使用兼容版本恢复，备份在 backups，详见运维说明")
            rollback()
        raise


def perform_release(root, payload):
    import fcntl
    with (root / "deployment.lock").open("a") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            raise DeploymentError("已有部署正在进行，请稍后重试")
        state = inspect_server(root)
        if state["source"] != payload["previous"]:
            raise DeploymentError("准备期间已有其他部署，重新运行 make deploy 即可")
        source = root / ("src.release-" + payload["release"])
        files = file_manifest(source)
        # .web is generated separately; compare every tracked source byte after transport.
        files = {p: h for p, h in files.items() if not p.startswith(".web/")}
        if files != payload["files"]:
            raise DeploymentError("源码上传校验失败，当前服务未改变")
        plan = make_plan(files, state["files"])
        plan["backend"] = plan["backend"] or not state["backend_verified"]
        if plan["web"] and file_manifest(source / ".web") != payload["web_files"]:
            raise DeploymentError("Web 上传校验失败")
        env = load_remote_env(root)
        env["PLATFORM_CONFIG_FILE"] = str(root / "config/platform.https.yaml")
        previous_web = (root / "web/current").resolve()
        previous_images = {service: container_value(service, "{{.Image}}") for service in ("api", "worker")}
        previous_migrations = database("SELECT name FROM schema_migrations ORDER BY name;\n")
        backup = root / "backups" / ("pre-" + payload["release"])
        backup.mkdir(mode=0o700, parents=True, exist_ok=False)
        with (backup / "deployment.log").open("w") as log:
            for filename in ("platform.env", "platform.https.yaml"):
                shutil.copy2(root / "config" / filename, backup / filename)
                os.chmod(backup / filename, 0o600)
            for service, filename in (("postgres", "business.pgdump"), ("identity-db", "identity.pgdump")):
                with (backup / filename).open("wb") as output:
                    result = subprocess.run(["docker", "exec", "agent-platform-" + service + "-1", "sh", "-c",
                             'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc'], stdout=output, stderr=log)
                    if result.returncode:
                        raise DeploymentError(f"数据库备份失败，详情 {log.name}")
                with (backup / filename).open("rb") as dump:
                    result = subprocess.run(["docker", "exec", "-i", "agent-platform-" + service + "-1", "pg_restore", "-l"],
                                            stdin=dump, stdout=log, stderr=log)
                    if result.returncode:
                        raise DeploymentError("数据库备份校验失败")
            (backup / "recovery.json").write_text(json.dumps({"source": state["source"], "web": str(previous_web),
                        "images": previous_images, "migrations": previous_migrations}, indent=2))
            hashes = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in backup.iterdir() if p.name != "deployment.log"}
            (backup / "checksums.json").write_text(json.dumps(hashes, indent=2))
            override = source / ".deployment-images.json"
            rollback_file = backup / "compose.rollback.json"
            override.write_text(json.dumps({"services": {s: {"image": "agent-platform-" + s + ":" + payload["release"]}
                                                       for s in ("api", "worker")}}))
            rollback_images = {}
            for service, image in previous_images.items():
                tag = "agent-platform-" + service + ":pre-" + payload["release"]
                run(["docker", "tag", image, tag], log=log)
                rollback_images[service] = {"image": tag}
            rollback_file.write_text(json.dumps({"services": rollback_images}))
            base = ["docker", "compose", "--env-file", str(root / "config/platform.env")]
            for filename in ("compose.yaml", "compose.execution.yaml", "compose.https.yaml"):
                base += ["-f", str(source / "deploy/platform" / filename)]

            def compose(arguments, file=override):
                return run(base + ["-f", str(file)] + arguments, cwd=source, env=env, log=log)

            compose(["config", "--quiet"])
            if plan["backend"]:
                print("备份校验通过，构建 API / Worker…", flush=True)
                compose(["build", "api", "worker"])
            web = root / "web/releases" / payload["release"]
            if plan["web"]:
                if not (source / ".web/index.html").is_file() or not (source / ".web/assets").is_dir():
                    raise DeploymentError("Web 发布缺少入口或资源")
                web.parent.mkdir(parents=True, exist_ok=True)
                shutil.copytree(source / ".web", web)
                for path in [web, *web.rglob("*")]:
                    path.chmod(0o755 if path.is_dir() else 0o644)

            def verify():
                verify_server(root, env)
                if plan["backend"]:
                    expected = sorted(Path(p).name for p in files if p.startswith(MIGRATIONS + "/") and p.endswith(".sql"))
                    if database("SELECT name FROM schema_migrations ORDER BY name;\n").splitlines() != expected:
                        raise DeploymentError("Migration 账本未匹配本次版本")

            def rollback():
                compose(["up", "-d", "--no-deps", "api", "worker"], rollback_file)
                atomic_link(previous_web, root / "web/current")
                atomic_link(state["source"], root / "src")
                wait_healthy("api")
                wait_healthy("worker")
                print("部署未完成，已恢复原应用版本。", flush=True)

            def promote():
                if plan["web"]:
                    atomic_link(web, root / "web/current")
                atomic_link(state["source"], root / "src.previous")
                atomic_link(source, root / "src")

            print("自动切换应用并检查健康…", flush=True)
            try:
                cutover(compose, rollback, plan, verify,
                        lambda: database("SELECT name FROM schema_migrations ORDER BY name;\n") != previous_migrations,
                        promote)
            except BaseException:
                # A Web-only failure must also restore its pointer.
                if not plan["backend"]:
                    atomic_link(previous_web, root / "web/current")
                    atomic_link(state["source"], root / "src")
                raise
            (source / ".deployment.json").write_text(json.dumps({"revision": payload["revision"], "plan": plan,
                        "backend_hash": group_hashes(files)["backend"],
                        "images": {s: container_value(s, "{{.Image}}") for s in ("api", "worker")}}, indent=2))
            print(f"发布 {payload['release']} 已通过健康、Readiness 和 OIDC 检查。", flush=True)


def remote_main(payload):
    import signal

    def interrupted(signum, frame):
        raise DeploymentError("部署连接中断，已进入恢复流程")

    for sig in (signal.SIGTERM, signal.SIGHUP, signal.SIGINT):
        signal.signal(sig, interrupted)
    root = Path(validate_profile({"host": "remote", "root": payload["root"]})["root"])
    action = payload["action"]
    if action == "inspect":
        print(json.dumps(inspect_server(root)))
    elif action == "verify":
        inspect_server(root)
        verify_server(root, load_remote_env(root))
        print("HTTPS、API、Worker 和 OIDC 检查通过。")
    else:
        if not re.fullmatch(r"app-[A-Za-z0-9-]+", payload.get("release", "")):
            raise DeploymentError("发布编号无效")
        if action == "prepare":
            inspect_server(root)
            (root / ("src.release-" + payload["release"])).mkdir(mode=0o700, exist_ok=False)
            print("{}")
        elif action == "release":
            perform_release(root, payload)
        else:
            raise DeploymentError("不支持的部署操作")


def main():
    parser = argparse.ArgumentParser(description="首次保存服务器，之后一个命令发布 main_temp 中的应用更新。")
    parser.add_argument("--configure", action="store_true", help="保存服务器与安装目录")
    parser.add_argument("--check", action="store_true", help="只检查配置和更新范围")
    parser.add_argument("--host", help="SSH 别名或 user@host")
    parser.add_argument("--root", help="已有安装的根目录")
    parser.add_argument("--remote", help=argparse.SUPPRESS)
    args = parser.parse_args()
    try:
        if args.remote:
            os.umask(0o077)
            remote_main(json.loads(args.remote))
        else:
            local_main(args)
    except (DeploymentError, ValueError, OSError) as error:
        print(f"部署停止：{error}", file=sys.stderr, flush=True)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
