"""Release elapsed-time boundaries and API issue behavior, without network mutations."""
import contextlib
import io
import json
from pathlib import Path
import tempfile
import subprocess
import unittest
from unittest.mock import patch

import release_timing as timing

SHA = 'a' * 40
TAG_SHA = 'b' * 40
START = '2026-10-14T04:00:00Z'
NOW = '2026-10-14T04:12:00Z'
POLICY = {'goal_seconds': 300, 'maximum_seconds': 600}
RUN = {'id': 1, 'head_sha': SHA, 'head_branch': 'main', 'event': 'push',
       'created_at': START, 'status': 'in_progress', 'conclusion': None,
       'html_url': 'https://github.com/example/project/actions/runs/1'}
RELEASE_RUN = {'id': 2, 'head_sha': TAG_SHA, 'head_branch': 'v1.2.3',
               'created_at': '2026-10-14T04:03:00Z',
               'html_url': 'https://github.com/example/project/actions/runs/2'}


def job(name, start, end=None, conclusion=None, steps=None):
    return {'name': name, 'started_at': start, 'completed_at': end,
            'status': 'completed' if end else 'in_progress',
            'conclusion': conclusion, 'steps': steps or [], 'run_attempt': 1}


class FakeGH:
    """Fake gh executable boundary: validates API requests and JSON issue payloads."""
    def __init__(self):
        self.data = {
            'repos/example/project/commits/' + SHA: {
                'commit': {'message': 'Application change', 'author': {'name': 'developer'}}},
            'repos/example/project/commits/' + TAG_SHA: {
                'commit': {'message': 'chore: release\n\nRelease-Source: ' + SHA,
                           'author': {'name': 'github-actions[bot]'}}},
            'repos/example/project/actions/runs/1/jobs?filter=all&per_page=100': {'jobs': []},
            'repos/example/project/actions/runs/2/jobs?filter=all&per_page=100': {'jobs': []},
        }
        self.issues = []
        self.created = []

    def call(self, args, **kwargs):
        assert args[:2] == ['gh', 'api']
        if '--method' in args:
            assert args[args.index('--method') + 1] == 'POST'
            assert args[args.index('--method') + 2] == 'repos/example/project/issues'
            payload = json.loads(kwargs['input'])
            issue = {**payload, 'html_url': 'https://github.com/example/project/issues/42'}
            self.created.append(issue)
            self.issues.append(issue)
            return json.dumps(issue)
        if '--paginate' in args:
            return json.dumps([self.issues])
        return json.dumps(self.data[args[-1]])


