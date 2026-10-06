#!/usr/bin/env python3
"""Install a new, dedicated Ubuntu server from the integrated main_temp revision."""
import argparse
import base64
import fcntl
import importlib.util
import json
import os
from pathlib import Path
import platform
import re
import secrets
import shlex
import shutil
import socket
import subprocess
import sys
import tarfile
import tempfile
import time
import uuid
from urllib.parse import quote, urlencode

import deployment as deploy


def validate_options(options):
    profile = deploy.validate_profile({key: options[key] for key in ("host", "root")})
    domain, email = options["domain"], options["email"]
    if not isinstance(domain, str) or len(domain) > 253 or not re.fullmatch(
            r"(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}", domain):
        raise deploy.DeploymentError("域名只填写主机名，例如 workspace.example.com")
    if not isinstance(email, str) or len(email) > 254 or not re.fullmatch(
            r"[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}", email):
        raise deploy.DeploymentError("请填写有效的管理员邮箱")
    return {**profile, "domain": domain, "email": email}


def public_config(options):
    origin = "https://" + options["domain"]
    return {"VITE_OIDC_AUTHORITY": origin + "/identity/realms/agent-platform",
            "VITE_OIDC_CLIENT_ID": "agent-platform-web",
            "VITE_OIDC_REDIRECT_URI": origin + "/auth/callback",
            "VITE_OIDC_POST_LOGOUT_REDIRECT_URI": origin}


def generate_config(source, options):
    """Pure generation; only the server calls this with real installation credentials."""
    root = options["root"]
    values = {}
    for line in (source / "deploy/platform/.env.example").read_text().splitlines():
        if line and not line.startswith("#"):
            key, value = line.split("=", 1)
            values[key] = value.strip('"')
    values.update(public_config(options))
    values.update(PUBLIC_HOST=options["domain"], ACME_EMAIL=options["email"],
                  BOOTSTRAP_ADMIN_EMAIL=options["email"], BOOTSTRAP_ADMIN_SUBJECT=str(uuid.uuid4()),
                  OIDC_ISSUER=values["VITE_OIDC_AUTHORITY"],
                  KEYCLOAK_BASE_URL="https://" + options["domain"] + "/identity",
                  DATA_ENCRYPTION_KEY=base64.b64encode(secrets.token_bytes(32)).decode(),
                  # Compose requires a nonempty value; unavailable Runtimes have an empty digest in YAML.
                  RUNTIME_IMAGE="disabled-until-conformance",
                  REGISTRATION_PUBLIC_URL="https://" + options["domain"],
                  REGISTRATION_CLIENT_SECRET=secrets.token_urlsafe(48))
    for key in ("POSTGRES_PASSWORD", "MINIO_ROOT_PASSWORD", "KEYCLOAK_DB_PASSWORD",
                "KEYCLOAK_ADMIN_PASSWORD", "PLATFORM_ADMIN_PASSWORD", "KEYCLOAK_SERVICE_CLIENT_SECRET"):
        values[key] = secrets.token_urlsafe(48)
    for key, suffix in {"WEB_RELEASE_ROOT": "web", "OIDC_REALM_FILE": "config/keycloak-realm.json",
                        "KEYCLOAK_THEME_ROOT": "identity-themes/current", "WORKSPACE_ROOT": "workspaces",
                        "WORKSPACE_KNOWN_HOSTS": "config/known_hosts", "CREDENTIAL_TEMP_ROOT": "run-credentials",
                        "SANDBOX_RESOLVER_FILE": "config/sandbox-resolv.conf"}.items():
        values[key] = root + "/" + suffix
    yaml = (source / "deploy/platform/config/platform.https.yaml").read_text()
    yaml = yaml.replace("available: true", "available: false").replace("native_resume: true", "native_resume: false")
    yaml = yaml.replace('image_digest: "${RUNTIME_IMAGE}"', 'image_digest: ""')
    realm = json.loads((source / "deploy/platform/config/keycloak-realm.json").read_text())
    administrator = realm["users"][0]
    administrator.update(id=values["BOOTSTRAP_ADMIN_SUBJECT"], email=options["email"])
    for client in realm["clients"]:
        if client["clientId"] == values["VITE_OIDC_CLIENT_ID"]:
            client["redirectUris"] = [values["VITE_OIDC_REDIRECT_URI"]]
            client["webOrigins"] = [values["VITE_OIDC_POST_LOGOUT_REDIRECT_URI"]]
            client["attributes"]["post.logout.redirect.uris"] = values["VITE_OIDC_POST_LOGOUT_REDIRECT_URI"]
    return values, yaml, realm


