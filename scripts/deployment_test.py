import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch
from types import SimpleNamespace

spec = importlib.util.spec_from_file_location("deployment", Path(__file__).with_name("deployment.py"))
deployment = importlib.util.module_from_spec(spec)
spec.loader.exec_module(deployment)


class DeploymentTests(unittest.TestCase):
    def test_profile_rejects_shell_options_traversal_and_secrets(self):
        for profile in ({"host": "-oProxyCommand=evil", "root": "/opt/platform"},
                        {"host": "user@host;touch /tmp/evil", "root": "/opt/platform"},
                        {"host": "host", "root": "/opt/../etc"},
                        {"host": "host", "root": "/"},
                        {"host": "host", "root": "/opt/platform", "password": "private"}):
            with self.subTest(profile=profile), self.assertRaises(deployment.DeploymentError):
                deployment.validate_profile(profile)

    def test_profile_is_private_json_and_replacement_is_atomic(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "settings/deploy.json"
            first = {"host": "user@host", "root": "/opt/platform"}
            deployment.save_profile(first, path)
            self.assertEqual(json.loads(path.read_text()), first)
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            with patch.object(deployment.os, "replace", side_effect=OSError("disk full")):
                with self.assertRaises(OSError):
                    deployment.save_profile({"host": "other", "root": "/opt/other"}, path)
            self.assertEqual(json.loads(path.read_text()), first)

    def test_deploy_plan_skips_docs_and_updates_only_affected_components(self):
        base = {"backend/main.go": "a", "frontend/app.vue": "b", "README.md": "c"}
        self.assertEqual(deployment.make_plan({**base, "README.md": "d"}, base),
                         {"backend": False, "web": False, "migration": False})
        self.assertEqual(deployment.make_plan({**base, "frontend/app.vue": "d"}, base),
                         {"backend": False, "web": True, "migration": False})
        self.assertEqual(deployment.make_plan({**base, "backend/main.go": "d"}, base),
                         {"backend": True, "web": False, "migration": False})

    def test_runtime_identity_and_existing_migration_changes_are_rejected(self):
        for path in ("deploy/runtimes/unified/Dockerfile", "deploy/platform/compose.yaml",
                     "deploy/platform/themes/agent-workspace/login/theme.properties",
                     deployment.MIGRATIONS + "/000001.sql"):
            with self.subTest(path=path), self.assertRaises(deployment.DeploymentError):
                deployment.make_plan({path: "new"}, {path: "old"})
        with self.assertRaises(deployment.DeploymentError):
            deployment.make_plan({}, {deployment.MIGRATIONS + "/000001.sql": "old"})
        self.assertTrue(deployment.make_plan({deployment.MIGRATIONS + "/000002.sql": "new"}, {})["migration"])

    def test_snapshot_uses_fetched_integration_not_dirty_worktree(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            origin, repo, source = root / "origin", root / "repo", root / "source"
            subprocess.run(["git", "init", "--bare", str(origin)], check=True, capture_output=True)
            subprocess.run(["git", "clone", str(origin), str(repo)], check=True, capture_output=True)
            for args in (["config", "user.email", "fixture@example.test"], ["config", "user.name", "Fixture"],
                         ["switch", "-c", "main_temp"]):
                deployment.run(["git", *args], cwd=repo)
            (repo / "application.txt").write_text("integrated")
            deployment.run(["git", "add", "application.txt"], cwd=repo)
            deployment.run(["git", "commit", "-m", "fixture"], cwd=repo)
            deployment.run(["git", "push", "origin", "main_temp"], cwd=repo)
            deployment.run(["git", "switch", "-c", "feature"], cwd=repo)
            (repo / "application.txt").write_text("uncommitted")
            (repo / ".env").write_text("SECRET=must-not-upload")
            revision = deployment.snapshot(repo, source)
            self.assertEqual(len(revision), 40)
            self.assertEqual((source / "application.txt").read_text(), "integrated")
            self.assertFalse((source / ".env").exists())
            self.assertEqual((repo / "application.txt").read_text(), "uncommitted")

    def test_manifest_rejects_symlink_and_private_env_but_prunes_dependencies(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "application").write_text("valid")
            (root / "node_modules").mkdir()
            (root / "node_modules/private.env").symlink_to("/etc/passwd")
            self.assertEqual(set(deployment.file_manifest(root)), {"application"})
            (root / ".env").write_text("SECRET=private")
            with self.assertRaises(deployment.DeploymentError):
                deployment.file_manifest(root)
            (root / ".env").unlink()
            (root / "link").symlink_to("/etc/passwd")
            with self.assertRaises(deployment.DeploymentError):
                deployment.file_manifest(root)

    def test_remote_payload_is_stdin_not_shell_argument_and_error_is_redacted(self):
        payload = {"action": "release", "files": {"file": "a" * 200000}}
        with patch.object(deployment.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, "{}", "")) as run:
            deployment.remote({"host": "server", "root": "/opt/platform"}, payload)
        args, kwargs = run.call_args
        self.assertLess(len(args[0][-1]), 1000)
        self.assertGreater(len(kwargs["input"]), 200000)
        with patch.object(deployment.subprocess, "run", return_value=subprocess.CompletedProcess([], 1, "secret", "secret\n部署停止：配置缺失\n")):
            with self.assertRaisesRegex(deployment.DeploymentError, "^配置缺失$"):
                deployment.remote({"host": "server", "root": "/opt/platform"}, payload)

    def test_public_config_rejects_embedded_secrets_and_http(self):
        good = {"VITE_OIDC_CLIENT_ID": "web", "VITE_OIDC_AUTHORITY": "https://host/identity/realms/platform",
                "VITE_OIDC_REDIRECT_URI": "https://host/auth/callback", "VITE_OIDC_POST_LOGOUT_REDIRECT_URI": "https://host"}
        deployment.validate_public(good)
        for url in ("http://host", "https://user:secret@host", "https://host?token=secret", "https://host/#token"):
            with self.subTest(url=url), self.assertRaises(deployment.DeploymentError):
                deployment.validate_public({**good, "VITE_OIDC_AUTHORITY": url})

    def test_backend_gates_do_not_install_or_rebuild_web(self):
        with patch.object(deployment, "run") as run:
            deployment.gates(Path("/source"), {"backend": True, "web": False}, {}, io.StringIO())
        self.assertEqual([c.args[0] for c in run.call_args_list],
                         [["make", "deploy-test"], ["make", "test"], ["make", "build"]])

    def test_build_space_blocks_before_gates_but_allows_noop_release(self):
        with patch.object(deployment.shutil, "disk_usage", return_value=SimpleNamespace(free=2 * 1024**3)):
            with self.assertRaisesRegex(deployment.DeploymentError, "构建空间不足 4 GiB"):
                deployment.require_build_space(Path("/source"), {"backend": True, "web": False})
            deployment.require_build_space(Path("/source"), {"backend": False, "web": False})

    def test_public_verification_rejects_stale_frontend_even_when_http_200(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            web = root / "web/current"
            web.mkdir(parents=True)
            (web / "index.html").write_text('<script src="/assets/index-current.js"></script>')
            (web / "assets").mkdir()
            (web / "assets/index-current.js").write_text("new-release")

            def curl(args):
                destination = args[args.index("-o") + 1]
                if destination != "/dev/null":
                    Path(destination).write_text((web / "index.html").read_text() if args[-1].endswith("/") else "old-release")

            env = {"VITE_OIDC_POST_LOGOUT_REDIRECT_URI": "https://host", "VITE_OIDC_AUTHORITY": "https://host/realms/test"}
            with patch.object(deployment, "container_value", return_value="healthy"), patch.object(deployment, "run", side_effect=curl):
                with self.assertRaisesRegex(deployment.DeploymentError, "公网 Web 内容"):
                    deployment.verify_server(root, env)

    def test_web_build_is_once_and_uses_remote_public_configuration(self):
        public = {"VITE_OIDC_CLIENT_ID": "remote-client"}
        with patch.object(deployment, "run") as run:
            deployment.gates(Path("/source"), {"backend": False, "web": True}, public, io.StringIO())
        self.assertEqual(sum(c.args[0] == ["make", "web-build"] for c in run.call_args_list), 1)
        self.assertEqual(run.call_args_list[-1].kwargs["env"]["VITE_OIDC_CLIENT_ID"], "remote-client")

    def test_corrupt_upload_aborts_before_any_configuration_or_services_change(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / "src.release-app-test"
            source.mkdir()
            (source / "application").write_text("tampered")
            state = {"source": "/previous", "files": {}, "backend_verified": True}
            with patch.object(deployment, "inspect_server", return_value=state), patch.object(deployment, "run") as run:
                with self.assertRaisesRegex(deployment.DeploymentError, "源码上传校验失败"):
                    deployment.perform_release(root, {"release": "app-test", "previous": "/previous", "files": {}})
            run.assert_not_called()
            self.assertFalse((root / "backups").exists())

    def test_remote_release_preserves_secrets_and_runtime_and_records_verified_images(self):
        self.exercise_remote_release("backend")

    def test_web_only_release_promotes_verified_assets_without_restarting_backend(self):
        self.exercise_remote_release("web")

    def exercise_remote_release(self, change):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            previous, candidate = root / "src.release-old", root / "src.release-app-test"
            for source, text in ((previous, "old"), (candidate, "new")):
                (source / deployment.MIGRATIONS).mkdir(parents=True)
                (source / deployment.MIGRATIONS / "000001.sql").write_text("immutable")
                (source / "backend/app.go").write_text(text if change == "backend" else "unchanged")
                (source / "frontend").mkdir()
                (source / "frontend/app.vue").write_text(text if change == "web" else "unchanged")
            (root / "src").symlink_to(previous)
            old_web = root / "web/releases/old"
            old_web.mkdir(parents=True)
            (root / "web/current").symlink_to(old_web)
            (root / "config").mkdir()
            for name in ("platform.env", "platform.https.yaml"):
                (root / "config" / name).write_text("PRIVATE_SECRET_UNCHANGED")
            commands = []

            def run(args, **kwargs):
                commands.append(args)
                return ""

            def dump(args, **kwargs):
                if "pg_dump" in args[-1]:
                    kwargs["stdout"].write(b"verified-database-fixture")
                return subprocess.CompletedProcess(args, 0)

            state = {"source": str(previous), "files": deployment.file_manifest(previous), "backend_verified": True}
            payload = {"release": "app-test", "previous": str(previous), "revision": "a" * 40,
                       "files": deployment.file_manifest(candidate), "web_files": {}}
            if change == "web":
                (candidate / ".web/assets").mkdir(parents=True)
                (candidate / ".web/index.html").write_text("release entry")
                (candidate / ".web/assets/app.js").write_text("release asset")
                payload["web_files"] = deployment.file_manifest(candidate / ".web")
            with patch.object(deployment, "inspect_server", return_value=state), \
                    patch.object(deployment, "load_remote_env", return_value={}), \
                    patch.object(deployment, "container_value", return_value="sha256:" + "b" * 64), \
                    patch.object(deployment, "database", return_value="000001.sql"), \
                    patch.object(deployment, "active_work", return_value=False), \
                    patch.object(deployment, "wait_healthy"), \
                    patch.object(deployment, "verify_server"), \
                    patch.object(deployment, "run", side_effect=run), \
                    patch.object(deployment.subprocess, "run", side_effect=dump):
                deployment.perform_release(root, payload)
            self.assertEqual((root / "src").resolve(), candidate.resolve())
            expected_web = root / "web/releases/app-test" if change == "web" else old_web
            self.assertEqual((root / "web/current").resolve(), expected_web.resolve())
            if change == "web":
                self.assertEqual((expected_web / "assets/app.js").read_text(), "release asset")
                self.assertEqual((expected_web / "assets/app.js").stat().st_mode & 0o777, 0o644)
            record = json.loads((candidate / ".deployment.json").read_text())
            self.assertEqual(record["backend_hash"], deployment.group_hashes(payload["files"])["backend"])
            self.assertEqual(set(record["images"]), {"api", "worker"})
            for name in ("platform.env", "platform.https.yaml"):
                self.assertEqual((root / "config" / name).read_text(), "PRIVATE_SECRET_UNCHANGED")
                self.assertEqual((root / "backups/pre-app-test" / name).stat().st_mode & 0o777, 0o600)
            self.assertTrue((root / "backups/pre-app-test/business.pgdump").exists())
            self.assertTrue((root / "backups/pre-app-test/identity.pgdump").exists())
            self.assertEqual(any(c[-3:] == ["build", "api", "worker"] for c in commands), change == "backend")
            if change == "web":
                self.assertFalse(any("up" in c or "stop" in c for c in commands))
            self.assertFalse(any("push" in c or "volume" in c or "identity" in c or "egress-controller" in c for c in commands))


class CutoverTests(unittest.TestCase):
    def execute(self, *, busy=(False, False), migration=False, failure=False):
        calls = []

        def compose(args):
            calls.append(tuple(args))

        def verify():
            calls.append(("verify",))
            if failure:
                raise deployment.DeploymentError("health failed")

        with patch.object(deployment, "active_work", side_effect=busy), patch.object(deployment, "wait_healthy"):
            deployment.cutover(compose, lambda: calls.append(("rollback",)), {"backend": True}, verify,
                               lambda: migration, lambda: calls.append(("promote",)))
        return calls

    def test_active_work_blocks_before_stopping_services(self):
        with patch.object(deployment, "active_work", return_value=True), patch.object(deployment, "wait_healthy"):
            commands = []
            with self.assertRaisesRegex(deployment.DeploymentError, "还有任务"):
                deployment.cutover(commands.append, lambda: commands.append("rollback"), {"backend": True},
                                   lambda: None, lambda: False, lambda: None)
            self.assertEqual(commands, [])

    def test_new_work_race_resumes_old_services_without_starting_new_binary(self):
        calls = []
        with patch.object(deployment, "active_work", side_effect=[False, True]):
            with self.assertRaisesRegex(deployment.DeploymentError, "有新任务"):
                deployment.cutover(calls.append, lambda: calls.append("rollback"), {"backend": True},
                                   lambda: None, lambda: False, lambda: None)
        self.assertEqual(calls, [["stop", "api", "worker"], "rollback"])

    def test_success_verifies_before_and_after_pointer_promotion(self):
        calls = self.execute()
        self.assertEqual(calls[-3:], [("verify",), ("promote",), ("verify",)])
        self.assertNotIn(("rollback",), calls)

    def test_failure_rolls_back_when_ledger_unchanged(self):
        calls = []
        with patch.object(deployment, "active_work", return_value=False), patch.object(deployment, "wait_healthy"):
            with self.assertRaisesRegex(deployment.DeploymentError, "health failed"):
                deployment.cutover(calls.append, lambda: calls.append("rollback"), {"backend": True},
                                   lambda: (_ for _ in ()).throw(deployment.DeploymentError("health failed")),
                                   lambda: False, lambda: None)
        self.assertEqual(calls[-1], "rollback")

    def test_new_or_unknown_schema_never_starts_old_binary_and_stops_worker(self):
        for changed in (lambda: True, lambda: (_ for _ in ()).throw(OSError("database unavailable"))):
            calls = []
            with patch.object(deployment, "active_work", return_value=False), patch.object(deployment, "wait_healthy"):
                with self.assertRaisesRegex(deployment.DeploymentError, "Worker 已停止"):
                    deployment.cutover(calls.append, lambda: calls.append("rollback"), {"backend": True},
                                       lambda: (_ for _ in ()).throw(deployment.DeploymentError("health failed")),
                                       changed, lambda: None)
            self.assertEqual(calls[-1], ["stop", "worker"])
            self.assertNotIn("rollback", calls)


if __name__ == "__main__":
    unittest.main()
