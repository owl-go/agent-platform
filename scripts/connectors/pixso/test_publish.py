import contextlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from urllib import error
import zipfile

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("pixso_publication", ROOT / "publish.py")
publication = importlib.util.module_from_spec(spec)
spec.loader.exec_module(publication)

class Response(io.BytesIO):
    def __init__(self, value):
        super().__init__(json.dumps(value).encode())

class PublicationTest(unittest.TestCase):
    def run_publication(self, existing=False, wrong_hash=False, callback_status=400, registration_matches=True):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            package = root / "pixso.zip"
            with zipfile.ZipFile(package, "w") as archive:
                for name in ["connector-meta.json", "mcp.json"]:
                    archive.writestr(name, (ROOT / name).read_bytes())
            revision = {"id":"revision", "source":"pixso", "package_version":"1.0.0", "mode":"mcp", "sha256":"b" * 64 if wrong_hash else "a" * 64, "icon":"data:image/png;base64,brand"}
            published = {"source":"pixso", "active_revision_id":"revision", "state":"available", "version":1}
            calls = []
            def api(base, token, method, path, body=None):
                calls.append((method,path,body))
                if path == "/api/v1/admin/connectors/publications":
                    return {"items":[{"revision":revision,"publication":published}] if existing or any("/publish" in p for _,p,_ in calls) else [{"revision":{"source":"other"},"publication":None}]}
                if path == "/api/v1/admin/connectors/packages": return revision
                if path.endswith("/publish"): return published
                if path == "/api/v1/connectors/catalog": return {"items":[{"source":"pixso","active_revision_id":"revision","revision":revision}]}
                raise AssertionError(path)
            class Opener:
                def open(self, req, timeout):
                    if isinstance(req,str):
                        raise error.HTTPError(req,callback_status,"",{"Cache-Control":"no-store","Referrer-Policy":"no-referrer"},None)
                    body = json.loads(req.data)
                    self_outer.assertEqual(req.full_url,"https://pixso.net/api/user/pixso/oauth2/register")
                    self_outer.assertEqual(body["token_endpoint_auth_method"],"none")
                    return Response({"client_id":"public-client","redirect_uris":body["redirect_uris"] if registration_matches else ["https://attacker.test"],"token_endpoint_auth_method":"none"})
            self_outer = self
            argv = ["publish.py","--config",str(root / "env"),"--package",str(package),"--normalized-sha256","a"*64,"--evidence-directory",str(root / "evidence")]
            with patch("sys.argv",argv), patch.object(publication.platform,"read_config",return_value={"VITE_OIDC_AUTHORITY":"https://workspace.test/identity/realms/platform"}), patch.object(publication.platform,"administrator_token",return_value="private-canary"), patch.object(publication.platform,"api",side_effect=api), patch.object(publication.request,"build_opener",return_value=Opener()), contextlib.redirect_stdout(io.StringIO()) as output:
                publication.main()
            self.assertNotIn("private-canary",output.getvalue())
            return calls

    def test_publish_checks_callback_registration_exact_revision_and_catalog(self):
        calls = self.run_publication()
        self.assertEqual(sum(method == "POST" for method,_,_ in calls),2)

    def test_repeated_publication_reuses_revision_without_mutations(self):
        self.assertFalse(any(method == "POST" for method,_,_ in self.run_publication(existing=True)))

    def test_staged_hash_mismatch_never_publishes(self):
        with self.assertRaisesRegex(RuntimeError,"staged revision differs"):
            self.run_publication(wrong_hash=True)

    def test_old_callback_requires_deployment(self):
        with self.assertRaisesRegex(RuntimeError,"not deployed"):
            self.run_publication(callback_status=401)

    def test_registration_redirect_mismatch_stops_activation(self):
        with self.assertRaisesRegex(RuntimeError,"registration mismatch"):
            self.run_publication(registration_matches=False)

if __name__ == "__main__":
    unittest.main()