def private_write(path, value):
    with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, delete=False) as output:
        temporary = Path(output.name)
        output.write(value)
        output.flush()
        os.fsync(output.fileno())
    try:
        temporary.chmod(0o600)
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)


def existing_state(root, options):
    if root.is_symlink() or root.resolve() != root:
        raise deploy.DeploymentError("安装目录及父目录不能是符号链接")
    marker = root / ".installation.json"
    if not marker.is_file():
        if root.exists() and any(path.name != ".installation.lock" for path in root.iterdir()):
            raise deploy.DeploymentError("安装目录已有文件；首次安装不会覆盖已有配置，请使用 make deploy")
        return None
    if marker.stat().st_mode & 0o077:
        raise deploy.DeploymentError("安装状态文件必须仅所有者可读")
    state = json.loads(marker.read_text())
    if state.get("options") != options:
        raise deploy.DeploymentError("安装参数与上次不同；请使用原服务器、目录、域名和邮箱重试")
    if state.get("complete"):
        raise deploy.DeploymentError("该服务器已经安装完成，更新请使用 make deploy")
    if not re.fullmatch(r"[a-f0-9]{40}", state.get("revision", "")):
        raise deploy.DeploymentError("安装版本记录无效")
    return state


def preflight(root, options):
    if os.geteuid() != 0 or platform.system() != "Linux" or platform.machine() != "x86_64":
        raise deploy.DeploymentError("首次安装需要 Ubuntu 22.04/24.04 amd64 专用服务器的 root SSH 权限")
    state = existing_state(root, options)
    release = dict(line.split("=", 1) for line in Path("/etc/os-release").read_text().splitlines() if "=" in line)
    if release.get("ID", "").strip('"') != "ubuntu" or release.get("VERSION_ID", "").strip('"') not in ("22.04", "24.04"):
        raise deploy.DeploymentError("自动安装当前支持 Ubuntu 22.04/24.04")
    ancestor = root
    while not ancestor.exists():
        ancestor = ancestor.parent
    if shutil.disk_usage(ancestor).free < 15 * 1024**3:
        raise deploy.DeploymentError("新安装需要至少 15 GiB 可用空间（包含构建空间）")
    socket.getaddrinfo(options["domain"], 443)
    if shutil.which("docker"):
        containers = set(deploy.run(["docker", "ps", "-aq"]).splitlines())
        owned = deploy.run(["docker", "ps", "-aq", "--filter", "label=com.docker.compose.project=agent-platform"]).strip()
        foreign = containers - set(owned.splitlines())
        volumes = deploy.run(["docker", "volume", "ls", "-q", "--filter", "label=com.docker.compose.project=agent-platform"]).strip()
        named_volumes = deploy.run(["docker", "volume", "ls", "-q", "--filter", "name=agent-platform_"]).strip()
        if foreign or (not state and (owned or volumes or named_volumes)):
            raise deploy.DeploymentError("服务器已有 Docker 容器或平台数据；请使用专用空服务器，已有平台使用 make deploy")
        if state and owned:
            for container in owned.splitlines():
                location = deploy.run(["docker", "inspect", "--format", '{{index .Config.Labels "com.docker.compose.project.working_dir"}}', container]).strip()
                if Path(location).resolve() != (root / "src/deploy/platform").resolve():
                    raise deploy.DeploymentError("同名平台容器属于另一安装目录，拒绝接管")
    if not state:
        for port in (80, 443):
            with socket.socket() as probe:
                try:
                    probe.bind(("0.0.0.0", port))
                except OSError:
                    raise deploy.DeploymentError(f"服务器端口 {port} 已被占用")
    return state


