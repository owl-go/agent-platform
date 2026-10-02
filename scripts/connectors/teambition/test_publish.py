import importlib.util
import unittest
from pathlib import Path
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('teambition_publish', Path(__file__).with_name('publish.py'))
publisher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(publisher)


class PublicationLifecycleTest(unittest.TestCase):
    def test_old_package_revision_supplies_exact_conformance_without_new_definition(self):
        revisions = [{'revision': {'package_version': '0.3.3', 'bundle_sha256': 'bundle',
                                  'runtime_digests': ['digest'], 'conformance_available': True}}]
        self.assertTrue(publisher.has_verified_revision(revisions, 'bundle', 'digest'))
        self.assertFalse(publisher.has_verified_revision(revisions, 'other-bundle', 'digest'))
        self.assertFalse(publisher.has_verified_revision(revisions, 'bundle', 'other-runtime'))

    def run_cleanup(self, usage):
        calls = []
        definition = {'id': 'stage-id', 'name': 'Teambition package build 0.3.3 hash', 'version': 5,
                      'npm_package': '@agent-platform/teambition-connector', 'state': 'disabled'}
        deleted = False

        def fake_api(base, token, method, path, body=None):
            nonlocal deleted
            calls.append((method, path))
            if method == 'DELETE':
                deleted = True
                return {'deleted': True}
            if path.endswith('/cli-health'):
                return {'items': [{'definition_id': 'stage-id', **usage}]}
            return {'items': [] if deleted else [definition]}

        with patch.object(publisher, 'api', fake_api):
            publisher.cleanup_staging_definitions('base', 'token')
        return calls

    def test_disabled_build_definition_is_soft_deleted_instead_of_left_as_a_second_card(self):
        calls = self.run_cleanup({'enablement_count': 0})
        self.assertIn(('DELETE', '/api/v1/admin/connectors/cli/stage-id?expected_version=5'), calls)
        self.assertFalse(any(path.endswith('/disable') for _, path in calls))

    def test_cleanup_stops_before_revoking_any_existing_user_usage(self):
        with self.assertRaisesRegex(RuntimeError, 'user usage'):
            self.run_cleanup({'enablement_count': 1})

    def test_modao_cleanup_is_scoped_and_soft_deletes_only_its_build(self):
        calls = []
        items = [{'id': 'modao-stage', 'name': 'Modao package build 0.1.1 hash', 'version': 2, 'npm_package': '@agent-platform/modao-connector'},
                 {'id': 'tb-stage', 'name': 'Teambition package build 0.3.5 hash', 'version': 2, 'npm_package': '@agent-platform/teambition-connector'}]
        def fake_api(base, token, method, path, body=None):
            calls.append((method, path))
            if method == 'DELETE':
                items.pop(0)
                return {'deleted': True}
            if path.endswith('/cli-health'):
                return {'items': [{'definition_id': 'modao-stage', 'enablement_count': 0, 'active_authorization_count': 0}]}
            return {'items': items.copy()}
        with patch.object(publisher, 'api', fake_api):
            publisher.cleanup_staging_definitions('base', 'token', 'modao')
        self.assertEqual([path for method, path in calls if method == 'DELETE'], ['/api/v1/admin/connectors/cli/modao-stage?expected_version=2'])
        self.assertEqual(publisher.build_identity('picset-ai'), ('Picset AI package build ', '@agent-platform/picset-ai-connector'))
        with self.assertRaisesRegex(RuntimeError, 'unreviewed'):
            publisher.build_identity('other')


if __name__ == '__main__':
    unittest.main()
