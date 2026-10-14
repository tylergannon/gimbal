#!/usr/bin/env python3
"""Measure main-to-publication wall time and open one issue per slow source SHA."""
import argparse
import datetime as dt
import json
import os
from pathlib import Path
import re
import subprocess
import sys


class GitHub:
    def __init__(self):
        self.cache = {}

    def get(self, endpoint):
        if endpoint not in self.cache:
            self.cache[endpoint] = json.loads(subprocess.check_output(
                ['gh', 'api', endpoint], text=True))
        return self.cache[endpoint]

    def collection(self, endpoint, key, oldest=None):
        items = []
        page = 1
        while True:
            values = self.get(endpoint + (f'&page={page}' if page > 1 else ''))[key]
            items.extend(values)
            if len(values) < 100 or (oldest and values and
                                     timestamp(values[-1]['created_at']) < timestamp(oldest)):
                return items
            page += 1

    def pages(self, endpoint):
        pages = json.loads(subprocess.check_output(
            ['gh', 'api', '--paginate', '--slurp', endpoint], text=True))
        return [item for page in pages for item in page]

    def create_issue(self, repo, title, body):
        return json.loads(subprocess.check_output(
            ['gh', 'api', '--method', 'POST', f'repos/{repo}/issues',
             '--input', '-'], input=json.dumps({'title': title, 'body': body}), text=True))


def timestamp(value):
    if not value:
        raise ValueError('Required timestamp is missing')
    value = dt.datetime.fromisoformat(value.replace('Z', '+00:00'))
    if value.tzinfo is None:
        raise ValueError('Timestamp must include its timezone')
    return value


def seconds(start, end):
    value = (timestamp(end) - timestamp(start)).total_seconds()
    if value < 0:
        raise ValueError(f'Timestamp order is invalid: {start} -> {end}')
    return value


def source_marker(commit):
    match = re.search(r'^Release-Source: ([0-9a-f]{40})$', commit['commit']['message'], re.M)
    return match.group(1) if match else None


def stages(jobs, now):
    result = []
    for job in jobs:
        if job['status'] in ('queued', 'waiting', 'pending', 'requested') or job.get('conclusion') == 'skipped':
            continue
        start = job.get('started_at')
        if not start:
            raise ValueError(f"Missing start timestamp for job {job['name']}")
        finish = job.get('completed_at') or now
        result.append({'name': job['name'], 'attempt': job.get('run_attempt', 1),
                       'start': start, 'end': finish, 'seconds': seconds(start, finish),
                       'status': job['status'], 'conclusion': job.get('conclusion'),
                       'release_run_id': job.get('release_run_id')})
    return result


