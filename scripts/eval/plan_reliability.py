#!/usr/bin/env python3
"""Measure whether models keep their plan in sync with the work they do.

Runs short multi-step tasks through `offcode eval-case` (one fresh working
directory and HOME per case) and records, per case: whether plan_set was
called, how many steps the model marked done itself, how many "unfinished
plan" reminders the engine had to send, and whether the produced files pass
a mechanical check. A passing check with an unfinished plan, or a failing
check with a fully "done" plan, are both reliability defects worth reading.
"""
import argparse
import csv
import json
import os
from pathlib import Path
import subprocess
import urllib.request

TIERS = ['qwen3:1.7b', 'qwen3.5:4b', 'qwen3:8b', 'gemma4:12b']


def check_package(work):
    py = [p for p in work.rglob('*.py')]
    tests = [p for p in py if 'test' in p.name.lower()]
    code = [p for p in py if p not in tests and 'def ' in p.read_text(errors='replace')]
    readme = [p for p in work.rglob('*') if p.is_file() and p.name.lower().startswith('readme')]
    return bool(code and tests and readme)


def check_notes(work):
    files = [work/'notes'/f for f in ('todo.md', 'ideas.md', 'done.md')]
    return all(f.is_file() and f.read_text(errors='replace').lstrip().startswith('#') for f in files)


def check_config(work):
    try:
        cfg = json.loads((work/'config.json').read_text())
    except (OSError, ValueError):
        return False
    return isinstance(cfg, dict) and str(cfg.get('port')) == '9090' and cfg.get('debug') is True and 'host' in cfg


def check_users(work):
    try:
        rows = [r for r in csv.reader((work/'users.csv').read_text().splitlines()) if any(c.strip() for c in r)]
        summary = (work/'summary.txt').read_text()
    except OSError:
        return False
    data = [r for r in rows if any('@' in c for c in r)]  # skips a header row
    return len(data) == 5 and ('5' in summary or 'five' in summary.lower())


# Prompts are written like real short user messages, not requirement lists.
TASKS = [
    ('package', 'make me a tiny python package: a function that reverses the words in a sentence, a test file for it, and a short README', check_package),
    ('notes', 'set up a notes folder with todo.md, ideas.md and done.md, each with a heading', check_notes),
    ('config', 'create config.json with a port and a host, then change the port to 9090 and add debug: true', check_config),
    ('users', 'make users.csv with 5 fake users (name, email), then write summary.txt saying how many users there are', check_users),
]


def check_inventory(work):
    try:
        rows = list(csv.DictReader((work/'inventory.csv').read_text().splitlines()))
        totals = json.loads((work/'totals.json').read_text())
        return (rows == [{'item': 'pencil', 'quantity': '3', 'price': '2'},
                         {'item': 'notebook', 'quantity': '2', 'price': '5'},
                         {'item': 'eraser', 'quantity': '4', 'price': '1'}]
                and totals == {'pencil': 6, 'notebook': 10, 'eraser': 4, 'total': 20}
                and '20' in (work/'summary.md').read_text()
                and 'inventory.csv' in (work/'README.md').read_text())
    except (OSError, ValueError):
        return False


def check_settings(work):
    try:
        original = json.loads((work/'config.original.json').read_text())
        current = json.loads((work/'config.json').read_text())
        return (original == {'host': 'localhost', 'port': 8080, 'debug': False}
                and current == {'host': 'localhost', 'port': 9090, 'debug': True}
                and all(s in (work/'CHANGELOG.md').read_text() for s in ('8080', '9090'))
                and all(s in (work/'README.md').read_text() for s in ('localhost', '9090')))
    except (OSError, ValueError):
        return False


# Explicit planning requests separate plan maintenance from spontaneous adoption.
LONG_TASKS = [
    ('inventory', 'Use a plan with at least four steps and keep it updated as you work. '
     'Create inventory.csv with columns item,quantity,price and rows pencil,3,2; notebook,2,5; eraser,4,1. '
     'Read it back, then calculate each row value and the grand total into totals.json '
     '(keys pencil, notebook, eraser, total; numeric values). Write summary.md explaining the total, '
     'and README.md describing inventory.csv. Read all outputs to verify them before finishing. '
     'Use file tools; shell execution is unavailable.', check_inventory),
    ('settings', 'Use a plan with at least four steps and update it after completing each step. '
     'Write config.json with host localhost, port 8080 and debug false. Read it and save an identical '
     'backup as config.original.json. Edit config.json to port 9090 and debug true, keeping host. '
     'Write CHANGELOG.md documenting old and new settings, then README.md describing current host and port. '
     'Read back both JSON files and docs to verify them before finishing. Use file tools; shell execution '
     'is unavailable.', check_settings),
]