class ReleaseTimingTest(unittest.TestCase):
    def setUp(self):
        self.fake = FakeGH()
        self.patch = patch('release_timing.subprocess.check_output', side_effect=self.fake.call)
        self.patch.start()
        self.addCleanup(self.patch.stop)
        self.api = timing.GitHub()

    def measure(self, now=NOW, runs=None):
        return timing.measure(self.api, 'example/project', RUN, runs or [], POLICY, now)

    def test_limit_boundary_is_strict(self):
        result = self.measure('2026-10-14T04:10:00Z')
        self.assertFalse(result['violation'])
        self.assertIsNone(timing.ensure_issue(self.api, 'example/project', result))
        self.assertFalse(self.fake.created)
        self.assertTrue(self.measure('2026-10-14T04:10:01Z')['violation'])

    def test_queued_or_failed_candidate_still_counts_wall_time(self):
        run = {**RUN, 'status': 'completed', 'conclusion': 'failure'}
        result = timing.measure(self.api, 'example/project', run, [], POLICY, NOW)
        self.assertEqual(result['elapsed_seconds'], 720)
        self.assertTrue(result['violation'])
        self.assertEqual(result['delivery'], 'not published')

    def test_create_then_dedupe_even_closed_issue(self):
        result = self.measure()
        first = timing.ensure_issue(self.api, 'example/project', result)
        self.fake.issues[0]['state'] = 'closed'
        second = timing.ensure_issue(self.api, 'example/project', result)
        self.assertTrue(first['created'])
        self.assertFalse(second['created'])
        self.assertEqual(len(self.fake.created), 1)
        self.assertIn(SHA, self.fake.created[0]['body'])
        self.assertIn('720s', self.fake.created[0]['body'])
        self.assertIn('600s maximum', self.fake.created[0]['body'])
        self.assertIn(RUN['html_url'], self.fake.created[0]['body'])

    def test_issue_key_reuses_incident_across_retry_run_ids(self):
        result = self.measure()
        timing.ensure_issue(self.api, 'example/project', result)
        result['run_id'] = 99
        timing.ensure_issue(self.api, 'example/project', result)
        self.assertEqual(len(self.fake.created), 1)

    def test_publication_stops_clock_at_feed_and_tap(self):
        self.fake.data['repos/example/project/actions/runs/2/jobs?filter=all&per_page=100']['jobs'] = [
            job('Publish GitHub Release and Homebrew Casks', '2026-10-14T04:08:00Z',
                '2026-10-14T04:09:00Z', 'success', [
                    {'name': 'Update Homebrew CLI and app Casks', 'conclusion': 'success',
                     'completed_at': '2026-10-14T04:08:50Z'}])]
        self.fake.data['repos/example/project/releases/tags/v1.2.3'] = {
            'draft': False, 'published_at': '2026-10-14T04:08:20Z',
            'assets': [{'name': 'appcast.xml', 'state': 'uploaded', 'updated_at': '2026-10-14T04:08:21Z'}]}
        result = self.measure(runs=[RELEASE_RUN])
        self.assertEqual(result['elapsed_seconds'], 530)
        self.assertFalse(result['violation'])
        self.assertEqual(result['delivery'], 'published feed and tap')

    def test_parallel_platforms_are_not_summed(self):
        self.fake.data['repos/example/project/actions/runs/2/jobs?filter=all&per_page=100']['jobs'] = [
            job('CLI (linux_amd64)', '2026-10-14T04:03:00Z', '2026-10-14T04:06:00Z', 'success'),
            job('CLI (linux_arm64)', '2026-10-14T04:03:00Z', '2026-10-14T04:07:00Z', 'success'),
            job('Signed and notarized macOS release assets', '2026-10-14T04:03:00Z',
                '2026-10-14T04:08:00Z', 'success')]
        result = self.measure(runs=[RELEASE_RUN])
        self.assertEqual(result['parallel_platform_critical_path_seconds'], 300)
        self.assertEqual(result['elapsed_seconds'], 720)

    def test_qualification_overlaps_platform_work_without_double_counting(self):
        self.fake.data['repos/example/project/actions/runs/1/jobs?filter=all&per_page=100']['jobs'] = [
            job('Check release source', START, '2026-10-14T04:00:30Z', 'success')]
        self.fake.data['repos/example/project/actions/runs/2/jobs?filter=all&per_page=100']['jobs'] = [
            job('Prepare shared release inputs', '2026-10-14T04:01:00Z', '2026-10-14T04:02:00Z', 'success'),
            job('CLI (linux_amd64)', '2026-10-14T04:02:00Z', '2026-10-14T04:06:00Z', 'success'),
            job('Signed and notarized macOS release assets', '2026-10-14T04:02:00Z',
                '2026-10-14T04:05:00Z', 'success')]
        result = self.measure(runs=[RELEASE_RUN])
        self.assertEqual(result['active_critical_path_seconds'], 330)
        self.assertEqual(result['elapsed_seconds'], 720)

    def test_retry_stages_preserved_and_original_start_unchanged(self):
        first = job('Check release source', START, '2026-10-14T04:02:00Z', 'failure')
        second = job('Check release source', '2026-10-14T04:05:00Z',
                     '2026-10-14T04:06:00Z', 'success')
        second['run_attempt'] = 2
        self.fake.data['repos/example/project/actions/runs/1/jobs?filter=all&per_page=100']['jobs'] = [first, second]
        result = self.measure()
        self.assertEqual([stage['attempt'] for stage in result['stages']], [1, 2])
        self.assertEqual(result['elapsed_seconds'], 720)

    def test_documentation_skip_vs_failed_source_check(self):
        jobs = self.fake.data['repos/example/project/actions/runs/1/jobs?filter=all&per_page=100']['jobs']
        jobs.extend([job('Check release source', START, START, 'success'),
                     job('Tag and publish checked source', START, START, 'skipped')])
        run = {**RUN, 'status': 'completed', 'conclusion': 'success'}
        result = timing.measure(self.api, 'example/project', run, [], POLICY, NOW)
        self.assertEqual(result['excluded'], 'deliberate no-release change')
        jobs[0]['conclusion'] = 'failure'
        self.api = timing.GitHub()
        result = timing.measure(self.api, 'example/project', run, [], POLICY, NOW)
        self.assertTrue(result['violation'])

    def test_metadata_commit_is_excluded(self):
        self.fake.data['repos/example/project/commits/' + SHA]['commit'] = {
            'message': 'chore: release\n\nRelease-Source: ' + 'c' * 40,
            'author': {'name': 'github-actions[bot]'}}
        self.assertEqual(self.measure()['excluded'], 'release metadata commit')

    def test_missing_publication_timestamp_is_an_error(self):
        jobs = self.fake.data['repos/example/project/actions/runs/2/jobs?filter=all&per_page=100']['jobs']
        jobs.append(job('Publish', START, NOW, 'success', [
            {'name': 'Update Homebrew CLI and app Casks', 'conclusion': 'success', 'completed_at': None}]))
        self.fake.data['repos/example/project/releases/tags/v1.2.3'] = {
            'draft': False, 'published_at': START,
            'assets': [{'name': 'appcast.xml', 'state': 'uploaded', 'updated_at': START}]}
        with self.assertRaisesRegex(ValueError, 'timestamp is missing'):
            self.measure(runs=[RELEASE_RUN])

    def test_api_failure_cannot_be_green(self):
        with patch('release_timing.subprocess.check_output', side_effect=subprocess.CalledProcessError(1, 'gh')):
            with self.assertRaises(subprocess.CalledProcessError):
                self.measure()

    def test_separate_retry_run_keeps_first_main_clock(self):
        retry = {**RUN, 'id': 3, 'event': 'workflow_dispatch',
                 'created_at': '2026-10-14T04:07:00Z'}
        self.fake.data['repos/example/project/actions/runs/3/jobs?filter=all&per_page=100'] = {'jobs': []}
        result = timing.measure(self.api, 'example/project', retry, [], POLICY, NOW, [RUN, retry])
        self.assertEqual(result['start'], START)
        self.assertEqual(result['elapsed_seconds'], 720)

    def test_skipped_and_queued_jobs_do_not_require_active_timestamps(self):
        jobs = self.fake.data['repos/example/project/actions/runs/1/jobs?filter=all&per_page=100']['jobs']
        jobs.extend([{'name': 'Release', 'status': 'completed', 'conclusion': 'skipped'},
                     {'name': 'Queued', 'status': 'queued', 'started_at': None}])
        result = self.measure()
        self.assertEqual(result['stages'], [])
        self.assertEqual(result['preparation_initial_queue_seconds'], 720)

    def test_superseded_preparation_is_excluded(self):
        jobs = self.fake.data['repos/example/project/actions/runs/1/jobs?filter=all&per_page=100']['jobs']
        jobs.append(job('Tag and publish checked source', START, NOW, 'success', [
            {'name': 'Dispatch tagged build and wait for publication', 'conclusion': 'skipped'}]))
        self.assertEqual(self.measure()['excluded'], 'superseded preparation')

    def test_hook_boundary_and_incomplete_checks_create_advisory_issue(self):
        policy = {'commit_hook_maximum_seconds': 3, 'commit_hook_goal_seconds': 1}
        report = {'sha': SHA, 'elapsed_seconds': 3, 'budget_exceeded': False,
                  'incomplete_checks': [], 'checks': ['format', 'Go affected tests']}
        self.assertFalse(timing.check_hook_report(self.api, 'example/project', report, policy)['violation'])
        report['elapsed_seconds'] = 3.001
        result = timing.check_hook_report(self.api, 'example/project', report, policy)
        self.assertTrue(result['issue']['created'])
        report['elapsed_seconds'] = 2.99
        report['incomplete_checks'] = ['Go affected tests']
        result = timing.check_hook_report(self.api, 'example/project', report, policy)
        self.assertFalse(result['issue']['created'])
        self.assertEqual(len(self.fake.created), 1)

    def test_hook_timing_violation_cli_exit_is_zero(self):
        with tempfile.TemporaryDirectory() as directory:
            report = Path(directory) / 'hook.json'
            report.write_text(json.dumps({'sha': SHA, 'elapsed_seconds': 4,
                                         'budget_exceeded': True, 'incomplete_checks': []}))
            policy = Path(directory) / 'policy.json'
            policy.write_text(json.dumps({**POLICY, 'commit_hook_maximum_seconds': 3,
                                          'commit_hook_goal_seconds': 1}))
            with contextlib.redirect_stdout(io.StringIO()):
                result = timing.main(['--repo', 'example/project', '--hook-report', str(report),
                                      '--policy', str(policy)])
            self.assertEqual(result, 0)
            self.assertEqual(len(self.fake.created), 1)

    def test_release_timing_violation_cli_exit_is_zero(self):
        self.fake.data['repos/example/project/actions/workflows/auto-release.yml/runs?per_page=100'] = {
            'workflow_runs': [RUN]}
        self.fake.data['repos/example/project/actions/workflows/release.yml/runs?per_page=100'] = {
            'workflow_runs': []}
        self.fake.data['repos/example/project/actions/runs/1'] = RUN
        with tempfile.TemporaryDirectory() as directory:
            policy = Path(directory) / 'policy.json'
            policy.write_text(json.dumps({**POLICY, 'enforce_after': START}))
            with contextlib.redirect_stdout(io.StringIO()):
                result = timing.main(['--repo', 'example/project', '--run-id', '1',
                                      '--policy', str(policy), '--now', NOW])
            self.assertEqual(result, 0)
            self.assertEqual(len(self.fake.created), 1)

    def test_missing_start_or_reversed_timestamps_fail(self):
        with self.assertRaises(ValueError):
            timing.seconds(None, NOW)
        with self.assertRaises(ValueError):
            timing.seconds(NOW, START)


if __name__ == '__main__':
    unittest.main()