def prepare_upload(root, options, payload):
    preflight(root, options)
    root.mkdir(parents=True, mode=0o755, exist_ok=True)
    with (root / ".installation.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        state = existing_state(root, options)
        if state != payload["state"]:
            raise deploy.DeploymentError("安装状态已变化，请重新执行安装命令")
        if state is None:
            state = {"options": options, "revision": payload["revision"], "complete": False}
            private_write(root / ".installation.json", json.dumps(state))
        (root / payload["upload"]).mkdir(mode=0o700)
        return state


def provision_host(log):
    env = {**os.environ, "DEBIAN_FRONTEND": "noninteractive"}
    command = lambda args: deploy.run(args, env=env, log=log)
    command(["apt-get", "update"])
    command(["apt-get", "install", "-y", "ca-certificates", "curl", "gnupg", "iptables", "openssl"])
    Path("/etc/apt/keyrings").mkdir(mode=0o755, exist_ok=True)
    if not shutil.which("docker"):
        command(["curl", "-fsSL", "--retry", "3", "https://download.docker.com/linux/ubuntu/gpg", "-o", "/etc/apt/keyrings/docker.asc"])
        Path("/etc/apt/keyrings/docker.asc").chmod(0o644)
        release = dict(line.split("=", 1) for line in Path("/etc/os-release").read_text().splitlines() if "=" in line)
        codename = release["VERSION_CODENAME"].strip('"')
        Path("/etc/apt/sources.list.d/docker.sources").write_text(
            "Types: deb\nURIs: https://download.docker.com/linux/ubuntu\nSuites: " + codename +
            "\nComponents: stable\nArchitectures: amd64\nSigned-By: /etc/apt/keyrings/docker.asc\n")
        command(["apt-get", "update"])
        command(["apt-get", "install", "-y", "docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin"])
    command(["docker", "compose", "version"])
    if not shutil.which("runsc"):
        command(["curl", "-fsSL", "--retry", "3", "https://gvisor.dev/archive.key", "-o", "/etc/apt/keyrings/gvisor.asc"])
        Path("/etc/apt/keyrings/gvisor.asc").chmod(0o644)
        Path("/etc/apt/sources.list.d/gvisor.list").write_text(
            "deb [arch=amd64 signed-by=/etc/apt/keyrings/gvisor.asc] https://storage.googleapis.com/gvisor/releases release main\n")
        command(["apt-get", "update"])
        command(["apt-get", "install", "-y", "runsc"])
    daemon = Path("/etc/docker/daemon.json")
    daemon.parent.mkdir(mode=0o755, exist_ok=True)
    settings = json.loads(daemon.read_text()) if daemon.exists() else {}
    desired = {"path": shutil.which("runsc"), "runtimeArgs": ["--host-uds=open"]}
    if settings.get("runtimes", {}).get("runsc") != desired:
        settings.setdefault("runtimes", {})["runsc"] = desired
        private_write(daemon, json.dumps(settings, indent=2) + "\n")
        command(["systemctl", "restart", "docker"])
    command(["systemctl", "enable", "docker"])
    runtimes = json.loads(deploy.run(["docker", "info", "--format", "{{json .Runtimes}}"], env=env))
    if "runsc" not in runtimes:
        raise deploy.DeploymentError("Docker 未注册 runsc，安装停止")
    command(["docker", "version", "--format", "{{.Server.Version}}"])
    command(["runsc", "--version"])


def prepare_config(root, source, options, log):
    for name in ("workspaces", "run-credentials"):
        directory = root / name
        if directory.is_symlink():
            raise deploy.DeploymentError("持久目录不能是符号链接")
        directory.mkdir(mode=0o700, exist_ok=True)
        directory.chmod(0o700)
        if name == "workspaces":
            os.chown(directory, 65532, 65532)
    config = root / "config"
    if config.exists():
        for name in ("platform.env", "platform.https.yaml", "keycloak-realm.json", "admin-access.txt"):
            if not (config / name).is_file():
                raise deploy.DeploymentError("已有安装配置不完整，停止以避免重新生成密钥")
        return deploy.load_remote_env(root)
    values, yaml, realm = generate_config(source, options)
    # PKCS8 DER is generated and encoded entirely on the server; never uploaded/downloaded.
    key = deploy.run(["openssl", "genpkey", "-algorithm", "RSA", "-pkeyopt", "rsa_keygen_bits:2048"])
    result = subprocess.run(["openssl", "pkcs8", "-topk8", "-nocrypt", "-outform", "DER"],
                            input=key.encode(), capture_output=True)
    if result.returncode:
        raise deploy.DeploymentError("注册签名密钥生成失败")
    values["REGISTRATION_SIGNING_KEY"] = base64.b64encode(result.stdout).decode()
    with tempfile.TemporaryDirectory(prefix=".config-", dir=root) as temporary:
        staging = Path(temporary)
        private_write(staging / "platform.env", "".join(key + "=" + shlex.quote(value) + "\n" for key, value in values.items()))
        private_write(staging / "platform.https.yaml", yaml)
        os.chown(staging / "platform.https.yaml", 65532, 65532)
        (staging / "platform.https.yaml").chmod(0o400)
        private_write(staging / "keycloak-realm.json", json.dumps(realm, indent=2) + "\n")
        os.chown(staging / "keycloak-realm.json", 1000, 0)
        (staging / "keycloak-realm.json").chmod(0o400)
        (staging / "known_hosts").write_text("")
        (staging / "known_hosts").chmod(0o644)
        private_write(staging / "admin-access.txt", "地址：https://" + options["domain"] +
                      "\n账号：platform-admin\n密码：" + values["PLATFORM_ADMIN_PASSWORD"] + "\n登录后请修改密码。\n")
        os.rename(staging, config)
    return values


def configure_identity(source, values):
    spec = importlib.util.spec_from_file_location("install_identity", source / "scripts/configure-identity-theme.py")
    identity = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(identity)
    token_url, realm_url = identity.endpoints(values["VITE_OIDC_AUTHORITY"])
    body = urlencode({"grant_type": "password", "client_id": "admin-cli", "username": values["KEYCLOAK_ADMIN_USER"],
                      "password": values["KEYCLOAK_ADMIN_PASSWORD"]}).encode()
    token = identity.request(token_url, "POST", body, content_type="application/x-www-form-urlencoded")["access_token"]
    request = lambda path, method="GET", body=None: identity.request(realm_url + path, method, body, token)
    clients = request("/clients?clientId=" + quote(values["KEYCLOAK_SERVICE_CLIENT_ID"], safe=""))
    management = request("/clients?clientId=realm-management")
    if len(clients) != 1 or len(management) != 1:
        raise deploy.DeploymentError("身份服务的账号管理 client 不完整")
    account = request("/clients/" + clients[0]["id"] + "/service-account-user")
    roles = request("/clients/" + management[0]["id"] + "/roles")
    wanted = {"manage-users", "view-users", "query-users", "query-groups", "manage-identity-providers"}
    grants = [role for role in roles if role["name"] in wanted]
    if len(grants) != len(wanted):
        raise deploy.DeploymentError("身份服务缺少必要的 realm 账号管理角色")
    role_path = "/users/" + account["id"] + "/role-mappings/clients/" + management[0]["id"]
    request(role_path, "POST", json.dumps(grants).encode())
    if not wanted.issubset({role["name"] for role in request(role_path)}):
        raise deploy.DeploymentError("身份服务账号管理角色验证失败")
    deploy.run(["python3", str(source / "scripts/configure-registration-identity.py"), "apply"],
               env={**os.environ, **values})


def wait_for(check, description, seconds=300):
    deadline = time.monotonic() + seconds
    while True:
        try:
            check()
            return
        except Exception:
            if time.monotonic() >= deadline:
                raise deploy.DeploymentError(description + "未通过；检查受限安装日志，修正后运行相同命令继续") from None
            time.sleep(5)


def extract_bundle(archive, destination):
    with tarfile.open(archive) as bundle:
        for item in bundle.getmembers():
            if not (item.isfile() or item.isdir()) or not (destination / item.name).resolve().is_relative_to(destination.resolve()):
                raise deploy.DeploymentError("安装包包含不安全路径或链接")
        bundle.extractall(destination)


def perform_install(root, options, payload):
    root.mkdir(parents=True, mode=0o755, exist_ok=True)
    with (root / ".installation.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        state = payload["state"]
        # The local check happened before upload; reject concurrent changes and installations here too.
        marker = root / ".installation.json"
        actual = json.loads(marker.read_text()) if marker.is_file() else None
        if actual != state:
            raise deploy.DeploymentError("安装状态已变化，请重新执行安装命令")
        preflight(root, options)
        if state is None or state["complete"] or state["revision"] != payload["revision"]:
            raise deploy.DeploymentError("重试必须继续原安装版本")
        upload = root / payload["upload"]
        source = root / ("src.release-install-" + state["revision"][:12])
        with (root / "installation.log").open("a") as log:
            if not source.exists():
                staging = upload / "source"
                staging.mkdir()
                extract_bundle(upload / "source.tar", staging)
                if deploy.file_manifest(staging) != payload["files"]:
                    raise deploy.DeploymentError("上传源码校验失败")
                os.rename(staging, source)
            current_files = deploy.file_manifest(source)
            current_files.pop(".deployment.json", None)
            if current_files != payload["files"]:
                raise deploy.DeploymentError("安装源码被修改，停止重试")
            print("安装 Docker / gVisor 并生成服务器配置…", flush=True)
            provision_host(log)
            values = prepare_config(root, source, options, log)
            theme = root / "identity-themes/releases/initial"
            if not theme.exists():
                temporary_theme = theme.parent / (".build-" + uuid.uuid4().hex)
                deploy.run(["python3", str(source / "scripts/build-identity-theme.py"), str(temporary_theme)], log=log)
                os.rename(temporary_theme, theme)
            for path in [theme.parent.parent, theme.parent, theme, *theme.rglob("*")]:
                path.chmod(0o755 if path.is_dir() else 0o644)
            deploy.atomic_link(theme, root / "identity-themes/current")
            web = root / "web/releases/initial"
            if not web.exists():
                web.parent.mkdir(parents=True, exist_ok=True)
                staged_web = upload / "web"
                staged_web.mkdir()
                extract_bundle(upload / "web.tar", staged_web)
                if deploy.file_manifest(staged_web) != payload["web_files"] or not (staged_web / "index.html").is_file():
                    raise deploy.DeploymentError("Web 上传校验失败")
                os.rename(staged_web, web)
            if deploy.file_manifest(web) != payload["web_files"]:
                raise deploy.DeploymentError("Web 发布被修改，停止重试")
            for path in [web.parent.parent, web.parent, web, *web.rglob("*")]:
                path.chmod(0o755 if path.is_dir() else 0o644)
            deploy.atomic_link(web, root / "web/current")
            deploy.atomic_link(source, root / "src")
            egress_env = {key: values[key] for key in ("AGENT_EGRESS_NETWORK", "AGENT_EGRESS_SUBNET", "AGENT_DNS_SERVERS")}
            egress_env["AGENT_RESOLVER_CONFIG_FILE"] = values["SANDBOX_RESOLVER_FILE"]
            private_write(root / "config/egress.env", "".join(k + "=" + shlex.quote(v) + "\n" for k, v in egress_env.items()))
            unit = (source / "deploy/sandbox/agent-platform-egress.service").read_text().replace("/srv/agent-workspace", str(root))
            private_write(Path("/etc/systemd/system/agent-platform-egress.service"), unit)
            deploy.run(["systemctl", "daemon-reload"], log=log)
            deploy.run(["systemctl", "enable", "--now", "agent-platform-egress"], log=log)
            command = ["docker", "compose", "--env-file", str(root / "config/platform.env")]
            for name in ("compose.yaml", "compose.https.yaml", "compose.execution.yaml"):
                command.extend(["-f", str(source / "deploy/platform" / name)])
            env = {**os.environ, "PLATFORM_CONFIG_FILE": str(root / "config/platform.https.yaml")}
            compose = lambda args: deploy.run(command + args, env=env, log=log)
            compose(["config", "--quiet"])
            print("构建服务，启动数据库、存储与身份服务…", flush=True)
            compose(["build", "api", "worker", "egress-controller"])
            compose(["up", "-d", "postgres", "minio", "minio-init", "identity-db", "identity"])
            # Caddy must establish TLS/discovery before the API performs OIDC startup validation.
            compose(["up", "-d", "--no-deps", "caddy"])
            wait_for(lambda: deploy.run(["curl", "-fsS", "--max-time", "15", "-o", "/dev/null",
                                         values["VITE_OIDC_AUTHORITY"] + "/.well-known/openid-configuration"], log=log), "HTTPS / 身份服务", 600)
            print("配置管理员权限，启动 API / Worker 并检查入口…", flush=True)
            configure_identity(source, values)
            compose(["up", "-d", "api"])
            wait_for(lambda: deploy.run(["docker", "exec", "agent-platform-api-1", "wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8080/readyz"], log=log), "API")
            compose(["up", "-d", "egress-controller", "worker"])
            wait_for(lambda: deploy.verify_server(root, values), "服务与 Web 资源")
            deploy.verify_migrations(payload["files"], deploy.migration_ledger(), {})
            images = {name: deploy.container_value(name, "{{.Image}}") for name in ("api", "worker")}
            private_write(source / ".deployment.json", json.dumps({"revision": state["revision"], "images": images,
                          "backend_hash": deploy.group_hashes(payload["files"])["backend"]}))
            state["complete"] = True
            private_write(marker, json.dumps(state))
        shutil.rmtree(upload)
        print("安装完成：https://" + options["domain"] + "；初始管理员凭据保存在 " + str(root / "config/admin-access.txt"), flush=True)


def remote_main(payload):
    options = validate_options(payload["options"])
    root = Path(options["root"])
    if payload["action"] == "check":
        state = preflight(root, options)
        print(json.dumps({"state": state}))
    elif payload["action"] in ("prepare", "install"):
        if not re.fullmatch(r"\.install-upload-[a-f0-9]{32}", payload.get("upload", "")) or not re.fullmatch(r"[a-f0-9]{40}", payload.get("revision", "")):
            raise deploy.DeploymentError("安装包标识无效")
        if payload["action"] == "prepare":
            print(json.dumps({"state": prepare_upload(root, options, payload)}))
        else:
            perform_install(root, options, payload)
    else:
        raise deploy.DeploymentError("未知安装操作")


def remote(options, payload, capture=True):
    bootstrap = ("import sys,json,types; payload=sys.stdin.readline(); source=json.loads(sys.stdin.readline()); "
                 "m=types.ModuleType('deployment'); m.__file__='<deployment>'; sys.modules['deployment']=m; "
                 "exec(compile(source,'<deployment>','exec'),m.__dict__); "
                 "sys.argv=['installation','--remote',payload]; exec(compile(sys.stdin.read(),'<installation>','exec'))")
    contents = json.dumps({**payload, "options": options}) + "\n" + json.dumps(Path(deploy.__file__).read_text()) + "\n" + Path(__file__).read_text()
    result = subprocess.run(["ssh", "-o", "BatchMode=yes", options["host"], "python3 -c " + shlex.quote(bootstrap)],
                            input=contents, text=True, capture_output=capture)
    if result.returncode:
        message = next((line.removeprefix("安装停止：") for line in (result.stderr or "").splitlines()
                        if line.startswith("安装停止：")), "服务器安装未完成，请检查 SSH 或服务器受限安装日志后重试")
        raise deploy.DeploymentError(message)
    return json.loads(result.stdout) if capture else None


def local_main(args):
    values = {key: getattr(args, key) for key in ("host", "root", "domain", "email")}
    for key, prompt in {"host": "服务器（root@host 或 root SSH 别名）：", "domain": "平台域名：", "email": "管理员邮箱："}.items():
        if not values[key]:
            if not sys.stdin.isatty():
                raise deploy.DeploymentError("请提供 --host、--domain、--email，或在终端运行 make install")
            values[key] = input(prompt).strip()
    options = validate_options(values)
    print("检查新服务器与安装状态…", flush=True)
    state = remote(options, {"action": "check"})["state"]
    if args.check:
        print("服务器检查通过。" + ("可继续上次安装。" if state else "可开始首次安装。"))
        return
    with tempfile.TemporaryDirectory(prefix="aw-install-") as temporary:
        source = Path(temporary) / "source"
        repo = Path(__file__).resolve().parent.parent
        revision = deploy.snapshot(repo, source)
        if state and state["revision"] != revision:
            shutil.rmtree(source)
            deploy.run(["git", "archive", "--format=tar", "-o", str(Path(temporary) / "source.tar"), state["revision"]], cwd=repo)
            source.mkdir()
            extract_bundle(Path(temporary) / "source.tar", source)
            revision = state["revision"]
        deploy.require_build_space(source, {"backend": True, "web": True})
        print("验证集成版本并构建 Web…", flush=True)
        log_path = deploy.profile_path().parent / "installation-build.log"
        log_path.parent.mkdir(parents=True, mode=0o700, exist_ok=True)
        with log_path.open("w") as log:
            log_path.chmod(0o600)
            deploy.gates(source, {"backend": True, "web": True}, public_config(options), log)
        dist = source / "frontend/dist"
        with tarfile.open(Path(temporary) / "web.tar", "w") as bundle:
            for path in sorted(dist.rglob("*")):
                bundle.add(path, arcname=path.relative_to(dist), recursive=False)
        payload = {"action": "install", "state": state, "revision": revision,
                   "files": deploy.file_manifest(source), "web_files": deploy.file_manifest(dist),
                   "upload": ".install-upload-" + uuid.uuid4().hex}
        destination = options["root"] + "/" + payload["upload"]
        payload["state"] = remote(options, {**payload, "action": "prepare"})["state"]
        deploy.run(["scp", "-q", str(Path(temporary) / "source.tar"), str(Path(temporary) / "web.tar"), options["host"] + ":" + destination + "/"])
        remote(options, payload, capture=False)
    deploy.save_profile({key: options[key] for key in ("host", "root")}, deploy.profile_path())
    print("部署地址已保存；以后更新运行 make deploy。")


def main():
    parser = argparse.ArgumentParser(description="首次安装：只填写服务器、域名与管理员邮箱，其余自动配置。")
    parser.add_argument("--host", help="root SSH 别名或 root@host")
    parser.add_argument("--root", default="/srv/agent-workspace", help="安装目录")
    parser.add_argument("--domain", help="已解析到服务器的域名")
    parser.add_argument("--email", help="管理员邮箱，同时用于 HTTPS 证书")
    parser.add_argument("--check", action="store_true", help="只检查服务器，不安装")
    parser.add_argument("--remote", help=argparse.SUPPRESS)
    args = parser.parse_args()
    try:
        if args.remote:
            os.umask(0o077)
            remote_main(json.loads(args.remote))
        else:
            local_main(args)
    except Exception as error:
        # HTTP responses / command output may include Secret values. Only our own errors are public.
        message = str(error) if isinstance(error, deploy.DeploymentError) else "检查失败，请查看服务器受限 installation.log 并重试"
        print("安装停止：" + message, file=sys.stderr, flush=True)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
