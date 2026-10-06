import contextlib
import io
import json
import os
import shutil
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import deployment
import installation as install


REPO = Path(__file__).resolve().parents[1]
OPTIONS = {"host": "root@server.example.com", "root": "/srv/agent-workspace",
           "domain": "workspace.example.com", "email": "admin@example.com"}


class InstallationTests(unittest.TestCase):
    def test_options_reject_command_injection_urls_and_unsafe_paths(self):
        for key, value in (("host", "-oProxyCommand=bad"), ("root", "/"), ("root", "/srv/../etc"),
                           ("domain", "https://example.com"), ("domain", "a.example.com;id"),
                           ("domain", "-bad.example.com"), ("email", "a@example.com\nKEY=bad")):
            with self.subTest(key=key, value=value), self.assertRaises(deployment.DeploymentError):
                install.validate_options({**OPTIONS, key: value})
        self.assertEqual(install.validate_options(OPTIONS), OPTIONS)

    def test_config_derives_urls_independent_keys_stable_subject_and_disabled_runtimes(self):
        values, yaml, realm = install.generate_config(REPO, OPTIONS)
        secret_keys = ("POSTGRES_PASSWORD", "MINIO_ROOT_PASSWORD", "KEYCLOAK_DB_PASSWORD", "KEYCLOAK_ADMIN_PASSWORD",
                       "PLATFORM_ADMIN_PASSWORD", "KEYCLOAK_SERVICE_CLIENT_SECRET", "REGISTRATION_CLIENT_SECRET")
        self.assertEqual(len(set(values[key] for key in secret_keys)), len(secret_keys))
        self.assertTrue(all(len(values[key]) >= 32 for key in secret_keys))
        self.assertEqual(values["BOOTSTRAP_ADMIN_SUBJECT"], realm["users"][0]["id"])
        self.assertEqual(OPTIONS["email"], realm["users"][0]["email"])
        self.assertNotIn("available: true", yaml)
        self.assertNotIn("native_resume: true", yaml)
        self.assertNotIn("${RUNTIME_IMAGE}", yaml)
        self.assertIn("runtime: runsc", yaml)
        web = next(client for client in realm["clients"] if client["clientId"] == values["VITE_OIDC_CLIENT_ID"])
        self.assertEqual(web["redirectUris"], ["https://workspace.example.com/auth/callback"])
        self.assertEqual(web["attributes"]["post.logout.redirect.uris"], "https://workspace.example.com")
        self.assertFalse(web["directAccessGrantsEnabled"])
        self.assertEqual(values["WORKSPACE_ROOT"], "/srv/agent-workspace/workspaces")

    def test_existing_installation_or_changed_parameters_never_regenerate_keys(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            options = {**OPTIONS, "root": str(root)}
            self.assertIsNone(install.existing_state(root, options))
            (root / "config").mkdir()
            (root / "config/platform.env").write_text("DO_NOT_ROTATE=secret")
            with self.assertRaises(deployment.DeploymentError):
                install.existing_state(root, options)
            marker = root / ".installation.json"
            state = {"options": options, "revision": "a" * 40, "complete": False}
            install.private_write(marker, json.dumps(state))
            self.assertEqual(install.existing_state(root, options), state)
            with self.assertRaises(deployment.DeploymentError):
                install.existing_state(root, {**options, "email": "other@example.com"})
            state["complete"] = True
            install.private_write(marker, json.dumps(state))
            with self.assertRaisesRegex(deployment.DeploymentError, "make deploy"):
                install.existing_state(root, options)
            self.assertEqual((root / "config/platform.env").read_text(), "DO_NOT_ROTATE=secret")
            self.assertEqual(marker.stat().st_mode & 0o777, 0o600)

    def test_preflight_refuses_other_container_or_existing_named_platform_volume(self):
        real_read = Path.read_text
        def read(path, *args, **kwargs):
            if str(path) == "/etc/os-release":
                return 'ID=ubuntu\nVERSION_ID="24.04"\nVERSION_CODENAME=noble\n'
            return real_read(path, *args, **kwargs)
        for forbidden in ("foreign", "volume"):
            with self.subTest(forbidden=forbidden), tempfile.TemporaryDirectory() as directory:
                root = Path(directory).resolve()
                options = {**OPTIONS, "root": str(root)}
                def run(args, **kwargs):
                    if forbidden == "foreign" and args == ["docker", "ps", "-aq"]:
                        return "unrelated-service"
                    if forbidden == "volume" and "name=agent-platform_" in args:
                        return "agent-platform_postgres-data"
                    return ""
                with patch("installation.os.geteuid", return_value=0), patch("installation.platform.system", return_value="Linux"), patch(
                        "installation.platform.machine", return_value="x86_64"), patch("pathlib.Path.read_text", read), patch(
                        "installation.shutil.disk_usage") as disk, patch("installation.socket.getaddrinfo"), patch(
                        "installation.shutil.which", return_value="docker"), patch("deployment.run", side_effect=run):
                    disk.return_value.free = 100 * 1024**3
                    with self.assertRaises(deployment.DeploymentError):
                        install.preflight(root, options)
                self.assertEqual(list(root.iterdir()), [])

    def test_wait_reports_failed_phase_without_raw_secret_errors(self):
        with patch("installation.time.monotonic", side_effect=[0, 2]):
            with self.assertRaises(deployment.DeploymentError) as error:
                install.wait_for(lambda: (_ for _ in ()).throw(RuntimeError("password=secret")), "API", 1)
        self.assertIn("API", str(error.exception))
        self.assertNotIn("secret", str(error.exception))

    def test_config_creation_is_atomic_and_retry_keeps_password_encryption_key_and_subject(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            options = {**OPTIONS, "root": str(root)}
            with patch("installation.os.chown"), patch("deployment.run", return_value="fake-private-key"), patch(
                    "installation.subprocess.run") as process:
                process.return_value.returncode = 0
                process.return_value.stdout = b"fake-der-key"
                original = install.prepare_config(root, REPO, options, None)
            self.assertEqual((root / "config/platform.env").stat().st_mode & 0o777, 0o600)
            self.assertEqual((root / "config/platform.https.yaml").stat().st_mode & 0o777, 0o400)
            self.assertEqual((root / "config/admin-access.txt").stat().st_mode & 0o777, 0o600)
            shutil.rmtree(root / "workspaces")  # Simulate interruption after committing credentials, before mounting directories.
            with patch("installation.os.chown"), patch("installation.generate_config", side_effect=AssertionError("must not rotate")):
                retried = install.prepare_config(root, REPO, options, None)
            self.assertEqual((root / "workspaces").stat().st_mode & 0o777, 0o700)
            for key in ("PLATFORM_ADMIN_PASSWORD", "DATA_ENCRYPTION_KEY", "BOOTSTRAP_ADMIN_SUBJECT", "REGISTRATION_SIGNING_KEY"):
                self.assertEqual(original[key], retried[key])

    def test_failed_key_generation_leaves_no_partial_configuration(self):
        with tempfile.TemporaryDirectory() as directory, patch("installation.os.chown"), patch("deployment.run", return_value="key"), patch("installation.subprocess.run") as process:
            process.return_value.returncode = 1
            with self.assertRaises(deployment.DeploymentError):
                install.prepare_config(Path(directory), REPO, OPTIONS, None)
            self.assertFalse((Path(directory) / "config").exists())

    def test_archives_reject_symlinks_and_path_traversal(self):
        for name, kind in (("../../escaped", tarfile.REGTYPE), ("link", tarfile.SYMTYPE)):
            with self.subTest(name=name), tempfile.TemporaryDirectory() as directory:
                archive = Path(directory) / "bad.tar"
                with tarfile.open(archive, "w") as bundle:
                    member = tarfile.TarInfo(name)
                    member.type = kind
                    member.linkname = "/etc/passwd"
                    bundle.addfile(member)
                with self.assertRaises(deployment.DeploymentError):
                    install.extract_bundle(archive, Path(directory) / "output")

    def test_identity_permissions_are_verified_without_realm_admin_or_group_mutation(self):
        values = install.generate_config(REPO, OPTIONS)[0]
        roles = [{"id": name, "name": name} for name in ("manage-users", "view-users", "query-users", "query-groups", "manage-identity-providers", "realm-admin")]
        calls = []
        def request(url, method="GET", body=None, token=None, content_type=None):
            calls.append((url, method, body))
            if url.endswith("/token"):
                self.assertEqual(content_type, "application/x-www-form-urlencoded")
                self.assertNotIn(values["KEYCLOAK_ADMIN_PASSWORD"], url)
                return {"access_token": "private-token"}
            if "/clients?" in url:
                return [{"id": "management" if "realm-management" in url else "service"}]
            if url.endswith("/service-account-user"):
                return {"id": "service-user"}
            if url.endswith("/roles"):
                return roles
            if method == "POST":
                grants = json.loads(body)
                self.assertNotIn("realm-admin", {item["name"] for item in grants})
                return None
            return [item for item in roles if item["name"] != "realm-admin"]
        original_loader = install.importlib.util.spec_from_file_location
        def loader(name, path):
            spec = original_loader(name, path)
            real_exec = spec.loader.exec_module
            def execute(module):
                real_exec(module)
                module.request = request
            spec.loader.exec_module = execute
            return spec
        with patch("installation.importlib.util.spec_from_file_location", side_effect=loader), patch("deployment.run"):
            install.configure_identity(REPO, values)
        self.assertFalse(any("/groups" in url and method != "GET" for url, method, _ in calls))

    def test_remote_failure_does_not_print_raw_provider_or_secret_output(self):
        with patch("installation.subprocess.run") as process:
            process.return_value.returncode = 1
            process.return_value.stderr = 'raw password=secret\n{"access_token":"private"}'
            with self.assertRaises(deployment.DeploymentError) as error:
                install.remote(OPTIONS, {"action": "check"})
            self.assertNotIn("secret", str(error.exception))
            self.assertNotIn("private", str(error.exception))

    def test_missing_cli_arguments_fail_without_remote_mutation(self):
        with patch("sys.argv", ["installation", "--domain", OPTIONS["domain"]]), patch("sys.stdin.isatty", return_value=False), patch("installation.remote") as remote, contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(install.main(), 1)
            remote.assert_not_called()

    def test_upload_records_original_revision_before_any_service_or_secret_is_created(self):
        with tempfile.TemporaryDirectory() as directory, patch("installation.preflight"):
            root = Path(directory).resolve()
            options = {**OPTIONS, "root": str(root)}
            payload = {"state": None, "revision": "b" * 40, "upload": ".install-upload-" + "a" * 32}
            first = install.prepare_upload(root, options, payload)
            self.assertEqual(first["revision"], payload["revision"])
            self.assertFalse((root / "config").exists())
            self.assertFalse(first["complete"])
            with self.assertRaises(deployment.DeploymentError):
                install.prepare_upload(root, options, payload)
            retried = install.prepare_upload(root, options, {**payload, "state": first, "upload": ".install-upload-" + "b" * 32})
            self.assertEqual(retried, first)

    def test_installation_starts_tls_identity_before_api_and_completes_only_after_health(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            options = {**OPTIONS, "root": str(root)}
            fixture = root / "fixture"
            fixture.mkdir()
            for name in ("deploy/platform/.env.example", "deploy/platform/config/platform.https.yaml",
                         "deploy/platform/config/keycloak-realm.json", "deploy/sandbox/agent-platform-egress.service"):
                target = fixture / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(REPO / name, target)
            payload = {"revision": "c" * 40, "upload": ".install-upload-" + "c" * 32,
                       "files": deployment.file_manifest(fixture), "web_files": {}}
            payload["state"] = {"revision": payload["revision"], "options": options, "complete": False}
            install.private_write(root / ".installation.json", json.dumps(payload["state"]))
            upload = root / payload["upload"]
            upload.mkdir()
            with tarfile.open(upload / "source.tar", "w") as archive:
                for path in sorted(fixture.rglob("*")):
                    archive.add(path, arcname=path.relative_to(fixture), recursive=False)
            web = root / "fixture-web"
            web.mkdir()
            (web / "index.html").write_text('<html><script src="/assets/app.js"></script></html>')
            (web / "assets").mkdir()
            (web / "assets/app.js").write_text("web")
            payload["web_files"] = deployment.file_manifest(web)
            with tarfile.open(upload / "web.tar", "w") as archive:
                for path in sorted(web.rglob("*")):
                    archive.add(path, arcname=path.relative_to(web), recursive=False)
            calls = []
            real_private_write = install.private_write
            def write(path, content):
                if str(path).startswith("/etc/"):
                    self.assertIn(str(root / "src"), content)
                    return
                real_private_write(path, content)
            def run(args, **kwargs):
                calls.append(args)
                if "build-identity-theme.py" in " ".join(args):
                    Path(args[-1]).mkdir(parents=True)
                if args[:2] == ["openssl", "genpkey"]:
                    return "test-private-key"
                return ""
            with patch("installation.preflight"), patch("installation.provision_host"), patch("installation.os.chown"), patch("installation.private_write", side_effect=write), patch("deployment.run", side_effect=run), patch(
                    "installation.subprocess.run") as process, patch("installation.configure_identity"), patch("deployment.verify_server") as verify, patch(
                    "deployment.migration_ledger", return_value={}), patch("deployment.verify_migrations"), patch("deployment.container_value", return_value="image"), contextlib.redirect_stdout(io.StringIO()):
                process.return_value.returncode = 0
                process.return_value.stdout = b"test-der"
                install.perform_install(root, options, payload)
            tls = next(i for i, args in enumerate(calls) if args[-4:] == ["up", "-d", "--no-deps", "caddy"])
            api = next(i for i, args in enumerate(calls) if args[-3:] == ["up", "-d", "api"])
            worker = next(i for i, args in enumerate(calls) if args[-4:] == ["up", "-d", "egress-controller", "worker"])
            self.assertLess(tls, api)
            self.assertLess(api, worker)
            verify.assert_called_once()
            self.assertTrue(json.loads((root / ".installation.json").read_text())["complete"])
            self.assertFalse(os.readlink(root / "web/current").startswith("/"))
            self.assertEqual((root / "identity-themes/current").resolve().stat().st_mode & 0o777, 0o755)
            with self.assertRaises(deployment.DeploymentError):
                install.existing_state(root, options)


if __name__ == "__main__":
    unittest.main()
