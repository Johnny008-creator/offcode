#!/usr/bin/env python3
"""Repeat the ten-task campaign with one process/directory per case.

Reports observations, not subjective quality scores. Review generated UIs and
calculator behavior manually. Run untrusted models in an OS/container sandbox;
a fresh working directory and restricted approvals are not an OS sandbox.
"""
import argparse
import json
import os
from pathlib import Path
import subprocess
import urllib.request

TIERS = ['qwen3:1.7b', 'qwen3.5:4b', 'qwen3:8b', 'gemma4:12b', 'gpt-oss:20b']
TASKS = [
 ('Say hi in one sentence and tell me what offcode tools you have available.', []),
 ('Create a file called hello.txt containing "hello world", then read it back to confirm the content is correct.', ['hello.txt']),
 ('Create three files a.txt, b.txt, c.txt, each containing just their own letter. Plan it out first.', ['a.txt','b.txt','c.txt']),
 ("I want to add a new feature to a small project but haven't decided what. Offer me a few reasonable options with one recommended, then wait for my choice.", []),
 ('Search for what SearXNG is and give me a short summary. If the search results do not fully answer it, look at the actual page.', []),
 ('Build a single-file HTML page for a simple note-taking app UI (no backend needed), with good modern CSS styling. Save it as notes.html.', ['notes.html']),
 ('Create a file called config.json with a few example settings, then edit it to add one more setting without rewriting the whole file.', ['config.json']),
 ('Run a command that lists all files in the current directory sorted by size, and tell me the largest one.', []),
 ('Build a tiny static website: an index.html, an about.html, and a shared style.css linked from both. Make it look professional.', ['index.html','about.html','style.css']),
 ('Build a simple single-page calculator app — HTML, CSS, and working JavaScript so the buttons actually compute results, all in one file called calculator.html.', ['calculator.html']),
]


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--binary', default='bin/offcode')
    p.add_argument('--backend', choices=['ollama','lmstudio'], default='ollama')
    p.add_argument('--base-url')
    p.add_argument('--models', nargs='+', help='backend model tags; defaults to all five Ollama tiers')
    p.add_argument('--output', required=True, help='new report directory')
    p.add_argument('--timeout', type=int, default=600)
    a = p.parse_args()
    if a.backend == 'lmstudio' and not a.models:
        p.error('--models is required for LM Studio')
    models = a.models or TIERS
    root = Path(a.output).resolve()
    root.mkdir(parents=True, exist_ok=False)
    url = (a.base_url or ('http://127.0.0.1:11434/v1' if a.backend == 'ollama' else 'http://127.0.0.1:1234/v1')).rstrip('/')
    report = {'backend':a.backend, 'base_url':url, 'models':models, 'cases':[], 'manual_review_required':True}
    endpoint = url.removesuffix('/v1')+'/api/tags' if a.backend == 'ollama' else url+'/models'
    try:
        with urllib.request.urlopen(endpoint, timeout=5) as response:
            json.load(response)
    except Exception as exc:
        report.update(status='blocked', error=str(exc))
        (root/'report.json').write_text(json.dumps(report, indent=2)+'\n', encoding='utf-8')
        print(f'Blocked: {exc}; report: {root / "report.json"}')
        return 1
    binary = str(Path(a.binary).resolve())
    for model_index, model in enumerate(models):
        for number, (prompt, expected) in enumerate(TASKS, 1):
            case = root/f'model-{model_index+1}'/f'task-{number:02}'
            work, home = case/'work', case/'home'
            work.mkdir(parents=True); home.mkdir()
            prompt_path = case/'prompt.txt'; prompt_path.write_text(prompt, encoding='utf-8')
            env = dict(os.environ, HOME=str(home), USERPROFILE=str(home))
            command = [binary,'eval-case','-backend',a.backend,'-base-url',url,'-model',model,
                       '-prompt-file',str(prompt_path),'-timeout',f'{a.timeout}s']
            try:
                process = subprocess.run(command, cwd=work, env=env, text=True, capture_output=True, timeout=a.timeout+45)
                (case/'stderr.txt').write_text(process.stderr, encoding='utf-8')
                result = json.loads(process.stdout)
            except subprocess.TimeoutExpired:
                result = {'status':'timeout'}
            except (ValueError, OSError) as exc:
                result = {'status':'runner_error','error':str(exc)}
            result.update(model=model, task=number, files={f:(work/f).is_file() and (work/f).stat().st_size>0 for f in expected})
            if number == 2 and (work/'hello.txt').is_file():
                result['content_correct'] = (work/'hello.txt').read_text(encoding='utf-8').strip()=='hello world'
            if number == 3:
                result['content_correct'] = all((work/f'{c}.txt').is_file() and (work/f'{c}.txt').read_text(encoding='utf-8').strip()==c for c in 'abc')
            result['manual_review_required'] = number in [1,4,5,6,7,8,9,10]
            (case/'result.json').write_text(json.dumps(result, indent=2)+'\n', encoding='utf-8')
            report['cases'].append(result)
            (root/'report.json').write_text(json.dumps(report, indent=2)+'\n', encoding='utf-8')
            print(model, number, result['status'], flush=True)
    report['status'] = 'campaign_finished_review_required'
    (root/'report.json').write_text(json.dumps(report, indent=2)+'\n', encoding='utf-8')
    return 0

if __name__ == '__main__':
    raise SystemExit(main())