def measure(api, repo, run, releases, policy, now, preparation_runs=None):
    if run['head_branch'] != 'main' or run['event'] not in ('push', 'workflow_dispatch'):
        return {'run_id': run['id'], 'excluded': 'not a main release preparation'}
    base = f'repos/{repo}'
    commit = api.get(f"{base}/commits/{run['head_sha']}")
    if source_marker(commit) and commit['commit']['author']['name'] == 'github-actions[bot]':
        return {'run_id': run['id'], 'excluded': 'release metadata commit'}
    preparations = preparation_runs or [run]
    latest_run = max(preparations, key=lambda value: value['created_at'])
    # A cancelled preparation was coalesced; its replacement owns the delivery deadline.
    if latest_run.get('conclusion') == 'cancelled':
        return {'run_id': run['id'], 'excluded': 'coalesced or explicitly cancelled preparation'}
    start = min(preparation['created_at'] for preparation in preparations)
    timestamp(start)
    source_jobs = []
    for preparation_run in preparations:
        source_jobs.extend({**job, 'preparation_run_id': preparation_run['id']} for job in api.collection(
            f"{base}/actions/runs/{preparation_run['id']}/jobs?filter=all&per_page=100", 'jobs'))
    newest_jobs = [job for job in source_jobs if job['preparation_run_id'] == latest_run['id']]
    latest_attempt = max([job.get('run_attempt', 1) for job in newest_jobs], default=1)
    latest_jobs = [job for job in newest_jobs if job.get('run_attempt', 1) == latest_attempt]
    preparation = next((job for job in latest_jobs if job['name'] == 'Tag and publish checked source'), None)
    failed_checks = any(job.get('conclusion') in ('failure', 'timed_out', 'cancelled', 'action_required')
                        for job in latest_jobs)
    if preparation and preparation.get('conclusion') == 'skipped' and not failed_checks \
            and latest_run['status'] == 'completed':
        return {'run_id': run['id'], 'excluded': 'deliberate no-release change'}
    if preparation and preparation.get('conclusion') == 'success':
        dispatch = next((step for step in preparation.get('steps', [])
                         if step['name'] == 'Dispatch tagged build and wait for publication'), None)
        if dispatch and dispatch.get('conclusion') == 'skipped':
            return {'run_id': run['id'], 'excluded': 'superseded preparation'}
    matches = []
    for release in releases:
        if timestamp(release['created_at']) < timestamp(start):
            continue
        tagged_commit = api.get(f"{base}/commits/{release['head_sha']}")
        if source_marker(tagged_commit) == run['head_sha']:
            matches.append(release)
    matches.sort(key=lambda value: value['id'])
    release_jobs = []
    for release in matches:
        release_jobs.extend({**job, 'release_run_id': release['id']} for job in api.collection(
            f"{base}/actions/runs/{release['id']}/jobs?filter=all&per_page=100", 'jobs'))
    tap_ends = [step['completed_at'] for job in release_jobs for step in job.get('steps', [])
                if step['name'] == 'Update Homebrew CLI and app Casks' and step['conclusion'] == 'success']
    end = None
    tag = matches[-1]['head_branch'] if matches else None
    if tap_ends:
        release = api.get(f'{base}/releases/tags/{tag}')
        if release['draft']:
            raise ValueError('Homebrew gate succeeded but release is still a draft')
        appcasts = [asset for asset in release['assets'] if asset['name'] == 'appcast.xml'
                    and asset['state'] == 'uploaded']
        if len(appcasts) != 1:
            raise ValueError('Published release has no unique uploaded appcast.xml')
        end = max([min(tap_ends, key=timestamp), release['published_at'], appcasts[0]['updated_at']],
                  key=timestamp)
    elapsed = seconds(start, end or now)
    all_stages = stages(source_jobs, now) + stages(release_jobs, now)
    platform_stages = [stage for stage in all_stages if stage['name'].startswith('CLI (')
                       or stage['name'] == 'Signed and notarized macOS release assets']
    platform_attempts = {}
    for stage in platform_stages:
        key = (stage['release_run_id'], stage['attempt'])
        platform_attempts[key] = max(platform_attempts.get(key, 0), stage['seconds'])
    # Count active wall intervals once: qualification now overlaps packaging.
    # The parent dispatch waiter itself is not additional execution time.
    intervals = [(timestamp(stage['start']), timestamp(stage['end']))
                 for stage in all_stages if stage['name'] != 'Tag and publish checked source']
    for job in source_jobs:
        if job['name'] != 'Tag and publish checked source' or job.get('conclusion') == 'skipped':
            continue
        dispatch = next((step for step in job.get('steps', [])
                         if step['name'] == 'Dispatch tagged build and wait for publication'), None)
        prefix_end = (dispatch or {}).get('started_at') or job.get('completed_at') or now
        if job.get('started_at'):
            intervals.append((timestamp(job['started_at']), timestamp(prefix_end)))
    active_critical_path = 0
    last_end = None
    for begin, finish in sorted(intervals):
        if last_end is None or begin > last_end:
            active_critical_path += (finish - begin).total_seconds()
        elif finish > last_end:
            active_critical_path += (finish - last_end).total_seconds()
        last_end = max(last_end, finish) if last_end else finish
    started = [job['started_at'] for job in source_jobs if job.get('started_at')
               and job['status'] not in ('queued', 'waiting', 'pending', 'requested')]
    return {'run_id': run['id'], 'source_sha': run['head_sha'], 'source_url': run['html_url'],
            'start': start, 'end': end, 'observed_at': now, 'elapsed_seconds': elapsed,
            'goal_seconds': policy['goal_seconds'], 'maximum_seconds': policy['maximum_seconds'],
            'over_goal': elapsed > policy['goal_seconds'], 'violation': elapsed > policy['maximum_seconds'],
            'delivery': 'published feed and tap' if end else 'not published',
            'preparation_conclusion': latest_run.get('conclusion'), 'tag': tag,
            'release_urls': [release['html_url'] for release in matches],
            'preparation_initial_queue_seconds': seconds(start, min(started, key=timestamp)) if started else elapsed,
            'active_critical_path_seconds': active_critical_path,
            'parallel_platform_critical_path_seconds': sum(platform_attempts.values()) if platform_attempts else None,
            'stages': all_stages}