def tool_calls(blocks, name):
    return [b for b in blocks if b.get('kind') == 'tool_call' and b.get('tool_name') == name]


def summarize(result):
    state = result.get('state') or {}
    blocks = state.get('blocks') or []
    plan = state.get('plan') or []
    statuses = [p.get('status') for p in plan]
    systems = [b.get('text', '') for b in blocks if b.get('kind') == 'system']
    first_nudge = next((i for i, b in enumerate(blocks)
                        if b.get('kind') == 'system' and 'unfinished steps' in b.get('text', '')), len(blocks))
    return {
        'plan_set_calls': len(tool_calls(blocks, 'plan_set')),
        'plan_update_calls': len(tool_calls(blocks, 'plan_update')),
        'plan_updates_before_reminder': len(tool_calls(blocks[:first_nudge], 'plan_update')),
        'plan_updates_after_reminder': len(tool_calls(blocks[first_nudge:], 'plan_update')),
        'plan_steps': len(plan),
        'plan_done': sum(s in ('done', 'skipped') for s in statuses),
        'plan_complete': bool(plan) and all(s in ('done', 'skipped') for s in statuses),
        'nudges': sum('unfinished steps' in t for t in systems),
        'stopped_unfinished': any('stopped without a final answer' in t for t in systems),
        'tool_errors': sum(1 for b in blocks if b.get('kind') == 'tool_result' and b.get('is_error')),
    }


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--binary', default='bin/offcode')
    p.add_argument('--backend', choices=['ollama', 'lmstudio'], default='ollama')
    p.add_argument('--base-url')
    p.add_argument('--models', nargs='+', default=TIERS)
    p.add_argument('--repeats', type=int, default=2)
    p.add_argument('--output', required=True, help='new report directory')
    p.add_argument('--timeout', type=int, default=600)
    p.add_argument('--suite', choices=['short', 'long', 'all'], default='short')
    a = p.parse_args()
    root = Path(a.output).resolve()
    root.mkdir(parents=True, exist_ok=False)
    url = (a.base_url or ('http://127.0.0.1:11434/v1' if a.backend == 'ollama' else 'http://127.0.0.1:1234/v1')).rstrip('/')
    endpoint = url.removesuffix('/v1')+'/api/tags' if a.backend == 'ollama' else url+'/models'
    report = {'backend': a.backend, 'base_url': url, 'models': a.models, 'repeats': a.repeats, 'suite': a.suite, 'cases': []}
    try:
        with urllib.request.urlopen(endpoint, timeout=5) as response:
            json.load(response)
    except Exception as exc:
        report.update(status='blocked', error=str(exc))
        (root/'report.json').write_text(json.dumps(report, indent=2)+'\n', encoding='utf-8')
        print(f'Blocked: {exc}')
        return 1
    binary = str(Path(a.binary).resolve())
    tasks = TASKS if a.suite == 'short' else LONG_TASKS if a.suite == 'long' else TASKS + LONG_TASKS
    for mi, model in enumerate(a.models):
        for name, prompt, check in tasks:
            for rep in range(1, a.repeats+1):
                case = root/f'model-{mi+1}'/f'{name}-{rep}'
                work, home = case/'work', case/'home'
                work.mkdir(parents=True)
                home.mkdir()
                prompt_path = case/'prompt.txt'
                prompt_path.write_text(prompt, encoding='utf-8')
                env = dict(os.environ, HOME=str(home), USERPROFILE=str(home))
                command = [binary, 'eval-case', '-backend', a.backend, '-base-url', url, '-model', model,
                           '-prompt-file', str(prompt_path), '-timeout', f'{a.timeout}s']
                try:
                    process = subprocess.run(command, cwd=work, env=env, text=True, capture_output=True, timeout=a.timeout+45)
                    (case/'stderr.txt').write_text(process.stderr, encoding='utf-8')
                    result = json.loads(process.stdout)
                except subprocess.TimeoutExpired:
                    result = {'status': 'timeout'}
                except (ValueError, OSError) as exc:
                    result = {'status': 'runner_error', 'error': str(exc)}
                (case/'result.json').write_text(json.dumps(result, indent=2)+'\n', encoding='utf-8')
                row = {'model': model, 'task': name, 'repeat': rep, 'status': result.get('status'),
                       'check_passed': check(work), **summarize(result)}
                report['cases'].append(row)
                (root/'report.json').write_text(json.dumps(report, indent=2)+'\n', encoding='utf-8')
                print(json.dumps(row), flush=True)
    report['status'] = 'finished'
    (root/'report.json').write_text(json.dumps(report, indent=2)+'\n', encoding='utf-8')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