def ensure_issue(api, repo, result):
    if not result.get('violation'):
        return None
    marker = f"<!-- release-timing:{result['source_sha']} -->"
    for issue in api.pages(f'repos/{repo}/issues?state=all&sort=created&direction=desc&per_page=100'):
        if 'pull_request' not in issue and marker in (issue.get('body') or ''):
            return {'url': issue['html_url'], 'created': False}
    rows = ['| Stage / attempt | Seconds | Result |', '| --- | ---: | --- |']
    for stage in result['stages']:
        rows.append(f"| {stage['name']} / {stage['attempt']} | {stage['seconds']:.0f} | "
                    f"{stage['conclusion'] or stage['status']} |")
    body = '\n'.join([marker,
        f"Main-to-publication time exceeded the {result['maximum_seconds']:.0f}s maximum "
        f"({result['goal_seconds']:.0f}s goal).",
        f"Source: `{result['source_sha']}`. Preparation: {result['source_url']}.",
        f"Start: {result['start']}. End: {result['end'] or 'still pending at ' + result['observed_at']}.",
        f"Elapsed: **{result['elapsed_seconds']:.0f}s**. Delivery: {result['delivery']}.",
        *result['release_urls'], '',
        'Wall time includes queueing and retries. Parent tagging waits overlap the child pipeline; '
        'do not sum the stage table. Active critical-path time counts parallel platform maximums.',
        f"Active critical path: {result['active_critical_path_seconds']:.0f}s.",
        *rows])
    issue = api.create_issue(repo, f"Release timing exceeded: source {result['source_sha'][:12]}", body)
    return {'url': issue['html_url'], 'created': True}


def check_hook_report(api, repo, report, policy):
    elapsed = report['elapsed_seconds']
    if not isinstance(elapsed, (int, float)) or isinstance(elapsed, bool) or elapsed < 0:
        raise ValueError('Hook elapsed_seconds must be a nonnegative number')
    sha = report['sha']
    if not re.fullmatch(r'[0-9a-f]{40}', sha):
        raise ValueError('Hook report must identify its full source SHA')
    maximum = policy['commit_hook_maximum_seconds']
    incomplete = report['incomplete_checks']
    result = {'source_sha': sha, 'elapsed_seconds': elapsed, 'maximum_seconds': maximum,
              'goal_seconds': policy['commit_hook_goal_seconds'], 'incomplete_checks': incomplete,
              'violation': elapsed > maximum or report['budget_exceeded'] or bool(incomplete)}
    if not result['violation']:
        return result
    marker = f'<!-- hook-timing:{sha} -->'
    for issue in api.pages(f'repos/{repo}/issues?state=all&sort=created&direction=desc&per_page=100'):
        if 'pull_request' not in issue and marker in (issue.get('body') or ''):
            result['issue'] = {'url': issue['html_url'], 'created': False}
            return result
    run_url = report.get('run_url') or report.get('proof_url')
    if not run_url and os.getenv('GITHUB_RUN_ID'):
        run_url = f'https://github.com/{repo}/actions/runs/{os.environ["GITHUB_RUN_ID"]}'
    body = '\n'.join([marker,
        f'Commit-hook timing or completion exceeded the {maximum}s budget '
        f'({result["goal_seconds"]}s goal). Performance findings do not block shipping.',
        f'Source: [{sha}](https://github.com/{repo}/commit/{sha}).',
        f'Measured elapsed: **{elapsed:.3f}s**. Budget exceeded: {report["budget_exceeded"]}.',
        f'Incomplete checks: {json.dumps(incomplete)}.',
        f'Checks: {json.dumps(report.get("checks", []))}.',
        f'Measurement: {run_url or "local hook report; no uploaded measurement URL supplied"}.'])
    issue = api.create_issue(repo, f'Commit hook timing exceeded: source {sha[:12]}', body)
    result['issue'] = {'url': issue['html_url'], 'created': True}
    return result


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', default='tylergannon/gimbal')
    parser.add_argument('--run-id', type=int, action='append')
    parser.add_argument('--hook-report', help='Check a pre-commit.py --report JSON measurement')
    parser.add_argument('--scan', action='store_true', help='Check recent main preparations (including queued/failed)')
    parser.add_argument('--policy', default=str(Path(__file__).with_name('release-performance.json')))
    parser.add_argument('--now', help='UTC observation time; primarily for reproducible measurements')
    parser.add_argument('--report', help='Write the complete measurements as JSON')
    parser.add_argument('--measure', action='store_true', help='Measure without creating issues or applying adoption cutoff')
    args = parser.parse_args(argv)
    policy = json.loads(Path(args.policy).read_text())
    if not 0 < policy['goal_seconds'] <= policy['maximum_seconds']:
        raise ValueError('Release goal and maximum must be positive and ordered')
    now = args.now or dt.datetime.now(dt.timezone.utc).isoformat()
    timestamp(now)
    api = GitHub()
    base = f'repos/{args.repo}/actions'
    runs = api.collection(f'{base}/workflows/auto-release.yml/runs?per_page=100',
                          'workflow_runs', oldest=policy['enforce_after']) if args.scan else []
    if args.run_id:
        if not args.scan:
            runs = api.collection(f'{base}/workflows/auto-release.yml/runs?per_page=100',
                                  'workflow_runs', oldest=policy['enforce_after'])
        runs.extend(api.get(f'{base}/runs/{run_id}') for run_id in args.run_id)
    if not runs and not args.scan and not args.hook_report:
        parser.error('Provide --scan, --run-id, or --hook-report')
    oldest = min([run['created_at'] for run in runs], default=now)
    release_runs = api.collection(f'{base}/workflows/release.yml/runs?per_page=100',
                                 'workflow_runs', oldest=oldest) if runs else []
    selected_shas = {run['head_sha'] for run in runs if run['id'] in (args.run_id or [])}
    results = []
    seen = set()
    for run in sorted(runs, key=lambda value: value['created_at']):
        if run['head_branch'] != 'main' or run['event'] not in ('push', 'workflow_dispatch'):
            continue
        if args.run_id and not args.scan and run['head_sha'] not in selected_shas:
            continue
        if run['head_sha'] in seen:
            continue
        seen.add(run['head_sha'])
        if not args.measure and run['head_sha'] not in selected_shas and \
                timestamp(run['created_at']) < timestamp(policy['enforce_after']):
            continue
        preparations = [candidate for candidate in runs if candidate['head_sha'] == run['head_sha']
                        and candidate['head_branch'] == 'main'
                        and candidate['event'] in ('push', 'workflow_dispatch')]
        preparations = list({candidate['id']: candidate for candidate in preparations}.values())
        result = measure(api, args.repo, run, release_runs, policy, now, preparations)
        if not args.measure:
            result['issue'] = ensure_issue(api, args.repo, result)
        results.append(result)
    report = {'policy': policy, 'measurements': results}
    if args.hook_report:
        hook_report = json.loads(Path(args.hook_report).read_text())
        if args.measure:
            report['hook_measurement'] = hook_report
        else:
            report['hook_measurement'] = check_hook_report(api, args.repo, hook_report, policy)
    output = json.dumps(report, indent=2)
    if args.report:
        Path(args.report).write_text(output + '\n')
    print(output)
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (ValueError, KeyError, subprocess.CalledProcessError) as error:
        print(f'Release timing check failed: {error}', file=sys.stderr)
        sys.exit(1)
